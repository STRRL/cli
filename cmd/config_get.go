package cmd

import (
	"fmt"
	"github.com/let-sh/cli/log"
	"github.com/let-sh/cli/requests"
	"github.com/spf13/cobra"
	"strings"
)

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

// configGetCmd represents the config command
var configGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get you personal configuration",
	Long: `Get your personal configurations

e.g. lets config get default_channel
`,
	Run: func(cmd *cobra.Command, args []string) {
		value, err := requests.GetPreference(strings.TrimSpace(args[0]))
		if err != nil {
			log.Errorf("cannot get configuration: %s", value)
			return
		}

		fmt.Println(value)
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// configCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// configCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}