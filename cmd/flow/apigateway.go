package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/flow-lab/flow/pkg/session"
	"github.com/urfave/cli"
	"io/ioutil"
	"strings"
)

var apiGateway = func() cli.Command {
	return cli.Command{
		Name: "apigateway",
		Subcommands: []cli.Command{
			{
				Name:        "export",
				Description: "exports all API specifications in oas3 and saves to files",
				Usage:       "export specifications ",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "file-type",
						Value: "yaml",
					},

					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					fileType := c.String("file-type")

					sess := session.NewSessionWithSharedProfile(profile)
					apig := apigateway.New(sess)

					input := apigateway.GetRestApisInput{}
					output, err := apig.GetRestApis(&input)
					if err != nil {
						panic(err)
					}

					if len(output.Items) > 0 {
						for _, restAPI := range output.Items {
							getStageInput := apigateway.GetStagesInput{
								RestApiId: restAPI.Id,
							}
							getStagesOutput, err := apig.GetStages(&getStageInput)
							if err != nil {
								panic(err)
							}

							for _, deployment := range getStagesOutput.Item {
								exportInput := apigateway.GetExportInput{
									Accepts:    aws.String(fmt.Sprintf("application/%s", fileType)),
									RestApiId:  restAPI.Id,
									StageName:  deployment.StageName,
									ExportType: aws.String("oas30"),
									Parameters: map[string]*string{
										"extensions": aws.String("documentation"),
									},
								}
								getExportOutput, err := apig.GetExport(&exportInput)
								if err != nil {
									panic(err)
								}

								var destFileName string
								name := strings.Replace(*restAPI.Name, " ", "", -1)
								if restAPI.Version != nil {
									destFileName = fmt.Sprintf("%s-%s.oas3.yml", name, *restAPI.Version)
								} else {
									destFileName = fmt.Sprintf("%s.oas3.yml", name)
								}

								fmt.Printf("saved: %s\n", destFileName)
								err = ioutil.WriteFile(destFileName, getExportOutput.Body, 0644)
								if err != nil {
									panic(err)
								}
							}
						}
					}

					return nil
				},
			},
		},
	}
}
