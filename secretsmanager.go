package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
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
				Usage: "exports secrets and their values to json",
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

					ssmc := secretsmanager.New(sess)

					listSecretsInput := secretsmanager.ListSecretsInput{}
					var entries []*SecretOutput
					ssmc.ListSecretsPages(&listSecretsInput, func(output *secretsmanager.ListSecretsOutput, lastPage bool) bool {
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

					if entries == nil {
						fmt.Println("no secrets found")
						return nil
					}

					b, _ := json.Marshal(entries)
					_ = ioutil.WriteFile(outFileName, b, 0644)

					fmt.Printf("exported to %s", outFileName)
					return nil
				},
			},
			{
				Name:  "delete-all",
				Usage: "deletes all values from secretsmanager",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					sess := NewSessionWithSharedProfile(profile)

					ssmc := secretsmanager.New(sess)

					listSecretsInput := secretsmanager.ListSecretsInput{}
					ssmc.ListSecretsPages(&listSecretsInput, func(output *secretsmanager.ListSecretsOutput, lastPage bool) bool {
						for _, elem := range output.SecretList {

							deleteSecretInput := secretsmanager.DeleteSecretInput{
								SecretId: elem.ARN,
							}
							_, err := ssmc.DeleteSecret(&deleteSecretInput)
							if err != nil {
								panic(err)
							}
							fmt.Printf("deleted: %v\n", &elem.Name)
						}
						return lastPage == false
					})
					return nil
				},
			},
			{
				Name:  "restore-all",
				Usage: "resotres all values from input file",
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
					sess := NewSessionWithSharedProfile(profile)
					inFileName := c.String("input-file-name")
					if inFileName == "" {
						return fmt.Errorf("input-file-name not found")
					}
					jsonFile, err := os.Open(inFileName)
					if err != nil {
						return fmt.Errorf("error when opening %s", inFileName)
					}
					defer jsonFile.Close()

					ssmc := secretsmanager.New(sess)

					var secrets []*Secret
					byteValue, _ := ioutil.ReadAll(jsonFile)
					if err := json.Unmarshal(byteValue, &secrets); err != nil {
						return err
					}

					for _, secret := range secrets {
						restoreInput := secretsmanager.RestoreSecretInput{
							SecretId: &secret.Name,
						}
						_, err := ssmc.RestoreSecret(&restoreInput)
						if err != nil {
							panic(err)
						}
						fmt.Printf("restored: %s\n", secret.Name)
					}
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

					var secrets []*Secret
					byteValue, _ := ioutil.ReadAll(jsonFile)
					if err := json.Unmarshal(byteValue, &secrets); err != nil {
						return err
					}

					var inputs []*secretsmanager.CreateSecretInput
					for _, secret := range secrets {
						if secret.Type == "value" || secret.Type == "password" {
							input := secretsmanager.CreateSecretInput{
								Name:         aws.String(secret.Name),
								SecretString: aws.String(secret.Value),
							}
							inputs = append(inputs, &input)
						} else if secret.Type == "rsa" {
							var rsa []byte
							if rsa, err = json.Marshal(secret.RSAValue); err != nil {
								return err
							}

							input := secretsmanager.CreateSecretInput{
								Name:         aws.String(secret.Name),
								SecretBinary: rsa,
							}
							inputs = append(inputs, &input)
						} else {
							fmt.Printf("unable to process version: %v", secret.Type)
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
			{
				Name:  "update",
				Usage: "update secrets from file",
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

					var secrets []*Secret
					byteValue, _ := ioutil.ReadAll(jsonFile)
					if err := json.Unmarshal(byteValue, &secrets); err != nil {
						return err
					}

					var inputs []*secretsmanager.UpdateSecretInput
					for _, secret := range secrets {
						if secret.Type == "value" || secret.Type == "password" {
							input := secretsmanager.UpdateSecretInput{
								SecretId:     aws.String(secret.Name),
								SecretString: aws.String(secret.Value),
							}
							inputs = append(inputs, &input)
						} else if secret.Type == "rsa" {
							var rsa []byte
							if rsa, err = json.Marshal(secret.RSAValue); err != nil {
								return err
							}

							input := secretsmanager.UpdateSecretInput{
								SecretId:     aws.String(secret.Name),
								SecretBinary: rsa,
							}
							inputs = append(inputs, &input)
						} else {
							fmt.Printf("unable to process version: %v", secret.Type)
						}
					}

					sm := secretsmanager.New(sess)

					for _, input := range inputs {
						if _, err := sm.UpdateSecret(input); err != nil {
							return err
						}
					}

					return nil
				},
			},
		},
	}
}
