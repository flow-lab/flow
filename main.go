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
	app := cli.NewApp()

	app.Commands = []cli.Command{
		{
			Name: "dynamodb",
			Subcommands: []cli.Command{
				{
					Name: "purge",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "profile",
							Value: "",
						},
						cli.StringFlag{
							Name:  "table-name",
							Value: "",
						},
						cli.StringSliceFlag{
							Name: "index",
						},
					},
					Action: func(c *cli.Context) error {
						profile := c.String("profile")
						tableName := c.String("table-name")
						sess := session.Must(session.NewSessionWithOptions(session.Options{
							AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
							SharedConfigState:       session.SharedConfigEnable,
							Profile:                 profile,
						}))

						ddbc := dynamodb.New(sess, &aws.Config{
							Region: aws.String(endpoints.EuWest1RegionID),
						})

						counter := 0
						params := dynamodb.ScanInput{
							TableName: &tableName,
						}
						err := ddbc.ScanPages(&params, func(page *dynamodb.ScanOutput, lastPage bool) bool {
							for _, element := range page.Items {
								key := map[string]*dynamodb.AttributeValue{}
								indexes := c.StringSlice("index")
								for _, index := range indexes {
									key[index] = element[index]
								}

								fmt.Printf("delete: %v", key)
								deleteItemParam := dynamodb.DeleteItemInput{
									TableName: &tableName,
									Key:       key,
								}
								_, err := ddbc.DeleteItem(&deleteItemParam)
								if err != nil {
									fmt.Printf("%v", err)
								} else {
									counter += 1
								}
							}

							return lastPage == false
						})

						fmt.Printf("Deleted %d", counter)

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
