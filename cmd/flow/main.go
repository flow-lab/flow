package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	asession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/aws/aws-sdk-go/service/cloudtrail"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/kafka"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	flowbase64 "github.com/flow-lab/flow/internal/base64"
	"github.com/flow-lab/flow/internal/creds"
	flowdynamo "github.com/flow-lab/flow/internal/dynamodb"
	flowkafka "github.com/flow-lab/flow/internal/kafka"
	"github.com/flow-lab/flow/internal/logs"
	"github.com/flow-lab/flow/internal/msk"
	flowpubsub "github.com/flow-lab/flow/internal/pubsub"
	"github.com/flow-lab/flow/internal/reader"
	"github.com/flow-lab/flow/internal/session"
	flowsqs "github.com/flow-lab/flow/internal/sqs"
	flowsts "github.com/flow-lab/flow/internal/sts"
	"github.com/pkg/errors"
	vegeta "github.com/tsenart/vegeta/lib"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/urfave/cli/v2"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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

type Parameter struct {
	ParameterMetadata *ssm.ParameterMetadata
	ParameterValue    *ssm.Parameter
}

func main() {
	app := cli.NewApp()
	app.Name = "Flow"
	app.Version = version
	a := cli.Author{
		Name:  "Krzysztof Grodzicki",
		Email: "krzysztof@flowlab.no",
	}
	app.Authors = []*cli.Author{&a}
	app.Usage = "Development CLI"
	app.Description = fmt.Sprintf("flow cli. Commit %v, build at %v", commit, date)
	app.EnableBashCompletion = true

	app.Commands = []*cli.Command{
		func() *cli.Command {
			return &cli.Command{
				Name:  "dynamodb",
				Usage: "AWS DynamoDB",
				Subcommands: []*cli.Command{
					{
						Name:  "delete",
						Usage: "delete item(s) from dynamodb using scan operation",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "filter-expression",
							},
							&cli.StringFlag{
								Name: "table-name",
							},
							&cli.StringFlag{
								Name: "expression-attribute-values",
							},
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")
							filterExpression := c.String("filter-expression")
							expressionAttributeValues := c.String("expression-attribute-values")
							sess := session.NewSessionWithSharedProfile(profile)
							ddbc := dynamodb.New(sess)

							if tableName == "" {
								return fmt.Errorf("table-name is required")
							}

							fc, err := flowdynamo.NewFlowDynamoDBClient(ddbc)
							if err != nil {
								return err
							}

							var filterExpressionPtr *string
							if filterExpression != "" {
								filterExpressionPtr = &filterExpression
							}

							var expressionAttributeValuesPtr *string
							if expressionAttributeValues != "" {
								expressionAttributeValuesPtr = &expressionAttributeValues
							}

							return fc.Delete(context.Background(), tableName, filterExpressionPtr, expressionAttributeValuesPtr)
						},
					},
					{
						Name:  "capacity",
						Usage: "update read and write capacity",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							&cli.StringFlag{
								Name: "table-name",
							},
							&cli.Int64Flag{
								Name:  "read",
								Value: int64(10),
							},
							&cli.Int64Flag{
								Name:  "write",
								Value: int64(10),
							},
							&cli.StringSliceFlag{
								Name: "global-secondary-index",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")
							read := c.Int64("read")
							write := c.Int64("write")
							gsis := c.StringSlice("global-secondary-index")

							if tableName == "" {
								return fmt.Errorf("table-name is required")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							ddbc := dynamodb.New(sess)

							var input dynamodb.UpdateTableInput
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

								input = dynamodb.UpdateTableInput{
									GlobalSecondaryIndexUpdates: gsiu,
									TableName:                   aws.String(tableName),
								}
							} else {
								input = dynamodb.UpdateTableInput{
									ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
										ReadCapacityUnits:  aws.Int64(read),
										WriteCapacityUnits: aws.Int64(write),
									},
									TableName: aws.String(tableName),
								}
							}

							update, err := ddbc.UpdateTable(&input)
							fmt.Printf("updated %v", update)

							return err
						},
					},
					{
						Name:  "describe-table",
						Usage: "get table details",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")

							if tableName == "" {
								return fmt.Errorf("table-name is required")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							ddbc := dynamodb.New(sess)
							input := dynamodb.DescribeTableInput{
								TableName: &tableName,
							}
							output, err := ddbc.DescribeTable(&input)
							fmt.Printf("%v", output)

							return err
						},
					},
					{
						Name:  "count-item",
						Usage: "counts elements in table using scan operation",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")

							if tableName == "" {
								return fmt.Errorf("table-name is required")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							ddbc := dynamodb.New(sess)

							params := dynamodb.ScanInput{
								TableName: &tableName,
							}
							nrOfItems := 0
							err := ddbc.ScanPages(&params, func(output *dynamodb.ScanOutput, b bool) bool {
								nrOfItems += len(output.Items)
								return b == false
							})

							fmt.Printf("nr of items: %v", nrOfItems)

							if err != nil {
								return err
							}

							return nil
						},
					},
					{
						Name:  "put-item",
						Usage: "put item(s)",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "input",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")
							input := c.String("input")
							var items []map[string]*dynamodb.AttributeValue

							if tableName == "" {
								return fmt.Errorf("missing --table-name parameter")
							}
							if input == "" {
								return fmt.Errorf("missing --input parameter")
							}

							jsonFile, err := os.Open(input)
							if err != nil {
								fmt.Printf("Error when opening %v \n", input)
								return err
							}
							defer jsonFile.Close()

							byteValue, _ := ioutil.ReadAll(jsonFile)
							err = json.Unmarshal(byteValue, &items)
							if err != nil {
								return err
							}

							sess := session.NewSessionWithSharedProfile(profile)

							ddbc := dynamodb.New(sess)

							var batches [][]map[string]*dynamodb.AttributeValue
							batchSize := 25
							for batchSize < len(items) {
								items, batches = items[batchSize:], append(batches, items[0:batchSize:batchSize])
							}
							batches = append(batches, items)

							for _, batch := range batches {
								var wrs []*dynamodb.WriteRequest
								input := &dynamodb.BatchWriteItemInput{}
								for _, item := range batch {
									dr := dynamodb.PutRequest{
										Item: item,
									}
									wrs = append(wrs, &dynamodb.WriteRequest{
										PutRequest: &dr,
									})

									input.RequestItems = map[string][]*dynamodb.WriteRequest{tableName: wrs}
								}

								retry := 1
								for {
									res, err := ddbc.BatchWriteItem(input)
									if err != nil || len(res.UnprocessedItems) > 0 {
										if aerr, ok := err.(awserr.Error); ok {
											fmt.Println(aerr.Error())
										} else {
											fmt.Print("\nthrottling when batch write, consider updating write capacity. Going to retry ...")
										}
										sleepTime := retry * retry * 100
										time.Sleep(time.Duration(sleepTime) * time.Millisecond)
										retry = retry + 1
									} else {
										break
									}
								}
								for range batch {
									fmt.Print(".")
								}
							}

							return nil
						},
					},
					{
						Name:  "delete-item",
						Usage: "delete item(s)",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "input",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")
							input := c.String("input")
							var items []map[string]*dynamodb.AttributeValue

							if tableName == "" {
								return fmt.Errorf("missing --table-name parameter")
							}
							if input == "" {
								return fmt.Errorf("missing --input parameter")
							}

							jsonFile, err := os.Open(input)
							if err != nil {
								fmt.Printf("Error when opening %v \n", input)
								return err
							}
							defer jsonFile.Close()

							byteValue, _ := ioutil.ReadAll(jsonFile)
							err = json.Unmarshal(byteValue, &items)
							if err != nil {
								return err
							}

							sess := session.NewSessionWithSharedProfile(profile)

							ddbc := dynamodb.New(sess)

							var batches [][]map[string]*dynamodb.AttributeValue
							batchSize := 25
							for batchSize < len(items) {
								items, batches = items[batchSize:], append(batches, items[0:batchSize:batchSize])
							}
							batches = append(batches, items)

							for _, batch := range batches {
								var wrs []*dynamodb.WriteRequest
								for _, item := range batch {
									dr := dynamodb.DeleteRequest{
										Key: item,
									}
									wrs = append(wrs, &dynamodb.WriteRequest{
										DeleteRequest: &dr,
									})
								}

								input := &dynamodb.BatchWriteItemInput{
									RequestItems: map[string][]*dynamodb.WriteRequest{
										tableName: wrs,
									},
								}
								retry := 1
								for {
									res, err := ddbc.BatchWriteItem(input)
									if err != nil || len(res.UnprocessedItems) > 0 {
										if aerr, ok := err.(awserr.Error); ok {
											fmt.Println(aerr.Error())
										}
										sleepTime := retry * retry * 100
										time.Sleep(time.Duration(sleepTime) * time.Millisecond)
										retry = retry + 1
									} else {
										break
									}
								}
								fmt.Print(".")
							}

							return nil
						},
					},
					{
						Name:  "map-to-primary-key",
						Usage: "gets GSI keys and maps to Primary Keys using Query",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "secondary-index",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "projection-expression",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "keys",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "file-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")
							projectionExpression := c.String("projection-expression")
							keys := c.String("keys")
							fileName := c.String("file-name")
							secondaryIndex := c.String("secondary-index")
							var items []map[string]*dynamodb.AttributeValue

							if tableName == "" {
								return fmt.Errorf("missing --table-name parameter")
							}
							if keys == "" {
								return fmt.Errorf("missing --keys parameter")
							}

							jsonFile, err := os.Open(keys)
							if err != nil {
								fmt.Printf("Error when opening %v \n", keys)
								return err
							}
							defer jsonFile.Close()

							byteValue, _ := ioutil.ReadAll(jsonFile)
							err = json.Unmarshal(byteValue, &items)
							if err != nil {
								return err
							}

							sess := session.NewSessionWithSharedProfile(profile)
							ddbc := dynamodb.New(sess)

							var shouldWriteToFile bool
							if fileName != "" {
								shouldWriteToFile = true
							}

							writer := bufio.NewWriter(os.Stdout)
							defer writer.Flush()
							if !shouldWriteToFile {
								if _, err := writer.Write([]byte("[")); err != nil {
									return err
								}
							}

							itemsBefore := false
							var results []map[string]*dynamodb.AttributeValue
							for _, item := range items {
								query := &dynamodb.QueryInput{
									TableName: &tableName,
								}
								if secondaryIndex != "" {
									query.IndexName = &secondaryIndex
								}
								if projectionExpression != "" {
									query.ProjectionExpression = &projectionExpression
								}

								expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
								for key, val := range item {
									expressionAttributeValues[":"+key] = val
								}
								query.ExpressionAttributeValues = expressionAttributeValues

								var keyConditionExpression string
								for key := range item {
									if keyConditionExpression == "" {
										keyConditionExpression = fmt.Sprintf("%s = :%s", key, key)
									} else {
										keyConditionExpression = fmt.Sprintf(" AND %s = :%s", key, key)
									}
								}
								query.KeyConditionExpression = aws.String(keyConditionExpression)

								retry := 1
								for {
									queryOutput, err := ddbc.Query(query)
									if err != nil {
										if aerr, ok := err.(awserr.Error); ok {
											fmt.Println(aerr.Error())
										}
										sleepTime := retry * retry * 100
										time.Sleep(time.Duration(sleepTime) * time.Millisecond)
										retry = retry + 1
									} else {
										if !shouldWriteToFile {
											if *queryOutput.Count > 0 && itemsBefore {
												if _, err := writer.Write([]byte(",")); err != nil {
													panic(err)
												}
											}
										}
										itemsBefore = true

										for i, item := range queryOutput.Items {
											if shouldWriteToFile {
												results = append(results, item)
											} else {
												if j, err := json.Marshal(item); err == nil {
													if _, err := writer.Write(j); err != nil {
														fmt.Printf("%v", item)
														panic(err)
													}
												} else {
													fmt.Printf("%v", item)
													panic("unable to marshal")
												}

												if i < len(queryOutput.Items)-1 {
													if _, err := writer.Write([]byte(",")); err != nil {
														panic(err)
													}
												}
											}
										}

										break
									}
								}
								if shouldWriteToFile {
									fmt.Print(".")
								}
							}

							if shouldWriteToFile {
								var jso []byte
								var werr error
								if jso, werr = json.Marshal(results); werr == nil {
									if err := ioutil.WriteFile(fileName, jso, 0644); err != nil {
										return err
									}
									fmt.Printf("result wrote to: %v", fileName)
								} else {
									fmt.Printf("%v", werr)
									panic("unable to write to file")
								}
							} else {
								if _, err := writer.Write([]byte("]")); err != nil {
									return err
								}
							}

							return nil
						},
					},
					{
						Name:  "restore-table-to-point-in-time",
						Usage: "restore table to point in time",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "source-table-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "target-table-name",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							sourceTableName := c.String("source-table-name")
							targetTableName := c.String("target-table-name")

							sess := session.NewSessionWithSharedProfile(profile)

							ddbc := dynamodb.New(sess)

							input := dynamodb.RestoreTableToPointInTimeInput{
								SourceTableName: &sourceTableName,
								TargetTableName: &targetTableName,
								// TODO [grokrz]: parameter ?
								UseLatestRestorableTime: aws.Bool(true),
							}

							result, err := ddbc.RestoreTableToPointInTime(&input)
							if err != nil {
								if aerr, ok := err.(awserr.Error); ok {
									return aerr
								}
								return err
							}

							fmt.Printf("result: %v", result)

							return nil
						},
					},
					{
						Name:  "search",
						Usage: "search for records using scan operation",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "filter-expression",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "expression-attribute-values",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "projection-expression",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "file-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")
							filterExpression := c.String("filter-expression")
							expressionAttributeValues := c.String("expression-attribute-values")
							projectionExpression := c.String("projection-expression")
							fileName := c.String("file-name")
							sess := session.NewSessionWithSharedProfile(profile)

							ddbc := dynamodb.New(sess)

							params := dynamodb.ScanInput{
								TableName: &tableName,
							}

							if filterExpression != "" {
								params.FilterExpression = &filterExpression
							}

							if expressionAttributeValues != "" {
								var m map[string]*dynamodb.AttributeValue
								err := json.Unmarshal([]byte(expressionAttributeValues), &m)
								if err != nil {
									return err
								}
								params.ExpressionAttributeValues = m
							}

							if projectionExpression != "" {
								params.ProjectionExpression = &projectionExpression
							}

							var l []map[string]*dynamodb.AttributeValue
							var shouldWriteToFile bool
							if fileName != "" {
								shouldWriteToFile = true
							}

							writer := bufio.NewWriter(os.Stdout)
							defer writer.Flush()
							if !shouldWriteToFile {
								if _, err := writer.Write([]byte("[")); err != nil {
									return err
								}
							}

							itemsBefore := false
							err := ddbc.ScanPages(&params, func(output *dynamodb.ScanOutput, lastPage bool) bool {
								if *output.Count > int64(0) {
									// lets try to stream in comma
									if itemsBefore && !shouldWriteToFile {
										if _, err := writer.Write([]byte(",")); err != nil {
											panic(err)
										}
									}
									// ok, there is one record in the list before
									itemsBefore = true

									for i, elem := range output.Items {
										if shouldWriteToFile {
											l = append(l, elem)
										} else {
											if j, err := json.Marshal(elem); err == nil {
												if _, err := writer.Write(j); err != nil {
													fmt.Printf("%v", elem)
													panic(err)
												}
											} else {
												fmt.Printf("%v", elem)
												panic("unable to marshal")
											}

											if i < len(output.Items)-1 {
												if _, err := writer.Write([]byte(",")); err != nil {
													panic(err)
												}
											}
										}
									}
								}

								return lastPage == false
							})
							if err != nil {
								return err
							}

							if shouldWriteToFile {
								var jso []byte
								var werr error
								if jso, werr = json.Marshal(l); werr == nil {
									if err := ioutil.WriteFile(fileName, jso, 0644); err != nil {
										return err
									}
									fmt.Printf("result wrote to: %v", fileName)
								} else {
									fmt.Printf("%v", werr)
									panic("unable to write to file")
								}
							} else {
								if _, err := writer.Write([]byte("]")); err != nil {
									return err
								}
							}

							return nil
						},
					},
					{
						Name:  "delete-backup",
						Usage: "delete backup(s)",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							&cli.StringSliceFlag{
								Name: "table-name",
							},
							&cli.StringFlag{
								Name:  "time-range-upper-bound",
								Value: "",
								Usage: "only backups created before this time will be deleted. It is exclusive",
							},
							&cli.StringFlag{
								Name:  "older-than",
								Value: "",
								Usage: "age in days",
							},
							&cli.StringFlag{
								Name:  "backup-type",
								Value: "USER",
								Usage: "USER, SYSTEM or ALL",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableNames := c.StringSlice("table-name")
							timeRangeUpperBound := c.String("time-range-upper-bound")
							olderThanDays := c.String("older-than")
							backupType := c.String("backup-type")

							if olderThanDays != "" && timeRangeUpperBound != "" {
								return fmt.Errorf("only one of paramters --time-range-upper-bound, --older-than can be provided")
							}

							var t time.Time
							if timeRangeUpperBound == "" {
								t = time.Now()
							} else {
								var err error
								t, err = time.Parse(time.RFC3339, timeRangeUpperBound)
								if err != nil {
									return err
								}
							}

							if olderThanDays != "" {
								days, err := strconv.Atoi(olderThanDays)
								if err != nil {
									return err
								}
								t = t.AddDate(0, 0, -days)
							}

							sess := session.NewSessionWithSharedProfile(profile)
							ddbc := dynamodb.New(sess)

							for _, tableName := range tableNames {
								var listBackupsOutput *dynamodb.ListBackupsOutput
								for {
									listBackupsInput := &dynamodb.ListBackupsInput{
										TableName:           &tableName,
										TimeRangeUpperBound: &t,
										BackupType:          &backupType,
									}

									if listBackupsOutput != nil {
										listBackupsInput.ExclusiveStartBackupArn = listBackupsOutput.LastEvaluatedBackupArn
									}

									var err error
									listBackupsOutput, err = ddbc.ListBackups(listBackupsInput)
									if err != nil {
										return err
									}

									for _, bs := range listBackupsOutput.BackupSummaries {
										deleteBackupInput := &dynamodb.DeleteBackupInput{
											BackupArn: bs.BackupArn,
										}
										output, err := ddbc.DeleteBackup(deleteBackupInput)
										if err != nil {
											panic(err)
										}
										//we need to sleep a bit, max 10 times per second
										time.Sleep(time.Duration(100) * time.Millisecond)
										fmt.Printf("deleted: %v \n", *output.BackupDescription.BackupDetails)
									}

									if listBackupsOutput.LastEvaluatedBackupArn == nil {
										break
									}
								}
							}

							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "sqs",
				Usage: "AWS SQS",
				Subcommands: []*cli.Command{
					{
						Name:  "purge",
						Usage: "purge all messages",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "queue-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							queueName := c.String("queue-name")
							sess := session.NewSessionWithSharedProfile(profile)

							sqsc := sqs.New(sess)

							params := sqs.ListQueuesInput{}
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
						Name:  "send",
						Usage: "send message to sqs",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "queue-name",
								Value: "",
							},
							&cli.StringFlag{
								Name: "input-file-name",
							},
							&cli.StringFlag{
								Name: "input",
							},
							&cli.StringFlag{
								Name:  "message-attributes",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							queueName := c.String("queue-name")
							inputStr := c.String("input")
							inFileName := c.String("input-file-name")

							if inputStr == "" && inFileName == "" {
								return fmt.Errorf("input-string or input-file-name is required")
							}

							msgAttributes := c.String("message-attributes")
							sess := session.NewSessionWithSharedProfile(profile)

							var byteValue []byte
							if inFileName != "" {
								jsonFile, err := os.Open(inFileName)
								if err != nil {
									return fmt.Errorf("error when opening %s", inFileName)
								}
								byteValue, err = ioutil.ReadAll(jsonFile)
								if err != nil {
									return err
								}
								defer jsonFile.Close()
							} else {
								byteValue = []byte(inputStr)
							}

							var messageAttributes map[string]*sqs.MessageAttributeValue
							if msgAttributes != "" {
								messageAttributes = map[string]*sqs.MessageAttributeValue{}
								err := json.Unmarshal([]byte(msgAttributes), &messageAttributes)
								if err != nil {
									return err
								}
							}

							sqsc := sqs.New(sess)
							params := sqs.ListQueuesInput{}
							resp, err := sqsc.ListQueues(&params)
							if err != nil {
								return err
							}

							var q *string
							for _, elem := range resp.QueueUrls {
								if strings.Contains(*elem, queueName) {
									q = elem
								}
							}
							if q == nil {
								return fmt.Errorf("queue %s does not exist", queueName)
							}

							smi := sqs.SendMessageInput{
								QueueUrl: q,
								MessageBody: func() *string {
									s := string(byteValue[:])
									return &s
								}(),
								MessageAttributes: messageAttributes,
							}
							_, err = sqsc.SendMessage(&smi)
							if err != nil {
								return err
							}
							fmt.Printf("Sent to %v", queueName)

							return nil
						},
					},
					{
						Name:  "describe",
						Usage: "get all attributes",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "queue-name",
								Value: "",
							},
							&cli.StringSliceFlag{
								Name: "attribute-names",
								Value: func() *cli.StringSlice {
									ss := &cli.StringSlice{}
									_ = ss.Set("All")
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

							sess := session.NewSessionWithSharedProfile(profile)

							sqsc := sqs.New(sess)

							params := sqs.ListQueuesInput{}
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
					{
						Name:  "receive-message",
						Usage: "receive-message",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "queue-name",
								Value: "",
							},
							&cli.Int64Flag{
								Name:  "max-number-of-messages",
								Value: 10,
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							queueName := c.String("queue-name")
							maxNumberOfMessages := c.Int64("max-number-of-messages")
							sess := session.NewSessionWithSharedProfile(profile)

							sqsc := sqs.New(sess)

							params := sqs.ListQueuesInput{}
							resp, err := sqsc.ListQueues(&params)
							if err != nil {
								return err
							}

							for _, elem := range resp.QueueUrls {
								if strings.Contains(*elem, queueName) {
									params := sqs.ReceiveMessageInput{
										QueueUrl:            elem,
										MaxNumberOfMessages: aws.Int64(maxNumberOfMessages),
										AttributeNames:      []*string{aws.String("All")},
									}
									resp, err := sqsc.ReceiveMessage(&params)
									if err != nil {
										return err
									}
									for _, msg := range resp.Messages {
										fmt.Printf("%v\n", msg)
									}
								}
							}

							return nil
						},
					},
					{
						Name:  "delete-message",
						Usage: "delete-message",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "queue-name",
								Value: "",
							},
							&cli.StringSliceFlag{
								Name: "receipt-handle",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							queueName := c.String("queue-name")
							receiptHandles := c.StringSlice("receipt-handle")
							sess := session.NewSessionWithSharedProfile(profile)

							if queueName == "" {
								return fmt.Errorf("queue-name is required")
							}
							if len(receiptHandles) < 1 {
								return fmt.Errorf("receipt-handle is required")
							}

							sqsc := sqs.New(sess)
							client, err := flowsqs.NewSQSClient(sqsc)
							if err != nil {
								return err
							}

							return client.Delete(context.Background(), queueName, receiptHandles)
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "sns",
				Usage: "AWS SNS",
				Subcommands: []*cli.Command{
					{
						Name:  "publish",
						Usage: "publish many messages",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "topic-name",
								Value: "",
							},
							&cli.StringFlag{
								Name: "message",
							},
							&cli.StringFlag{
								Name:  "times",
								Value: "1",
							},
							&cli.StringFlag{
								Name:  "delay",
								Value: "0",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							topicName := c.String("topic-name")
							message := c.String("message")
							times, err := strconv.Atoi(c.String("times"))
							if err != nil {
								return err
							}
							delay, err := strconv.ParseInt(c.String("delay"), 10, 0)
							if err != nil {
								return err
							}
							sess := session.NewSessionWithSharedProfile(profile)

							sqsc := sns.New(sess)

							var wg sync.WaitGroup
							listTopicsParams := sns.ListTopicsInput{}
							out, err := sqsc.ListTopics(&listTopicsParams)
							if err != nil {
								return err
							}
							for _, topic := range out.Topics {
								if strings.Contains(*topic.TopicArn, topicName) {
									for i := 0; i < times; i++ {
										wg.Add(1)
										go func(topicArn string) {
											defer wg.Done()
											params := sns.PublishInput{
												Message:  &message,
												TopicArn: &topicArn,
											}
											fmt.Println(params)
											_, err := sqsc.Publish(&params)

											if err != nil {
												fmt.Println("Unable to send to sns", err.Error())
											}

											fmt.Print(".")

											time.Sleep(time.Duration(delay) * time.Millisecond)
										}(*topic.TopicArn)
									}
								}
							}

							wg.Wait()
							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "cloudwatch",
				Usage: "AWS CloudWatch",
				Subcommands: []*cli.Command{
					{
						Name:  "delete-alarm",
						Usage: "deletes cloudwatch alarm(s)",
						Flags: []cli.Flag{
							&cli.StringSliceFlag{
								Name: "name",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							var alarmNames []*string
							for _, elem := range c.StringSlice("name") {
								alarmNames = append(alarmNames, &elem)
							}
							sess := session.NewSessionWithSharedProfile(profile)

							cwlc := cloudwatch.New(sess)

							params := cloudwatch.DeleteAlarmsInput{
								AlarmNames: alarmNames,
							}
							_, err := cwlc.DeleteAlarms(&params)
							if err != nil {
								return err
							}
							fmt.Printf("ok")

							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "cloudwatchlogs",
				Usage: "AWS CloudWatch Logs",
				Subcommands: []*cli.Command{
					{
						Name:  "retention",
						Usage: "set log group retention in days",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "log-group-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "days",
								Usage: "retention in days",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							logGroupName := c.String("log-group-name")
							days := c.String("days")
							if days == "" {
								return fmt.Errorf("days is required")
							}

							retention, err := strconv.ParseInt(days, 10, 0)
							if err != nil {
								return err
							}
							if logGroupName == "" {
								return fmt.Errorf("log-group-name is required")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							cwlc := cloudwatchlogs.New(sess)
							return logs.SetRetention(logGroupName, retention, cwlc)
						},
					},
					{
						Name:  "write-to-file",
						Usage: "writes events to file in json format",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "log-group-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "file-name",
								Usage: "output file name",
								Value: "output",
							},
							&cli.StringFlag{
								Name:  "filter-pattern",
								Usage: "the filter pattern to use. If not provided, all the events are matched",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							logGroupName := c.String("log-group-name")
							fileName := c.String("file-name")
							filterPattern := c.String("filter-pattern")

							sess := session.NewSessionWithSharedProfile(profile)

							cwlc := cloudwatchlogs.New(sess)

							params := cloudwatchlogs.FilterLogEventsInput{
								LogGroupName:  &logGroupName,
								FilterPattern: &filterPattern,
							}

							pageNum := 0
							var logEvents []*cloudwatchlogs.FilteredLogEvent
							err := cwlc.FilterLogEventsPages(&params, func(page *cloudwatchlogs.FilterLogEventsOutput, lastPage bool) bool {
								pageNum++
								for _, event := range page.Events {
									logEvents = append(logEvents, event)
								}
								return lastPage == false
							})

							b, _ := json.Marshal(logEvents)
							err = ioutil.WriteFile(fileName, b, 0644)
							if err != nil {
								return err
							}

							return nil
						},
					},
					{
						Name:  "delete-subscription-filter",
						Usage: "delete subscription for log group",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "log-group-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "filter-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							logGroupName := c.String("log-group-name")
							if logGroupName == "" {
								return fmt.Errorf("log-group-name is required")
							}
							filterName := c.String("filter-name")
							if filterName == "" {
								return fmt.Errorf("filter-name is required")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							cwlc := cloudwatchlogs.New(sess)

							params := cloudwatchlogs.DeleteSubscriptionFilterInput{
								LogGroupName: &logGroupName,
								FilterName:   &filterName,
							}

							_, err := cwlc.DeleteSubscriptionFilter(&params)
							if err != nil {
								return fmt.Errorf("DeleteSubscriptionFilter failed: %s", err)
							}

							return nil
						},
					},
					{
						Name:  "delete-all-subscription-filters",
						Usage: "delete all subscriptions for log group",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "log-group-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							logGroupName := c.String("log-group-name")
							if logGroupName == "" {
								return fmt.Errorf("log-group-name is required")
							}
							profile := c.String("profile")

							sess := session.NewSessionWithSharedProfile(profile)
							cwlc := cloudwatchlogs.New(sess)

							describeSubscriptionFiltersInput := cloudwatchlogs.DescribeSubscriptionFiltersInput{
								LogGroupName: &logGroupName,
							}

							describeSubscriptionFiltersOutput, err := cwlc.DescribeSubscriptionFilters(&describeSubscriptionFiltersInput)
							if err != nil {
								return fmt.Errorf("DescribeSubscriptionFilters failed: %s", err)
							}

							for _, sf := range describeSubscriptionFiltersOutput.SubscriptionFilters {
								params := cloudwatchlogs.DeleteSubscriptionFilterInput{
									LogGroupName: &logGroupName,
									FilterName:   sf.FilterName,
								}

								_, err := cwlc.DeleteSubscriptionFilter(&params)
								if err != nil {
									return fmt.Errorf("DeleteSubscriptionFilter failed: %s", err)
								}
							}

							return nil
						},
					},
					{
						Name:  "describe",
						Usage: "describe log groups",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "log-group-name-prefix",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							logGroupNamePrefix := c.String("log-group-name-prefix")
							sess := session.NewSessionWithSharedProfile(profile)
							cwlc := cloudwatchlogs.New(sess)

							logGroups, err := logs.Describe(&logGroupNamePrefix, cwlc)
							if err != nil {
								return err
							}

							bytes, err := json.Marshal(logGroups)
							if err != nil {
								return err
							}

							fmt.Print(string(bytes))

							return nil
						},
					},
					{
						Name:  "summary",
						Usage: "summary log groups",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "log-group-name-prefix",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							sess := session.NewSessionWithSharedProfile(profile)
							cwlc := cloudwatchlogs.New(sess)
							logGroupNamePrefix := c.String("log-group-name-prefix")
							logGroups, err := logs.Describe(&logGroupNamePrefix, cwlc)
							if err != nil {
								return err
							}

							summary := logs.Summary(logGroups)
							bytes, err := json.Marshal(summary)
							if err != nil {
								return err
							}
							fmt.Print(string(bytes))

							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "ssm",
				Usage: "AWS SSM",
				Subcommands: []*cli.Command{
					{
						Name:  "export",
						Usage: "exports all ssm parameters and their values to json",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "output-file-name",
								Value: "ssm.json",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							outFileName := c.String("output-file-name")
							sess := session.NewSessionWithSharedProfile(profile)

							ssmc := ssm.New(sess)

							listTopicsParams := ssm.DescribeParametersInput{}
							var parameters []Parameter
							err := ssmc.DescribeParametersPages(&listTopicsParams, func(output *ssm.DescribeParametersOutput, lastPage bool) bool {
								fmt.Println(output.Parameters)
								for _, elem := range output.Parameters {
									param := ssm.GetParameterInput{
										Name:           elem.Name,
										WithDecryption: aws.Bool(true),
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
								return err
							}

							b, err := json.Marshal(parameters)
							if err != nil {
								return err
							}
							err = ioutil.WriteFile(outFileName, b, 0644)
							fmt.Printf("wrote to %v", outFileName)
							return err
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "secretsmanager",
				Usage: "AWS SecretsManager",
				Subcommands: []*cli.Command{
					{
						Name:        "get-secret-value",
						Description: "Gets secret value",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "secret-id",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							secretId := c.String("secret-id")
							profile := c.String("profile")

							if secretId == "" {
								return fmt.Errorf("secred-id is required")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							ssmc := secretsmanager.New(sess)

							getSecretValueInput := secretsmanager.GetSecretValueInput{
								SecretId: aws.String(secretId),
							}
							getSecretValueOutput, err := ssmc.GetSecretValue(&getSecretValueInput)
							if err != nil {
								panic(err)
							}

							fmt.Printf("%v", getSecretValueOutput)

							return nil
						},
					},
					{
						Name:  "export",
						Usage: "exports secrets and their values to json",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "output-file-name",
								Value: "secretsmanager.json",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							outFileName := c.String("output-file-name")

							sess := session.NewSessionWithSharedProfile(profile)

							ssmc := secretsmanager.New(sess)

							listSecretsInput := secretsmanager.ListSecretsInput{}
							var entries []*SecretOutput
							err := ssmc.ListSecretsPages(&listSecretsInput, func(output *secretsmanager.ListSecretsOutput, lastPage bool) bool {
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

							if err != nil {
								panic(err)
							}

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
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							sess := session.NewSessionWithSharedProfile(profile)

							ssmc := secretsmanager.New(sess)

							listSecretsInput := secretsmanager.ListSecretsInput{}
							err := ssmc.ListSecretsPages(&listSecretsInput, func(output *secretsmanager.ListSecretsOutput, lastPage bool) bool {
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

							if err != nil {
								panic(err)
							}

							return nil
						},
					},
					{
						Name:  "restore-all",
						Usage: "resotres all values from input file",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "input-file-name",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							sess := session.NewSessionWithSharedProfile(profile)
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
							&cli.StringFlag{
								Name: "input-file-name",
							},
							&cli.StringFlag{
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

							sess := session.NewSessionWithSharedProfile(profile)

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
							&cli.StringFlag{
								Name: "input-file-name",
							},
							&cli.StringFlag{
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

							sess := session.NewSessionWithSharedProfile(profile)

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
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "kinesis",
				Usage: "AWS Kinesis",
				Subcommands: []*cli.Command{
					{
						Name:  "update-shard-count",
						Usage: "update shard count",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "stream-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "count",
								Value: "1",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							streamName := c.String("stream-name")
							count, err := strconv.ParseInt(c.String("count"), 10, 0)
							if err != nil {
								return err
							}
							profile := c.String("profile")

							sess := session.NewSessionWithSharedProfile(profile)

							kinesisc := kinesis.New(sess)
							updateShardCountInput := kinesis.UpdateShardCountInput{
								StreamName:       &streamName,
								TargetShardCount: aws.Int64(count),
								ScalingType:      aws.String(kinesis.ScalingTypeUniformScaling),
							}
							_, err = kinesisc.UpdateShardCount(&updateShardCountInput)
							if err != nil {
								fmt.Println("Unable to update shard count", err.Error())
							}

							fmt.Print("done")

							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:        "base64",
				Description: "encoding/decoding base64",
				Usage:       "encoding/decoding base64",
				Subcommands: []*cli.Command{
					{
						Name:  "encode",
						Usage: "encodes string to base64",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "input",
							},
							&cli.StringFlag{
								Name: "file",
							},
						},
						Action: func(c *cli.Context) error {
							input := c.String("input")
							file := c.String("file")
							if input == "" && file == "" {
								return fmt.Errorf("--input or --file should be given")
							}

							if input != "" {
								encode := flowbase64.Encode(input)
								fmt.Println(string(encode))
							}

							if file != "" {
								b, err := ioutil.ReadFile(file)
								if err != nil {
									return err
								}
								encode := flowbase64.Encode(string(b))
								fmt.Println(string(encode))
							}

							return nil
						},
					},
					{
						Name:  "decode",
						Usage: "decodes base64 encoded string",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "input",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							input := c.String("input")
							if input == "" {
								return fmt.Errorf("missing --input")
							}

							bytes, err := flowbase64.Decode(input)
							if err != nil {
								return fmt.Errorf("call to Decode failed: %s", err)
							}
							fmt.Println(string(bytes))

							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "s3",
				Usage: "AWS S3",
				Subcommands: []*cli.Command{
					{
						Name:  "purge",
						Usage: "delete all objects and it versions",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "bucket-name",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "filter",
								Value: "",
								Usage: "delete only objects matching filter",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							bucketName := c.String("bucket-name")
							filter := c.String("filter")

							if bucketName == "" {
								return fmt.Errorf("missing --bucket-name parameter")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							s3c := s3.New(sess)

							input := s3.ListObjectVersionsInput{
								Bucket: aws.String(bucketName),
							}
							err := s3c.ListObjectVersionsPages(&input, func(output *s3.ListObjectVersionsOutput, b bool) bool {
								var objectIdentifiers []*s3.ObjectIdentifier
								for _, version := range output.Versions {
									if filter != "" {
										if strings.Contains(*version.Key, filter) {
											objectIdentifiers = append(objectIdentifiers, &s3.ObjectIdentifier{
												Key:       version.Key,
												VersionId: version.VersionId,
											})
										}
									} else {
										objectIdentifiers = append(objectIdentifiers, &s3.ObjectIdentifier{
											Key:       version.Key,
											VersionId: version.VersionId,
										})
									}
								}

								for _, deleteMarker := range output.DeleteMarkers {
									if filter != "" {
										if strings.Contains(*deleteMarker.Key, filter) {
											objectIdentifiers = append(objectIdentifiers, &s3.ObjectIdentifier{
												Key:       deleteMarker.Key,
												VersionId: deleteMarker.VersionId,
											})
										}
									} else {
										objectIdentifiers = append(objectIdentifiers, &s3.ObjectIdentifier{
											Key:       deleteMarker.Key,
											VersionId: deleteMarker.VersionId,
										})
									}
								}

								if len(objectIdentifiers) > 0 {
									deleteObjectsInput := s3.DeleteObjectsInput{
										Bucket: output.Name,
										Delete: &s3.Delete{
											Objects: objectIdentifiers,
										},
									}
									_, err := s3c.DeleteObjects(&deleteObjectsInput)
									if err != nil {
										panic(err)
									}
									fmt.Printf("deleted: %v\n", objectIdentifiers)
								}

								return output.NextKeyMarker != nil
							})
							if err != nil {
								return err
							}

							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "apigateway",
				Usage: "AWS API Gateway",
				Subcommands: []*cli.Command{
					{
						Name:        "export",
						Description: "exports all API specifications in oas3 and saves to files",
						Usage:       "export specifications ",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "file-type",
								Value: "yaml",
								Usage: "'json' or 'yaml'",
							},
							&cli.StringFlag{
								Name:  "export-type",
								Value: "oas30",
								Usage: "'oas30' for OpenAPI 3.0.x and 'swagger' for Swagger/OpenAPI 2.0",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							fileType := c.String("file-type")
							exportType := c.String("export-type")

							sess := session.NewSessionWithSharedProfile(profile)
							apig := apigateway.New(sess)

							input := apigateway.GetRestApisInput{}
							output, err := apig.GetRestApis(&input)
							if err != nil {
								return err
							}

							if len(output.Items) > 0 {
								for _, restAPI := range output.Items {
									getStageInput := apigateway.GetStagesInput{
										RestApiId: restAPI.Id,
									}
									getStagesOutput, err := apig.GetStages(&getStageInput)
									if err != nil {
										return err
									}

									for _, deployment := range getStagesOutput.Item {
										exportInput := apigateway.GetExportInput{
											Accepts:    aws.String(fmt.Sprintf("application/%s", fileType)),
											RestApiId:  restAPI.Id,
											StageName:  deployment.StageName,
											ExportType: aws.String(exportType),
											Parameters: map[string]*string{
												"extensions": aws.String("documentation"),
											},
										}
										getExportOutput, err := apig.GetExport(&exportInput)
										if err != nil {
											return err
										}

										var destFileName string
										name := strings.Replace(*restAPI.Name, " ", "", -1)
										if restAPI.Version != nil {
											destFileName = fmt.Sprintf("%s-%s.%s.yml", name, *restAPI.Version, exportType)
										} else {
											destFileName = fmt.Sprintf("%s.%s.yml", name, exportType)
										}

										err = ioutil.WriteFile(destFileName, getExportOutput.Body, 0644)
										fmt.Printf("saved: %s\n", destFileName)
										if err != nil {
											return err
										}
									}
								}
							}

							return nil
						},
					},
				},
			}
		}(),
		{
			Name:        "test",
			Description: "load test",
			Usage:       "HTTP load testing commands",
			Subcommands: []*cli.Command{
				{
					Name: "http",
					Flags: []cli.Flag{
						&cli.StringSliceFlag{
							Name:  "url",
							Usage: "url",
						},
						&cli.IntFlag{
							Name:  "frequency",
							Value: 1,
							Usage: "frequency (number of occurrences) per second",
						},
						&cli.StringFlag{
							Name:  "duration",
							Value: "1s",
							Usage: "a duration string is a possibly signed sequence of decimal numbers, each with " +
								"optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'. " +
								"Valid time units are 'ns', 'us' (or 'µs'), 'ms', 's', 'm', 'h'",
						},
						&cli.StringFlag{
							Name:  "authorization",
							Usage: "authorization header for all requests",
						},
					},
					Action: func(c *cli.Context) error {
						urls := c.StringSlice("url")
						f := c.Int("frequency")
						d := c.String("duration")
						a := c.String("authorization")

						if len(urls) == 0 {
							return fmt.Errorf("url is required")
						}
						if f <= 0 {
							return fmt.Errorf("frequency cannot be <= 0")
						}

						var tgts []vegeta.Target
						for _, u := range urls {
							t := vegeta.Target{
								URL:    u,
								Method: "GET",
							}
							if a != "" {
								t.Header = http.Header{
									"Authorization": []string{a},
								}
							}
							tgts = append(tgts, t)
						}

						st := vegeta.NewStaticTargeter(tgts...)
						attacker := vegeta.NewAttacker()

						var metrics vegeta.Metrics
						rate := vegeta.Rate{Freq: f, Per: time.Second}
						duration, err := time.ParseDuration(d)
						if err != nil {
							return err
						}

						fmt.Printf("running load test. Url: %v, Rate: %v, Duration: %v \n", urls, rate, duration)
						for res := range attacker.Attack(st, rate, duration, "Load test") {
							metrics.Add(res)
						}
						metrics.Close()
						reporter := vegeta.NewTextReporter(&metrics)
						return reporter.Report(os.Stdout)
					},
				},
			},
		},
		func() *cli.Command {
			return &cli.Command{
				Name:  "kafka",
				Usage: "AWS MSK",
				Subcommands: []*cli.Command{
					{
						Name:  "list-clusters",
						Usage: "",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)

							listClusterInput := kafka.ListClustersInput{}

							listClusterOutput, err := kc.ListClusters(&listClusterInput)
							if err != nil {
								return err
							}

							for _, cluster := range listClusterOutput.ClusterInfoList {
								fmt.Printf("%v", cluster)
							}

							return nil
						},
					},
					{
						Name:  "describe-cluster",
						Usage: "",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "cluster-name",
								Usage: "MSK cluster name, eg 'kafka-dev'",
							},
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							clusterName := c.String("cluster-name")

							if clusterName == "" {
								return fmt.Errorf("cluster-name is required")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)
							m := msk.New(kc)

							arn, err := m.GetClusterArn(clusterName)
							if err != nil {
								return err
							}

							describeClusterInput := kafka.DescribeClusterInput{
								ClusterArn: arn,
							}

							describeClusterOutput, err := kc.DescribeCluster(&describeClusterInput)
							if err != nil {
								return err
							}

							fmt.Printf("%v", describeClusterOutput)

							return nil
						},
					},
					{
						Name:  "get-bootstrap-brokers",
						Usage: "",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "cluster-name",
								Usage: "MSK cluster name, eg 'kafka-dev'",
							},
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							clusterName := c.String("cluster-name")

							if clusterName == "" {
								return fmt.Errorf("cluster-name is required")
							}

							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)
							m := msk.New(kc)

							bb, err := m.GetBootstrapBrokers(clusterName)
							if err != nil {
								return err
							}
							fmt.Print(*bb)
							return nil
						},
					},
					{
						Name:        "send",
						Description: "send message to topic",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "cluster-name",
								Usage: "MSK cluster name, eg 'kafka-dev'",
							},
							&cli.StringFlag{
								Name:  "bootstrap-broker",
								Usage: "bootstrap broker, eg. localhost:9092",
							},
							&cli.StringFlag{
								Name:  "topic",
								Usage: "topic name",
							},
							&cli.StringFlag{
								Name:  "message",
								Usage: "message body",
							},
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							clusterName := c.String("cluster-name")
							bootstrapBrokers := c.String("bootstrap-broker")

							if clusterName == "" && bootstrapBrokers == "" {
								return fmt.Errorf("cluster-name or bootstrap-broker is required")
							}
							msg := c.String("message")
							if msg == "" {
								return fmt.Errorf("message is required")
							}

							topic := c.String("topic")
							if topic == "" {
								return fmt.Errorf("topic is required")
							}
							profile := c.String("profile")

							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)
							m := msk.New(kc)

							bb, err := m.GetBootstrapBrokers(clusterName)
							if err != nil {
								return fmt.Errorf("getBootstrapBroker failed: %s", err)
							}

							fk := flowkafka.NewFlowKafka(&flowkafka.ServiceConfig{
								BootstrapBroker: *bb,
							})
							return fk.Produce(topic, []byte(msg))
						},
					},
					{
						Name:        "create-topic",
						Description: "crates the topic",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "cluster-name",
								Usage: "MSK cluster name, eg 'kafka-dev'",
							},
							&cli.StringFlag{
								Name:  "bootstrap-broker",
								Usage: "bootstrap broker, eg. localhost:9092",
							},
							&cli.StringFlag{
								Name:  "topic",
								Usage: "topic name",
							},
							&cli.IntFlag{
								Name:  "num-partitions",
								Value: 1,
								Usage: "number of partitions",
							},
							&cli.IntFlag{
								Name:  "replication-factor",
								Value: 1,
								Usage: "replication factor",
							},
							&cli.StringFlag{
								Name:  "retention-ms",
								Value: "-1",
								Usage: "retention ms",
							},
							&cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							clusterName := c.String("cluster-name")
							bootstrapBrokers := c.String("bootstrap-broker")

							if clusterName == "" && bootstrapBrokers == "" {
								return fmt.Errorf("cluster-name or bootstrap-broker is required")
							}

							topic := c.String("topic")
							if topic == "" {
								return fmt.Errorf("topic is required")
							}

							numPartitions := c.Int("num-partitions")
							replicationFactor := c.Int("replication-factor")
							retentionMs := c.String("retention-ms")

							profile := c.String("profile")

							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)
							m := msk.New(kc)

							bb, err := m.GetBootstrapBrokers(clusterName)
							if err != nil {
								return fmt.Errorf("getBootstrapBroker failed: %s", err)
							}
							ks := flowkafka.NewFlowKafka(&flowkafka.ServiceConfig{
								BootstrapBroker: *bb,
							})
							return ks.CreateTopic(topic, numPartitions, replicationFactor, retentionMs)
						},
					},
					{
						Name:        "delete-topic",
						Description: "deletes the topic",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "cluster-name",
								Usage: "MSK cluster name, eg 'kafka-dev'",
							},
							&cli.StringFlag{
								Name:  "bootstrap-broker",
								Usage: "bootstrap broker, eg. localhost:9092",
							},
							&cli.StringFlag{
								Name:  "topic",
								Usage: "topic name",
							},
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							clusterName := c.String("cluster-name")
							bootstrapBrokers := c.String("bootstrap-broker")

							if clusterName == "" && bootstrapBrokers == "" {
								return fmt.Errorf("cluster-name or bootstrap-broker is required")
							}

							topic := c.String("topic")
							if topic == "" {
								return fmt.Errorf("topic is required")
							}

							profile := c.String("profile")

							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)
							m := msk.New(kc)

							bb, err := m.GetBootstrapBrokers(clusterName)
							if err != nil {
								return fmt.Errorf("getBootstrapBroker failed: %s", err)
							}
							fk := flowkafka.NewFlowKafka(&flowkafka.ServiceConfig{
								BootstrapBroker: *bb,
							})
							return fk.DeleteTopic(topic)
						},
					},
					{
						Name:        "describe-topic",
						Description: "describe topic",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "cluster-name",
								Value: "",
								Usage: "MSK cluster name, eg 'kafka-dev'",
							},
							&cli.StringFlag{
								Name:  "bootstrap-broker",
								Usage: "bootstrap broker, eg. localhost:9092",
							},
							&cli.StringSliceFlag{
								Name:  "topic",
								Usage: "topic(s) name",
							},
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							clusterName := c.String("cluster-name")
							bootstrapBrokers := c.String("bootstrap-broker")

							if clusterName == "" && bootstrapBrokers == "" {
								return fmt.Errorf("cluster-name or bootstrap-broker is required")
							}

							topic := c.StringSlice("topic")
							if len(topic) == 0 {
								return fmt.Errorf("topic is required")
							}

							profile := c.String("profile")

							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)
							m := msk.New(kc)

							bb, err := m.GetBootstrapBrokers(clusterName)
							if err != nil {
								return fmt.Errorf("getBootstrapBroker failed: %s", err)
							}
							fk := flowkafka.NewFlowKafka(&flowkafka.ServiceConfig{
								BootstrapBroker: *bb,
							})

							var tr []*flowkafka.Topic
							for _, t := range topic {
								r, err := fk.DescribeTopic(t)
								if err != nil {
									return fmt.Errorf("unable to describe topic: %s", err)
								}
								tr = append(tr, r)
							}
							bytes, err := json.Marshal(tr)
							if err != nil {
								return fmt.Errorf("unable to serialize response")
							}
							fmt.Printf("%v", string(bytes))

							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "sts",
				Usage: "AWS STS",
				Subcommands: []*cli.Command{
					{
						Name:  "assume-role",
						Usage: "",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name: "profile",
							},
							&cli.StringFlag{
								Name:     "role-arn",
								Required: true,
								EnvVars:  []string{"AWS_ROLE_ARN"},
							},
							&cli.StringFlag{
								Name:    "region",
								EnvVars: []string{"AWS_DEFAULT_REGION"},
							},
							&cli.Int64Flag{
								Name:  "duration-seconds",
								Value: 3600,
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							roleArn := c.String("role-arn")
							region := c.String("region")
							durationSeconds := c.Int64("duration-seconds")
							sess := session.NewSessionWithSharedProfile(profile)
							client := sts.New(sess)

							err, cred := flowsts.AssumeRole(context.Background(), client, durationSeconds, "flow", roleArn)
							if err != nil {
								return err
							}

							err, s := creds.GenerateEnv(region, *cred.AccessKeyId, *cred.SecretAccessKey, *cred.SessionToken)
							if err != nil {
								return err
							}

							fmt.Println(s)

							return nil
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "pubsub",
				Usage: "GCP Pub/Sub for testing locally with emulator",
				Subcommands: []*cli.Command{
					{
						Name:  "create-topic",
						Usage: "Create topic",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "topic",
								Aliases:  []string{"t"},
								Required: true,
							},
							&cli.StringFlag{
								Name:     "project-id",
								EnvVars:  []string{"PUBSUB_PROJECT_ID"},
								Required: true,
							},
							&cli.StringFlag{
								Name:     "pubsub-emulator-host",
								Aliases:  []string{"eh"},
								EnvVars:  []string{"PUBSUB_EMULATOR_HOST"},
								Usage:    "Pub/Sub emulator host like: 192.168.64.3:30867",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							topic := c.String("topic")
							projectID := c.String("project-id")
							eh := c.String("pubsub-emulator-host")

							if err := os.Setenv("PUBSUB_EMULATOR_HOST", eh); err != nil {
								return err
							}

							timeout, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
							defer cancelFunc()

							client, err := flowpubsub.NewPubSubClient(timeout, projectID)
							if err != nil {
								return err
							}

							return flowpubsub.CreateTopic(timeout, client, topic)
						},
					},
					{
						Name:  "create-subscription",
						Usage: "Create Subscription for a given topic",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "topic",
								Aliases:  []string{"t"},
								Required: true,
							},
							&cli.StringFlag{
								Name:     "subscription",
								Aliases:  []string{"sub"},
								Required: true,
							},
							&cli.StringFlag{
								Name:     "project-id",
								EnvVars:  []string{"PUBSUB_PROJECT_ID"},
								Required: true,
							},
							&cli.StringFlag{
								Name:     "pubsub-emulator-host",
								Aliases:  []string{"eh"},
								EnvVars:  []string{"PUBSUB_EMULATOR_HOST"},
								Usage:    "Pub/Sub emulator host like: 192.168.64.3:30867",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							topic := c.String("topic")
							subscription := c.String("subscription")
							projectID := c.String("project-id")
							eh := c.String("pubsub-emulator-host")

							if err := os.Setenv("PUBSUB_EMULATOR_HOST", eh); err != nil {
								return err
							}

							timeout, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
							defer cancelFunc()

							client, err := flowpubsub.NewPubSubClient(timeout, projectID)
							if err != nil {
								return err
							}

							return flowpubsub.CreateSubscription(timeout, client, topic, subscription)
						},
					},
					{
						Name:  "publish",
						Usage: "Publish message to topic",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "topic",
								Aliases:  []string{"t"},
								Required: true,
							},
							&cli.StringFlag{
								Name:    "message",
								Aliases: []string{"msg"},
								Usage:   "The message. Will be initialized from stdin if empty",
							},
							&cli.StringFlag{
								Name:    "attributes",
								Aliases: []string{"attr"},
								Usage:   "The attributes like: {\"event-type\":\"user-updated\",\"key\":\"val\"}",
							},
							&cli.StringFlag{
								Name:    "project-id",
								EnvVars: []string{"PUBSUB_PROJECT_ID"},
							},
							&cli.StringFlag{
								Name:     "pubsub-emulator-host",
								Aliases:  []string{"eh"},
								EnvVars:  []string{"PUBSUB_EMULATOR_HOST"},
								Usage:    "Pub/Sub emulator host like: 192.168.64.3:30867",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							topic := c.String("topic")
							msg := c.String("message")
							attributes := c.String("attributes")
							projectID := c.String("project-id")
							eh := c.String("pubsub-emulator-host")

							if err := os.Setenv("PUBSUB_EMULATOR_HOST", eh); err != nil {
								return err
							}

							if msg == "" {
								msg = reader.ReadStdin()
							}

							var attr map[string]string
							if attributes != "" {
								if err := json.Unmarshal([]byte(attributes), &attr); err != nil {
									return err
								}
							}

							timeout, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
							defer cancelFunc()

							client, err := flowpubsub.NewPubSubClient(timeout, projectID)
							if err != nil {
								return err
							}
							return flowpubsub.Publish(timeout, client, topic, msg, attr)
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "cloudtrail",
				Usage: "AWS CloudTrail",
				Subcommands: []*cli.Command{
					{
						Name:  "find",
						Usage: "Find events",
						Flags: []cli.Flag{
							&cli.StringSliceFlag{
								Name:        "contains",
								DefaultText: "event have to contia",
							},
							&cli.TimestampFlag{
								Name:   "start-time",
								Layout: time.RFC3339,
								Value:  cli.NewTimestamp(time.Now().Add(-1 * time.Hour)),
							},
							&cli.TimestampFlag{
								Name:   "end-time",
								Layout: time.RFC3339,
								Value:  cli.NewTimestamp(time.Now()),
							},
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							startTime := c.Timestamp("start-time")
							endTime := c.Timestamp("end-time")
							ss := c.StringSlice("contains")
							sess := session.NewSessionWithSharedProfile(profile)
							ctc := cloudtrail.New(sess)

							r := cloudtrail.LookupEventsInput{
								EndTime:   endTime,
								StartTime: startTime,
							}
							var events []*cloudtrail.Event
							defer func() {
								str, err := json.Marshal(events)
								if err != nil {
									panic(err)
								}
								fmt.Print(string(str))
							}()
							err := ctc.LookupEventsPagesWithContext(context.Background(), &r, func(o *cloudtrail.LookupEventsOutput, b bool) bool {
								for _, event := range o.Events {
									if len(ss) == 0 {
										events = append(events, event)
										continue
									}
									for _, s := range ss {
										if strings.Contains(*event.CloudTrailEvent, s) {
											events = append(events, event)
										}
									}
								}
								return b == true
							})
							return err
						},
					},
				},
			}
		}(),
		func() *cli.Command {
			return &cli.Command{
				Name:  "eks",
				Usage: "AWS EKS",
				Subcommands: []*cli.Command{
					{
						Name:  "dashboard",
						Usage: "Opens K8 dashboard from local machine",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "cluster",
								Required: true,
							},
							&cli.StringFlag{
								Name: "profile",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							cluster := c.String("cluster")
							sess := session.NewSessionWithSharedProfile(profile)

							// new session with assumed role
							eksc := eks.New(sess)
							cres, err := eksc.DescribeClusterWithContext(context.Background(), &eks.DescribeClusterInput{Name: aws.String(cluster)})
							if err != nil {
								return errors.Wrapf(err, "describe cluster %s", cluster)
							}

							clientset, err := newClientset(cres.Cluster, sess)
							if err != nil {
								return errors.Wrapf(err, "new clientset for %s", cluster)
							}

							cs := clientset.CoreV1().Secrets("kube-system")

							ls, err := cs.List(context.Background(), metav1.ListOptions{})
							if err != nil {
								return errors.Wrapf(err, "list secrets")
							}

							var s v1.Secret
							for _, item := range ls.Items {
								if strings.Contains(item.Name, "eks-admin-token") {
									s = item
									break
								}
							}

							bytes, ok := s.Data["token"]
							if !ok {
								return fmt.Errorf("eks-admin-token not found")
							}

							fmt.Printf("token: %s\n\n", string(bytes))

							var cmd *exec.Cmd
							if profile != "" {
								cmd = exec.Command("aws", "eks", "update-kubeconfig", "--name", cluster, "--profile", profile)
							} else {
								cmd = exec.Command("aws", "eks", "update-kubeconfig", "--name", cluster)
							}
							fmt.Printf("running command: %s\n", cmd.String())
							if err := cmd.Run(); err != nil {
								return errors.Wrapf(err, "update-kubeconfig")
							}

							rpcmd := exec.Command("kubectl", "proxy")
							fmt.Printf("running command: %s\n", rpcmd.String())
							url := " http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/#!/login"
							fmt.Printf("open in browser(use token from above): %s\n", url)
							if err := rpcmd.Run(); err != nil {
								return errors.Wrapf(err, "kubectl proxy")
							}

							return err
						},
					},
				},
			}
		}(),
	}

	app.Action = func(c *cli.Context) error {
		fmt.Println("try: flow --help")
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func newClientset(cluster *eks.Cluster, sess *asession.Session) (*kubernetes.Clientset, error) {
	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, err
	}
	opts := &token.GetTokenOptions{
		ClusterID: aws.StringValue(cluster.Name),
		Session:   sess,
	}
	tok, err := gen.GetWithOptions(opts)
	if err != nil {
		return nil, err
	}

	ca, err := base64.StdEncoding.DecodeString(aws.StringValue(cluster.CertificateAuthority.Data))
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(
		&rest.Config{
			Host:        aws.StringValue(cluster.Endpoint),
			BearerToken: tok.Token,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: ca,
			},
		},
	)
}
