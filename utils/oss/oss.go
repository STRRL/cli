package oss

import (
	"bufio"
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/denormal/go-gitignore"
	"github.com/let-sh/cli/log"
	"github.com/let-sh/cli/requests"
	"github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var bar *mpb.Bar
var uploadStatus = make(map[string]fileUplaodStatus)
var mutex = &sync.Mutex{}

type fileUplaodStatus struct {
	FilePath     string
	ConsumedSize int64
	TotalSize    int64
}

func UploadFileToCodeSource(filedir, filename, projectName string) {
	// create and start new bar
	fi, _ := os.Stat(filedir)

	file, _ := os.Open(filedir)
	r := bufio.NewReader(file)

	p := mpb.New(
		mpb.WithWidth(64),
		mpb.WithRefreshRate(200*time.Millisecond),
	)

	bar = p.AddBar(fi.Size(), mpb.BarStyle("[=>-|"),
		mpb.PrependDecorators(
			decor.CountersKiloByte("% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 90),
			decor.Name(" ] "),
			decor.EwmaSpeed(decor.UnitKB, "% .2f", 1024),
		),
		mpb.BarRemoveOnComplete(),
	)

	stsToken, err := requests.GetStsToken("buildBundle", projectName)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}

	// 创建OSSClient实例
	endpoint := strings.Join(strings.Split(stsToken.Host, ".")[1:], ".")
	client, err := oss.New(endpoint, stsToken.AccessKeyID, stsToken.AccessKeySecret, oss.SecurityToken(stsToken.SecurityToken))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}

	bucketName := strings.Replace(strings.Split(stsToken.Host, ".")[0], "https://", "", 1)

	// 获取存储空间。
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}

	// create proxy reader
	proxyReader := bar.ProxyReader(r)
	defer proxyReader.Close()

	logrus.WithFields(logrus.Fields{
		"objKey": filename,
	}).Debug("put object from file")

	err = bucket.PutObject(filename, proxyReader, oss.Progress(&OssProgressListener{}))
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}
	bar.Completed()
}

func UploadDirToStaticSource(dirPath, projectName, bundleID string) error {
	log.BPause()
	uploadStatus = make(map[string]fileUplaodStatus)
	stsToken, err := requests.GetStsToken("static", projectName)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}
	// 创建OSSClient实例
	endpoint := strings.Join(strings.Split(stsToken.Host, ".")[1:], ".")
	client, err := oss.New(endpoint, stsToken.AccessKeyID, stsToken.AccessKeySecret, oss.SecurityToken(stsToken.SecurityToken))

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}
	bucketName := strings.Replace(strings.Split(stsToken.Host, ".")[0], "https://", "", 1)

	// 获取存储空间。
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}

	// Read directory files
	var names []string
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// respect .gitignore and .letignore
	if _, err := os.Stat(filepath.Join(dirPath, ".gitignore")); err == nil {
		// match a file against a particular .gitignore
		ignore, _ := gitignore.NewFromFile(filepath.Join(dirPath, ".gitignore"))

		tmp := []string{}
		for _, v := range names {
			match := ignore.Match(v)
			if match != nil {
				if !match.Ignore() {
					tmp = append(tmp, v)
				}
			}
		}

		names = tmp
	}

	// .letignore
	if _, err := os.Stat(filepath.Join(dirPath + ".letignore")); err == nil {
		// match a file against a particular .gitignore
		ignore, _ := gitignore.NewFromFile(filepath.Join(dirPath + ".letignore"))

		tmp := []string{}
		for _, v := range names {
			match := ignore.Match(v)
			if match != nil {
				if !match.Ignore() {

					fi, _ := os.Stat(v)
					mutex.Lock()
					// register upload status
					uploadStatus[v] = struct {
						FilePath     string
						ConsumedSize int64
						TotalSize    int64
					}{FilePath: fi.Name(), ConsumedSize: 0, TotalSize: fi.Size()}
					mutex.Unlock()

					tmp = append(tmp, v)
				}
			}
		}

		names = tmp
	}

	// Copy names to a channel for workers to consume. Close the
	// channel so that workers stop when all work is complete.
	namesChan := make(chan string, len(names))
	for _, name := range names {
		namesChan <- name
	}
	close(namesChan)

	// Create a maximum of 8 workers

	workers := 8
	if len(names) < workers {
		workers = len(names)
	}

	errChan := make(chan error, 1)
	resChan := make(chan *error, len(names))

	// Run workers

	for i := 0; i < workers; i++ {
		go func() {
			// Consume work from namesChan. Loop will end when no more work.
			for name := range namesChan {
				if err != nil {
					select {
					case errChan <- err:
						// will break parent goroutine out of loop
					default:
						// don't care, first error wins
					}
					return
				}

				objKey := filepath.Join(bundleID, strings.Replace(name, dirPath, "", 1))
				filePath := name

				// skip dir
				fi, err := os.Stat(filePath)

				if err != nil {
					fmt.Println(err)
					resChan <- &err
					return
				}
				if fi.IsDir() {
					resChan <- &err
					return
				}

				logrus.WithFields(logrus.Fields{
					"objKey":   objKey,
					"filePath": filePath,
				}).Debug("put object from file")
				err = bucket.PutObjectFromFile(func() string {
					if runtime.GOOS == "windows" {
						return filepath.ToSlash(objKey)
					}
					return objKey
				}(), filePath, oss.Progress(&OssProgressListener{filepath: filePath}))
				if err != nil {
					select {
					case errChan <- err:
						// will break parent goroutine out of loop
					default:
						// don't care, first error wins
					}
					return
				}
				resChan <- &err
			}
		}()
	}

	// Collect results from workers
	for i := 0; i < len(names); i++ {
		select {
		case res := <-resChan:
			// collect result
			_ = res
		case err := <-errChan:
			return err
		}
	}
	log.S.Suffix(" deploying ")
	log.BUnpause()
	return nil
}

type OssProgressListener struct {
	filepath string
}

func (listener *OssProgressListener) ProgressChanged(event *oss.ProgressEvent) {

	switch event.EventType {

	case oss.TransferStartedEvent:
		//fmt.Printf("Transfer Started, ConsumedBytes: %d, TotalBytes %d.\n",
		//	event.ConsumedBytes, event.TotalBytes)

	case oss.TransferDataEvent:
		UpdateUploadBar()

		mutex.Lock()
		//todo: add uploading bar
		//bar.IncrBy(int(event.ConsumedBytes - uploadStatus[listener.filepath].ConsumedSize))
		uploadStatus[listener.filepath] = struct {
			FilePath     string
			ConsumedSize int64
			TotalSize    int64
		}{FilePath: listener.filepath, ConsumedSize: event.ConsumedBytes, TotalSize: event.TotalBytes}
		mutex.Unlock()

		//fmt.Printf("\rTransfer Data, ConsumedBytes: %d, TotalBytes %d, %d%%.",
		//	event.ConsumedBytes, event.TotalBytes, event.ConsumedBytes*100/event.TotalBytes)

	case oss.TransferCompletedEvent:
		//fmt.Printf("\nTransfer Completed, ConsumedBytes: %d, TotalBytes %d.\n",
		//	event.ConsumedBytes, event.TotalBytes)
	case oss.TransferFailedEvent:
		//fmt.Printf("\nTransfer Failed, ConsumedBytes: %d, TotalBytes %d.\n",
		//	event.ConsumedBytes, event.TotalBytes)
	default:
	}
}

func UpdateUploadBar() (totalConsumedSize, totalSize int64) {

	mutex.Lock()
	for _, v := range uploadStatus {
		totalConsumedSize += v.ConsumedSize
		totalSize += v.TotalSize
	}
	mutex.Unlock()
	//todo: add uploading bar
	//if bar == nil {
	//	p := mpb.New(
	//		mpb.WithWidth(64),
	//		mpb.WithRefreshRate(200*time.Millisecond),
	//	)
	//
	//	bar = p.AddBar(totalSize, mpb.BarStyle("[=>-|"),
	//		mpb.PrependDecorators(
	//			decor.CountersKiloByte("% .2f / % .2f"),
	//		),
	//		mpb.AppendDecorators(
	//			decor.EwmaETA(decor.ET_STYLE_GO, 90),
	//			decor.Name(" ] "),
	//			decor.EwmaSpeed(decor.UnitKB, "% .2f", 1024),
	//		),
	//		mpb.BarRemoveOnComplete(),
	//	)
	//}

	//if totalConsumedSize == totalSize {
	//	bar.SetTotal(totalSize, true)
	//} else {
	//	bar.SetTotal(totalSize, false)
	//}
	return totalConsumedSize, totalSize
}
