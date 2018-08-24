package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
)

type RSAValue struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

type Secret struct {
	Id       string
	Name     string
	Type     string
	Value    string
	RSAValue RSAValue `json:"rsaValue,omitempty"`
}

type Secrets struct {
	Secrets []Secret `json:"secrets"`
}
type SecretOutput struct {
	Entry *secretsmanager.SecretListEntry
	Value *secretsmanager.GetSecretValueOutput
}

var secretsmanagerCommand = func() cli.Command {
	return cli.Command{
		Name: "secretsmanager",
		Subcommands: []cli.Command{
			{
				Name:  "export",
				Usage: "exports ssm parameters and their values to json",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "output-file-name",
						Value: "secretsmanager.json",
					},
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					outFileName := c.String("output-file-name")
					sess := NewSessionWithSharedProfile(profile)

					ssmc := secretsmanager.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

					listTopicsParams := secretsmanager.ListSecretsInput{}
					var entries []*SecretOutput
					ssmc.ListSecretsPages(&listTopicsParams, func(output *secretsmanager.ListSecretsOutput, lastPage bool) bool {
						for _, elem := range output.SecretList {

							getSecretValueInput := secretsmanager.GetSecretValueInput{
								SecretId: elem.ARN,
							}
							getSecretValueOutput, err := ssmc.GetSecretValue(&getSecretValueInput)
							if err != nil {
								panic(err)
							}

							entries = append(entries, &SecretOutput{
								Entry: elem,
								Value: getSecretValueOutput,
							})
						}
						return lastPage == false
					})

					b, _ := json.Marshal(entries)
					_ = ioutil.WriteFile(outFileName, b, 0644)
					return nil
				},
			},
			{
				Name:  "create-secrets",
				Usage: "createsSecrets from file",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name: "input-file-name",
					},
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					inFileName := c.String("input-file-name")
					if inFileName == "" {
						return fmt.Errorf("input-file-name not found")
					}

					sess := NewSessionWithSharedProfile(profile)

					jsonFile, err := os.Open(inFileName)
					if err != nil {
						return fmt.Errorf("error when opening %s", inFileName)
					}
					defer jsonFile.Close()

					secrets := Secrets{}
					byteValue, _ := ioutil.ReadAll(jsonFile)
					if err := json.Unmarshal(byteValue, &secrets); err != nil {
						return err
					}

					var inputs []*secretsmanager.CreateSecretInput
					for _, secret := range secrets.Secrets {
						if secret.Type == "value" || secret.Type == "password" {
							input := secretsmanager.CreateSecretInput{
								ClientRequestToken: aws.String(secret.Id),
								Name:               aws.String(secret.Name),
								SecretString:       aws.String(secret.Value),
							}
							inputs = append(inputs, &input)
						}

						if secret.Type == "rsa" {
							var rsa []byte
							if rsa, err = json.Marshal(secret.RSAValue); err != nil {
								return err
							}

							input := secretsmanager.CreateSecretInput{
								ClientRequestToken: aws.String(secret.Id),
								Name:               aws.String(secret.Name),
								SecretString:       aws.String(string(rsa)),
							}
							inputs = append(inputs, &input)
						}
					}

					sm := secretsmanager.New(sess)

					for _, input := range inputs {
						if _, err := sm.CreateSecret(input); err != nil {
							return err
						}
					}

					return nil
				},
			},
		},
	}
}
