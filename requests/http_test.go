package requests

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestGetLatestVersion(t *testing.T) {
	fmt.Println(GetLatestVersion("rc"))
}

func TestHttpClient(t *testing.T) {
	resp, err := httpClient.Get("https://graphql.let.sh")
	if err != nil {
		panic(err)

	}
	defer resp.Body.Close()
	s, err := ioutil.ReadAll(resp.Body)
	fmt.Printf(string(s))
}
