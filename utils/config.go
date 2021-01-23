package utils

import (
	"encoding/json"
	"github.com/let-sh/cli/log"
	"github.com/let-sh/cli/types"
	"github.com/mitchellh/go-homedir"
	"io/ioutil"
	"os"
)

var Credentials types.Credentials

func init() {
	// check config dir exists
	home, _ := homedir.Dir()
	_, err := os.Stat(home + "/.let")
	if os.IsNotExist(err) {
		os.MkdirAll(home+"/.let", os.ModePerm)
	}

	_, err = os.Stat(home + "/.let/credentials.json")
	if os.IsNotExist(err) {
		f, _ := os.Create(home + "/.let/credentials.json")
		f.WriteString("{}")
	}

	_, err = os.Stat(home + "/.let/projects.json")
	if os.IsNotExist(err) {
		f, _ := os.Create(home + "/.let/projects.json")
		f.WriteString("{}")
	}

	// bootstrap configs
	credentialsFile, _ := ioutil.ReadFile(home + "/.let/credentials.json")
	err = json.Unmarshal(credentialsFile, &Credentials)
	if err != nil {
		log.Error(err)
	}
}

func Load() {}

func SetToken(token string) {
	Credentials.Token = token
	home, _ := homedir.Dir()

	file, _ := json.MarshalIndent(Credentials, "", "  ")
	_ = ioutil.WriteFile(home+"/.let/credentials.json", file, 0644)
}
