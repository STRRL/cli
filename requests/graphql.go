package requests

import (
	"context"
	"github.com/let-sh/cli/utils"
	"github.com/machinebox/graphql"
)

// create a client (safe to share across requests)
var Graphql = graphql.NewClient("https://api.let.sh.cn/query")

func CheckDeployCapability(projectName string) (hashID string, exists bool, err error) {
	req := graphql.NewRequest(`
    query($name: String!) {
        checkDeployCapability (projectName:$name) {
			hashID
			exists
        }
    }
`)

	req.Var("name", projectName)
	req.Header.Set("Authorization", "Bearer "+utils.Credentials.Token)

	// run it and capture the response
	var respData struct {
		CheckDeployCapability struct {
			HashID string `json:"hashID"`
			Exists bool   `json:"exists"`
		} `json:"checkDeployCapability,omitempty"`
	}
	if err := Graphql.Run(context.Background(), req, &respData); err != nil {
		//if len(respData.Error.Errors) > 0 {
		//	return "", false, errors.New(respData.Error.Errors[0].Message)
		//}

		return "", false, err
	}

	return respData.CheckDeployCapability.HashID, respData.CheckDeployCapability.Exists, nil
}

func GetStsToken(uploadType, projectName string) (data struct {
	Host            string `json:"host"`
	AccessKeyID     string `json:"accessKeyID"`
	AccessKeySecret string `json:"accessKeySecret"`
	SecurityToken   string `json:"securityToken"`
}, err error) {
	req := graphql.NewRequest(`
query($type: String!, $name: String!) {
	stsToken(type:$type,projectName:$name) {
		host
		accessKeyID
		accessKeySecret
		securityToken
	}
}
`)

	req.Var("type", uploadType)
	req.Var("name", projectName)
	req.Header.Set("Authorization", "Bearer "+utils.Credentials.Token)

	// run it and capture the response
	var respData struct {
		StsToken struct {
			Host            string `json:"host"`
			AccessKeyID     string `json:"accessKeyID"`
			AccessKeySecret string `json:"accessKeySecret"`
			SecurityToken   string `json:"securityToken"`
		} `json:"stsToken,omitempty"`
	}
	if err := Graphql.Run(context.Background(), req, &respData); err != nil {
		//if len(respData.Error.Errors) > 0 {
		//	return "", false, errors.New(respData.Error.Errors[0].Message)
		//}

		return data, err
	}

	return respData.StsToken, nil
}
