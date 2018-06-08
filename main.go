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
	"github.com/aws/aws-sdk-go/service/sqs"
	"strings"
	"strconv"
	"sync"
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

						purge := func(c *cli.Context, elements []map[string]*dynamodb.AttributeValue, tableName string, ddbc *dynamodb.DynamoDB) {
							var wg sync.WaitGroup
							wg.Add(len(elements))
							for _, element := range elements {
								key := map[string]*dynamodb.AttributeValue{}
								indexes := c.StringSlice("index")
								for _, index := range indexes {
									key[index] = element[index]
								}
								go func() {
									fmt.Print(".")
									defer wg.Done()
									deleteItemParam := dynamodb.DeleteItemInput{
										TableName: &tableName,
										Key:       key,
									}
									_, err := ddbc.DeleteItem(&deleteItemParam)
									if err != nil {
										fmt.Printf("%v", err)
									}
								}()
							}

							wg.Wait()
						}

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
					Name: "capacity",
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
		},

		{
			Name: "sqs",
			Subcommands: []cli.Command{
				{
					Name: "purge",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "profile",
							Value: "",
						},
						cli.StringFlag{
							Name:  "queue-name",
							Value: "",
						},
					},
					Action: func(c *cli.Context) error {
						profile := c.String("profile")
						queueName := c.String("queue-name")
						sess := session.Must(session.NewSessionWithOptions(session.Options{
							AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
							SharedConfigState:       session.SharedConfigEnable,
							Profile:                 profile,
						}))

						sqsc := sqs.New(sess, &aws.Config{
							Region: aws.String(endpoints.EuWest1RegionID),
						})

						params := sqs.ListQueuesInput{
						}
						resp, err := sqsc.ListQueues(&params)
						if err != nil {
							return err
						}

						for _, elem := range resp.QueueUrls {
							if strings.Contains(*elem, queueName) {
								purgeQueue := sqs.PurgeQueueInput{
									QueueUrl: elem,
								}
								_, err := sqsc.PurgeQueue(&purgeQueue)
								if err != nil {
									return err
								}
								fmt.Printf("Purged %v", queueName)
							}
						}

						return nil
					},
				},
				{
					Name: "describe",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "profile",
							Value: "",
						},
						cli.StringFlag{
							Name:  "queue-name",
							Value: "",
						},
						cli.StringSliceFlag{
							Name: "attribute-names",
							Value: func() *cli.StringSlice {
								ss := &cli.StringSlice{}
								ss.Set("All")
								return ss
							}(),
						},
					},
					Action: func(c *cli.Context) error {
						profile := c.String("profile")
						queueName := c.String("queue-name")
						var attributeNames []*string
						for _, elem := range c.StringSlice("attribute-names") {
							attributeNames = append(attributeNames, &elem)
						}

						sess := session.Must(session.NewSessionWithOptions(session.Options{
							AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
							SharedConfigState:       session.SharedConfigEnable,
							Profile:                 profile,
						}))

						sqsc := sqs.New(sess, &aws.Config{
							Region: aws.String(endpoints.EuWest1RegionID),
						})

						params := sqs.ListQueuesInput{
						}
						resp, err := sqsc.ListQueues(&params)
						if err != nil {
							return err
						}

						for _, elem := range resp.QueueUrls {
							if strings.Contains(*elem, queueName) {
								purgeQueue := sqs.GetQueueAttributesInput{
									QueueUrl:       elem,
									AttributeNames: attributeNames,
								}
								output, err := sqsc.GetQueueAttributes(&purgeQueue)
								if err != nil {
									return err
								}
								fmt.Printf("%v", output.String())
							}
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
