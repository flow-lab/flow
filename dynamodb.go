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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"time"
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
					purge := func(items []map[string]*dynamodb.AttributeValue, pageNr int) {
						defer wg.Done()

						for _, element := range items {
							key := map[string]*dynamodb.AttributeValue{}
							keys := c.StringSlice("key")
							for _, myKey := range keys {
								key[myKey] = element[myKey]
							}

							deleteItemParam := dynamodb.DeleteItemInput{
								TableName: &tableName,
								Key:       key,
							}
							retry := 1
							for {
								_, err := ddbc.DeleteItem(&deleteItemParam)
								if err != nil {
									if aerr, ok := err.(awserr.Error); ok {
										switch aerr.Code() {
										case dynamodb.ErrCodeConditionalCheckFailedException:
											fmt.Println(dynamodb.ErrCodeConditionalCheckFailedException, aerr.Error())
										case dynamodb.ErrCodeProvisionedThroughputExceededException:
											fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
										case dynamodb.ErrCodeResourceNotFoundException:
											fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
										case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
											fmt.Println(dynamodb.ErrCodeItemCollectionSizeLimitExceededException, aerr.Error())
										case dynamodb.ErrCodeInternalServerError:
											fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
										default:
											fmt.Println(aerr.Error())
										}
									} else {
										fmt.Println(err.Error())
									}
									sleepTime := retry * retry * 100
									time.Sleep(time.Duration(sleepTime) * time.Millisecond)
									retry = retry + 1
									fmt.Printf("Sleeping for %d milliseconds. Retry DeleteItem... \n", sleepTime)
								} else {
									break
								}
							}
						}
						fmt.Printf("deleted: %d\n", len(items))
					}

					params := dynamodb.ScanInput{
						TableName: &tableName,
					}
					pageNr := 0
					err := ddbc.ScanPages(&params, func(output *dynamodb.ScanOutput, b bool) bool {
						targetMap := make([]map[string]*dynamodb.AttributeValue, len(output.Items))
						for key, value := range output.Items {
							targetMap[key] = value
						}
						wg.Add(1)
						go purge(output.Items, pageNr)
						pageNr += 1
						return b == false
					})
					if err != nil {
						return err
					}

					wg.Wait()

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
					cli.StringSliceFlag{
						Name: "global-secondary-index",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					tableName := c.String("table-name")
					gsis := c.StringSlice("global-secondary-index")

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

					var input *dynamodb.UpdateTableInput

					if len(gsis) > 0 {
						var gsiu []*dynamodb.GlobalSecondaryIndexUpdate
						for _, indexName := range gsis {
							gsiu = append(gsiu, &dynamodb.GlobalSecondaryIndexUpdate{
								Update: &dynamodb.UpdateGlobalSecondaryIndexAction{
									IndexName: &indexName,
									ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
										ReadCapacityUnits:  aws.Int64(read),
										WriteCapacityUnits: aws.Int64(write),
									},
								},
							})
						}

						input = &dynamodb.UpdateTableInput{
							ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
								ReadCapacityUnits:  aws.Int64(read),
								WriteCapacityUnits: aws.Int64(write),
							},
							GlobalSecondaryIndexUpdates: gsiu,
							TableName:                   aws.String(tableName),
						}
					} else {
						input = &dynamodb.UpdateTableInput{
							ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
								ReadCapacityUnits:  aws.Int64(read),
								WriteCapacityUnits: aws.Int64(write),
							},
							TableName: aws.String(tableName),
						}
					}

					update, err := ddbc.UpdateTable(input)
					fmt.Printf("updated %v", update)

					if err != nil {
						return err
					}

					return nil
				},
			},
			{
				Name:  "describe-table",
				Usage: "get table details",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
					cli.StringFlag{
						Name:  "table-name",
						Value: "",
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

					input := dynamodb.DescribeTableInput{
						TableName: &tableName,
					}

					output, err := ddbc.DescribeTable(&input)
					fmt.Printf("updated %v", output)

					if err != nil {
						return err
					}

					return nil
				},
			},
		},
	}
}
