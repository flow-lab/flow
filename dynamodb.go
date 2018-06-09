package main

import (
	"github.com/urfave/cli"
	"sync"
	"fmt"
	"strconv"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws/endpoints"
)

var dynamodbCommand = func() cli.Command {
	return cli.Command{
		Name: "dynamodb",
		Subcommands: []cli.Command{
			{
				Name:  "purge",
				Usage: "fast purge dynamodb using scan operation",
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
						Name: "key",
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

					var wg sync.WaitGroup
					purge := func(c *cli.Context, elements []map[string]*dynamodb.AttributeValue, tableName string, ddbc *dynamodb.DynamoDB) {
						wg.Add(1)
						go func() {
							defer wg.Done()
							for _, element := range elements {
								key := map[string]*dynamodb.AttributeValue{}
								keys := c.StringSlice("key")
								for _, index := range keys {
									key[index] = element[index]
								}

								deleteItemParam := dynamodb.DeleteItemInput{
									TableName: &tableName,
									Key:       key,
								}
								_, err := ddbc.DeleteItem(&deleteItemParam)
								if err != nil {
									fmt.Printf("%v", err)
								}
							}
						}()
					}
					wg.Wait()
					fmt.Println()

					params := dynamodb.ScanInput{
						TableName: &tableName,
					}
					err := ddbc.ScanPages(&params, func(output *dynamodb.ScanOutput, b bool) bool {
						purge(c, output.Items, tableName, ddbc)
						return b == false
					})
					if err != nil {
						return err
					}

					return nil
				},
			},

			{
				Name:  "capacity",
				Usage: "update read and write capacity",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
					cli.StringFlag{
						Name:  "table-name",
						Value: "",
					},
					cli.StringFlag{
						Name:  "read",
						Value: "10",
					},
					cli.StringFlag{
						Name:  "write",
						Value: "10",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					tableName := c.String("table-name")

					read, err := strconv.ParseInt(c.String("read"), 10, 0)
					if err != nil {
						return err
					}

					write, err := strconv.ParseInt(c.String("write"), 10, 0)
					if err != nil {
						return err
					}

					sess := session.Must(session.NewSessionWithOptions(session.Options{
						AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
						SharedConfigState:       session.SharedConfigEnable,
						Profile:                 profile,
					}))

					ddbc := dynamodb.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

					input := &dynamodb.UpdateTableInput{
						ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
							ReadCapacityUnits:  aws.Int64(read),
							WriteCapacityUnits: aws.Int64(write),
						},
						TableName: aws.String(tableName),
					}

					update, err := ddbc.UpdateTable(input)
					fmt.Printf("updated %v", update)

					if err != nil {
						return err
					}

					return nil
				},
			},
		},
	}
}
