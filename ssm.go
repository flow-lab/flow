package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/urfave/cli"
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
					sess := NewSessionWithSharedProfile(profile)

					ssmc := ssm.New(sess)

					listTopicsParams := ssm.DescribeParametersInput{}
					var parameters []Parameter
					err := ssmc.DescribeParametersPages(&listTopicsParams, func(output *ssm.DescribeParametersOutput, lastPage bool) bool {
						fmt.Println(output.Parameters)
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
					if err != nil {
						panic(err)
					}

					b, err := json.Marshal(parameters)
					if err != nil {
						panic(err)
					}
					_ = ioutil.WriteFile(outFileName, b, 0644)
					fmt.Printf("wrote to %v", outFileName)
					return nil
				},
			},
		},
	}
}
