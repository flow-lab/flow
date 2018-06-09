package main

import (
	"github.com/urfave/cli"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/ssm"
	"encoding/json"
	"io/ioutil"
)

type Parameter struct {
	ParameterMetadata *ssm.ParameterMetadata
	ParameterValue    *ssm.Parameter
}

var ssmCommand = func() cli.Command {
	return cli.Command{
		Name: "ssm",
		Subcommands: []cli.Command{
			{
				Name:  "export",
				Usage: "exports ssm parameters and their values to json",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "output-file-name",
						Value: "ssm.json",
					},
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					outFileName := c.String("output-file-name")
					sess := session.Must(session.NewSessionWithOptions(session.Options{
						AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
						SharedConfigState:       session.SharedConfigEnable,
						Profile:                 profile,
					}))

					ssmc := ssm.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

					listTopicsParams := ssm.DescribeParametersInput{
					}
					var parameters []Parameter
					ssmc.DescribeParametersPages(&listTopicsParams, func(output *ssm.DescribeParametersOutput, lastPage bool) bool {
						for _, elem := range output.Parameters {

							param := ssm.GetParameterInput{
								Name: elem.Name,
								WithDecryption: func() *bool {
									var b = true
									return &b
								}(),
							}
							out, _ := ssmc.GetParameter(&param)

							parameters = append(parameters, Parameter{
								ParameterMetadata: elem,
								ParameterValue:    out.Parameter,
							})
						}
						return lastPage == false
					})

					b, _ := json.Marshal(parameters)
					_ = ioutil.WriteFile(outFileName, b, 0644)
					return nil
				},
			},
		},
	}
}
