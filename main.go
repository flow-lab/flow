package main

import (
	"log"
	"os"
	"github.com/urfave/cli"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
)

func main() {
	var profile string
	var tableName string
	var indexName string

	app := cli.NewApp()

	app.Commands = []cli.Command{
		{
			Name: "dynamodb",
			Subcommands: []cli.Command{
				{
					Name: "purge",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:        "profile",
							Value:       "",
							Destination: &profile,
						},
						cli.StringFlag{
							Name:        "table-name",
							Value:       "",
							Destination: &tableName,
						},
						cli.StringFlag{
							Name:        "index-name",
							Value:       "id",
							Destination: &indexName,
						},
					},
					Action: func(c *cli.Context) error {
						sess := session.Must(session.NewSessionWithOptions(session.Options{
							AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
							SharedConfigState:       session.SharedConfigEnable,
							Profile:                 profile,
						}))

						ddbc := dynamodb.New(sess, &aws.Config{
							Region: aws.String(endpoints.EuWest1RegionID),
						})

						params := dynamodb.ScanInput{
							TableName: &tableName,
						}
						err := ddbc.ScanPages(&params, func(page *dynamodb.ScanOutput, lastPage bool) bool {
							for _, element := range page.Items {
								deleteItemParam := dynamodb.DeleteItemInput{
									TableName: &tableName,
									Key:       element,
								}
								_, err := ddbc.DeleteItem(&deleteItemParam)
								if err != nil {
									fmt.Errorf("%e", err)
								}
							}

							return lastPage == false
						})

						if err != nil {
							return err
						}

						return nil
					},
				},
			},
		},
	}

	app.Action = func(c *cli.Context) error {

		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
