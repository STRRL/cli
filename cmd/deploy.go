/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
	"github.com/let-sh/cli/log"
	"github.com/let-sh/cli/requests"
	"github.com/let-sh/cli/types"
	"github.com/let-sh/cli/utils"
	"github.com/let-sh/cli/utils/oss"
	"github.com/mholt/archiver/v3"
	c "github.com/otiai10/copy"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"time"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your current project to let.sh",
	Long:  `Deploy your current project to let.sh with a single command line`,
	Run: func(cmd *cobra.Command, args []string) {
		log.BStart("deploying")

		// check whether user is logged in
		if utils.Credentials.Token == "" {
			log.Warning("please login via `lets login` first")
			return
		}

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
				// if file exists
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
			log.BStop()
			log.Error(err)
			return
		}

		// get project type config from api

		{
			// TODO: check current dir, if too many files, alert user
			// TODO: respect .gitignore

		}
		// if contains static, upload static files to oss

		// if contains dynamic, upload dynamic files to oss
		// then trigger deployment
		{

			// create temp dir
			dir := os.TempDir()

			defer os.RemoveAll(dir)
			//fmt.Println(dir)
			//os.MkdirAll(dir+"/source", os.ModePerm)

			// copy current dir to temp dir
			c.Copy("./", dir+"/"+deploymentConfig.Name+"-"+hashID)

			// remove if not clean
			os.Remove(dir + "/" + deploymentConfig.Name + "-" + hashID + ".tar.gz")
			err = archiver.Archive([]string{dir + "/" + deploymentConfig.Name + "-" + hashID}, dir+"/"+deploymentConfig.Name+"-"+hashID+".tar.gz")
			if err != nil {
				log.Error(err)
				return
			}

			oss.UploadFileToCodeSource(dir+"/"+deploymentConfig.Name+"-"+hashID+".tar.gz", deploymentConfig.Name+"-"+hashID+".tar.gz", deploymentConfig.Name)
		}

		// awaiting deployment result

		time.Sleep(time.Second * 4)
		log.BStop()
		return
	},
}

var deploymentConfig types.LetConfig
var inputProjectName string
var inputProjectType string

func init() {
	rootCmd.AddCommand(deployCmd)

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	deployCmd.Flags().StringVarP(&inputProjectName, "project", "p", "", "current project name")
	deployCmd.Flags().StringVarP(&inputProjectType, "type", "t", "", "current project type, e.g. react")
}
