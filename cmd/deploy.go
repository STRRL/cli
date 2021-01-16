/*
Copyright © 2021 Fred Liang <fred@oasis.ac>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/let-sh/cli/log"
	"github.com/let-sh/cli/requests"
	"github.com/let-sh/cli/types"
	"github.com/let-sh/cli/utils"
	"github.com/let-sh/cli/utils/oss"
	"github.com/mholt/archiver/v3"
	c "github.com/otiai10/copy"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"
	"time"
)

var DeploymentID string

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your current project to let.sh",
	Long:  `Deploy your current project to let.sh with a single command line`,
	Run: func(cmd *cobra.Command, args []string) {
		// Setup our Ctrl+C handler
		SetupCloseHandler()

		// check whether user is logged in
		if utils.Credentials.Token == "" {
			log.Warning("please login via `lets login` first")
			return
		}

		log.BStart("deploying")
		// merge config
		// cli flag > config file > auto saved config > detected config & types
		{
			// detect current project config first
			// init current project name
			dir, _ := os.Getwd()
			deploymentConfig.Name = filepath.Base(dir)

			// check if static by index.html
			_, err := os.Stat("index.html")
			if !os.IsNotExist(err) {
				deploymentConfig.Type = "static"
				deploymentConfig.Static = "./"
			}

			if len(inputStaticDir) > 0 {
				deploymentConfig.Type = "static"
				deploymentConfig.Static = inputStaticDir
			}

			// check if js by package.json
			//_, err := os.Stat("package.json")
			//if !os.IsNotExist(err) {
			//	deploymentConfig.Type = "static"
			//}

			// check if golang by go.mod
			_, err = os.Stat("go.mod")
			if !os.IsNotExist(err) {
				deploymentConfig.Type = "gin"
			}

			// if not match anything
			if deploymentConfig.Type == "" {
				deploymentConfig.Type = "static"
			}

			// Step2: get cache config

			// Step3: load user config
			_, err = os.Stat("let.json")
			if !os.IsNotExist(err) {
				// TODO: if file exists
				jsonFile, err := os.Open("let.json")
				// if we os.Open returns an error then handle it
				if err != nil {
					log.Error(err)
					return
				}
				// defer the closing of our jsonFile so that we can parse it later on
				defer jsonFile.Close()
				byteValue, _ := ioutil.ReadAll(jsonFile)
				json.Unmarshal(byteValue, &deploymentConfig)
			}

			// Step4: merge cli flag config
			if inputProjectName != "" {
				deploymentConfig.Name = inputProjectName
			}

			if inputProjectType != "" {
				deploymentConfig.Type = inputProjectType
			}
		}

		// check Check Deploy Capability
		hashID, _, err := requests.CheckDeployCapability(deploymentConfig.Name)
		if err != nil {
			log.Error(err)
			return
		}

		// get project type config from api

		{
			// check not home dir
			dir, _ := os.Getwd()
			usr, _ := user.Current()
			if dir == usr.HomeDir {
				log.Error(errors.New("currently under home dir, please switch to your project dir"))
				return
			}

			// limit files to 10000
			files, _ := ioutil.ReadDir("./")
			if len(files) > 10000 {
				log.Error(errors.New("too many files in current dir, please check whether in the correct directory"))
				return
			}

		}

		log.S.StopFail()

		fmt.Println("")
		fmt.Println(log.CyanBold("Detected Project Info"))
		fmt.Printf("name: %s\n", deploymentConfig.Name)
		fmt.Printf("type: %s\n", deploymentConfig.Type)
		fmt.Println("")

		// if contains static, upload static files to oss
		if utils.ItemExists([]string{"static"}, deploymentConfig.Type) {
			if err := oss.UploadDirToStaticSource(deploymentConfig.Static, deploymentConfig.Name, deploymentConfig.Name+"-"+hashID); err != nil {
				log.Error(err)
				return
			}
		}

		// if contains dynamic, upload dynamic files to oss
		// then trigger deployment
		if utils.ItemExists([]string{"gin", "express"}, deploymentConfig.Type) {

			// create temp dir
			dir := os.TempDir()

			defer os.RemoveAll(dir)
			//fmt.Println(dir)
			//os.MkdirAll(dir+"/source", os.ModePerm)

			// copy current dir to temp dir
			c.Copy("./", dir+"/"+deploymentConfig.Name+"-"+hashID)

			// remove if not clean
			os.Remove(dir + "/" + deploymentConfig.Name + "-" + hashID + ".tar.gz")
			err = archiver.Archive([]string{"."}, dir+"/"+deploymentConfig.Name+"-"+hashID+".tar.gz")
			if err != nil {
				log.Error(err)
				return
			}

			oss.UploadFileToCodeSource(dir+"/"+deploymentConfig.Name+"-"+hashID+".tar.gz", deploymentConfig.Name+"-"+hashID+".tar.gz", deploymentConfig.Name)
		}

		configBytes, _ := json.Marshal(deploymentConfig)
		deployment, err := requests.Deploy(deploymentConfig.Type, deploymentConfig.Name, string(configBytes), inputCN)

		if err != nil {
			log.Error(err)
			return
		}

		DeploymentID = deployment.ID

		log.BStart("deploying")

		// awaiting deployment result
		for {
			currentStatus, err := requests.GetDeploymentStatus(deployment.ID)
			if err != nil {
				log.Error(err)
			}

			log.BUpdate(" NetworkStage: " + currentStatus.NetworkStage + ", PackerStage: " + currentStatus.PackerStage + ", Status: " + currentStatus.Status)

			if currentStatus.Done {
				if currentStatus.Status == "Failed" {
					log.Error(errors.New(currentStatus.ErrorMessage))
					break
				}

				log.S.StopFail()
				fmt.Println(
					color.New(color.Bold).Sprint("Preview: ")+color.New().Sprint("https://"+currentStatus.TargetFQDN), "\n"+
						color.New(color.Bold).Sprint("Details: ")+color.New().Sprint("https://alpha.let.sh.cn/console/project/"+deploymentConfig.Name+"/details"),
				)
				break

				// interval
				time.Sleep(1)
			}

		}

		return
	},
}

var deploymentConfig types.LetConfig
var inputProjectName string
var inputProjectType string
var inputCN bool
var inputStaticDir string

func init() {
	rootCmd.AddCommand(deployCmd)

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	deployCmd.Flags().StringVarP(&inputProjectName, "project", "p", "", "current project name")
	deployCmd.Flags().StringVarP(&inputProjectType, "type", "t", "", "current project type, e.g. react")
	deployCmd.Flags().StringVarP(&inputStaticDir, "static", "", "", "static dir name (if deploy type is static)")
	deployCmd.Flags().BoolVarP(&inputCN, "cn", "", true, "deploy in mainland of china")
	deployCmd.Flags().MarkHidden("cn")
}

func SetupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if len(DeploymentID) > 0 {
			succeed, err := requests.CancelDeployment(DeploymentID)
			if err != nil {
				log.S.StopFail()
				log.Error(err)
				os.Exit(0)
			}
			if succeed {
				log.S.StopFail()
				log.Warning("Deployment canceled")
				os.Exit(0)
			} else {
				log.S.StopFail()
				log.Warning("Deployment cancellation failed")
				os.Exit(0)
			}
		} else {
			log.S.StopFail()
			log.Warning("Deployment canceled")
			os.Exit(0)
		}

	}()
}
