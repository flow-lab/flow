package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kafka"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/flow-lab/flow/pkg/base64"
	"github.com/flow-lab/flow/pkg/session"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/urfave/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// TODO [grokrz]: move to pkg
func getClusterArn(s string, kc *kafka.Kafka) (*string, error) {
	listClusterInput := kafka.ListClustersInput{}
	listClusterOutput, err := kc.ListClusters(&listClusterInput)
	if err != nil {
		return nil, fmt.Errorf("unable to list clusters: %v", err)
	}

	for _, cluster := range listClusterOutput.ClusterInfoList {
		if s == *cluster.ClusterName {
			return cluster.ClusterArn, nil
		}
	}

	return nil, fmt.Errorf("cluster arn for %s not found", s)
}

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
	app.Name = fmt.Sprintf("development tooling for AWS. Commit: %s, release date: %s", commit, date)
	app.Version = version

	app.Commands = []cli.Command{
		func() cli.Command {
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
								Name:  "filter-expression",
								Value: "",
							},
							cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "max-concurrent-pages-delete",
								Value: "50",
								Usage: "Max number of concurrent delete pages returned by scan operation",
							},
							cli.StringFlag{
								Name:  "expression-attribute-values",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")
							filterExpression := c.String("filter-expression")
							expressionAttributeValues := c.String("expression-attribute-values")
							maxConcurrent, err := strconv.Atoi(c.String("max-concurrent-pages-delete"))
							if err != nil {
								return err
							}
							sess := session.NewSessionWithSharedProfile(profile)

							ddbc := dynamodb.New(sess)

							input := &dynamodb.DescribeTableInput{
								TableName: aws.String(tableName),
							}

							result, err := ddbc.DescribeTable(input)
							if err != nil {
								return err
							}

							purge := func(items []map[string]*dynamodb.AttributeValue, pageNr int, wg *sync.WaitGroup) {
								defer wg.Done()

								if len(items) == 0 {
									return
								}

								var batches [][]map[string]*dynamodb.AttributeValue

								batchSize := 25
								for batchSize < len(items) {
									items, batches = items[batchSize:], append(batches, items[0:batchSize:batchSize])
								}
								batches = append(batches, items)

								for _, batch := range batches {
									time.Sleep(time.Duration(10) * time.Millisecond)

									var wrs []*dynamodb.WriteRequest

									for _, item := range batch {
										key := make(map[string]*dynamodb.AttributeValue)
										for _, ks := range result.Table.KeySchema {
											key[*ks.AttributeName] = item[*ks.AttributeName]
										}

										dr := dynamodb.DeleteRequest{
											Key: key,
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
											sleepTime := retry * retry * 100
											time.Sleep(time.Duration(sleepTime) * time.Millisecond)
											retry = retry + 1
										} else {
											break
										}
									}
								}
								fmt.Print(".")
							}

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

							pageNr := 0
							var wg sync.WaitGroup
							err = ddbc.ScanPages(&params, func(output *dynamodb.ScanOutput, lastPage bool) bool {
								targetMap := make([]map[string]*dynamodb.AttributeValue, len(output.Items))
								for key, value := range output.Items {
									targetMap[key] = value
								}
								wg.Add(1)
								cpy := make([]map[string]*dynamodb.AttributeValue, len(output.Items))
								copy(cpy, output.Items)
								go purge(cpy, pageNr, &wg)
								pageNr++

								if pageNr%maxConcurrent == 0 {
									wg.Wait()
								}
								return lastPage == false
							})
							if err != nil {
								return err
							}

							wg.Wait()

							fmt.Println("done")
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

							sess := session.NewSessionWithSharedProfile(profile)

							ddbc := dynamodb.New(sess)

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

							sess := session.NewSessionWithSharedProfile(profile)

							ddbc := dynamodb.New(sess)

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
					{
						Name:  "count-item",
						Usage: "counts elements in table using scan operation",
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
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							cli.StringFlag{
								Name:  "input",
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
							cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "keys",
								Value: "",
							},
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							tableName := c.String("table-name")
							keys := c.String("keys")
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
							cli.StringFlag{
								Name:  "table-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "secondary-index",
								Value: "",
							},
							cli.StringFlag{
								Name:  "projection-expression",
								Value: "",
							},
							cli.StringFlag{
								Name:  "keys",
								Value: "",
							},
							cli.StringFlag{
								Name:  "file-name",
								Value: "",
							},
							cli.StringFlag{
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
												if json, err := json.Marshal(item); err == nil {
													if _, err := writer.Write(json); err != nil {
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
								if jso, werr = json.Marshal(results); err == nil {
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
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							cli.StringFlag{
								Name:  "source-table-name",
								Value: "",
							},
							cli.StringFlag{
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
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							cli.StringFlag{
								Name:  "filter-expression",
								Value: "",
							},
							cli.StringFlag{
								Name:  "expression-attribute-values",
								Value: "",
							},
							cli.StringFlag{
								Name:  "projection-expression",
								Value: "",
							},
							cli.StringFlag{
								Name:  "file-name",
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
											if json, err := json.Marshal(elem); err == nil {
												if _, err := writer.Write(json); err != nil {
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
								if jso, werr = json.Marshal(l); err == nil {
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
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
							cli.StringSliceFlag{
								Name: "table-name",
							},
							cli.StringFlag{
								Name:  "time-range-upper-bound",
								Value: "",
								Usage: "only backups created before this time will be deleted. It is exclusive",
							},
							cli.StringFlag{
								Name:  "older-than",
								Value: "",
								Usage: "age in days",
							},
							cli.StringFlag{
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
		func() cli.Command {
			return cli.Command{
				Name: "sqs",
				Subcommands: []cli.Command{
					{
						Name:  "purge",
						Usage: "purge all messages",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "queue-name",
								Value: "",
							},
							cli.StringFlag{
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
							cli.StringFlag{
								Name:  "queue-name",
								Value: "",
							},
							cli.StringFlag{
								Name: "input-file-name",
							},
							cli.StringFlag{
								Name:  "message-attributes",
								Value: "",
							},
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							queueName := c.String("queue-name")
							inFileName := c.String("input-file-name")
							if inFileName == "" {
								return fmt.Errorf("input-file-name not found")
							}
							msgAttributes := c.String("message-attributes")
							sess := session.NewSessionWithSharedProfile(profile)

							jsonFile, err := os.Open(inFileName)
							if err != nil {
								return fmt.Errorf("error when opening %s", inFileName)
							}
							byteValue, _ := ioutil.ReadAll(jsonFile)
							defer jsonFile.Close()

							sqsc := sqs.New(sess)

							params := sqs.ListQueuesInput{}
							resp, err := sqsc.ListQueues(&params)
							if err != nil {
								return err
							}

							var messageAttributes map[string]*sqs.MessageAttributeValue

							if msgAttributes != "" {
								messageAttributes = map[string]*sqs.MessageAttributeValue{}
								json.Unmarshal([]byte(msgAttributes), &messageAttributes)
							}

							for _, elem := range resp.QueueUrls {
								if strings.Contains(*elem, queueName) {
									smi := sqs.SendMessageInput{
										QueueUrl: elem,
										MessageBody: func() *string {
											s := string(byteValue[:])
											return &s
										}(),
										MessageAttributes: messageAttributes,
									}
									_, err := sqsc.SendMessage(&smi)
									if err != nil {
										return err
									}
									fmt.Printf("Sent to %v", queueName)
								}
							}

							return nil
						},
					},
					{
						Name:  "describe",
						Usage: "get all attributes",
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
							cli.StringFlag{
								Name:  "queue-name",
								Value: "",
							},
							cli.StringFlag{
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
									params := sqs.ReceiveMessageInput{
										QueueUrl: elem,
									}
									resp, err := sqsc.ReceiveMessage(&params)
									if err != nil {
										return err
									}
									for msg := range resp.Messages {
										fmt.Printf("%v\n", msg)
									}
								}
							}

							return nil
						},
					},
				},
			}
		}(),
		func() cli.Command {
			return cli.Command{
				Name: "sns",
				Subcommands: []cli.Command{
					{
						Name:  "publish",
						Usage: "publish many messages",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "topic-name",
								Value: "",
							},
							cli.StringFlag{
								Name: "message",
							},
							cli.StringFlag{
								Name:  "times",
								Value: "1",
							},
							cli.StringFlag{
								Name:  "delay",
								Value: "0",
							},
							cli.StringFlag{
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
		func() cli.Command {
			return cli.Command{
				Name: "cloudwatch",
				Subcommands: []cli.Command{
					{
						Name:  "delete-alarm",
						Usage: "deletes cloudwatch alarm(s)",
						Flags: []cli.Flag{
							cli.StringSliceFlag{
								Name: "name",
							},
							cli.StringFlag{
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
		func() cli.Command {
			return cli.Command{
				Name: "cloudwatchlogs",
				Subcommands: []cli.Command{
					{
						Name:  "retention",
						Usage: "set log group retention in days",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "log-group-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "days",
								Usage: "retention in days",
								Value: "",
							},
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							logGroupName := c.String("log-group-name")
							retention, err := strconv.ParseInt(c.String("days"), 10, 0)
							if err != nil {
								return err
							}
							sess := session.NewSessionWithSharedProfile(profile)

							cwlc := cloudwatchlogs.New(sess)

							params := cloudwatchlogs.PutRetentionPolicyInput{
								LogGroupName:    &logGroupName,
								RetentionInDays: aws.Int64(retention),
							}

							_, err = cwlc.PutRetentionPolicy(&params)
							if err != nil {
								return err
							}
							fmt.Printf("ok")

							return nil
						},
					},
					{
						Name:  "write-to-file",
						Usage: "writes events to file in json format",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "log-group-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "file-name",
								Usage: "output file name",
								Value: "output",
							},
							cli.StringFlag{
								Name:  "filter-pattern",
								Usage: "the filter pattern to use. If not provided, all the events are matched",
								Value: "",
							},
							cli.StringFlag{
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
							cli.StringFlag{
								Name:  "log-group-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "filter-name",
								Value: "",
							},
							cli.StringFlag{
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
							cli.StringFlag{
								Name:  "log-group-name",
								Value: "",
							},
							cli.StringFlag{
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
							cli.StringFlag{
								Name:  "log-group-name-prefix",
								Value: "",
							},
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							logGroupNamePrefix := c.String("log-group-name-prefix")
							sess := session.NewSessionWithSharedProfile(profile)
							cwlc := cloudwatchlogs.New(sess)

							input := &cloudwatchlogs.DescribeLogGroupsInput{}

							if logGroupNamePrefix != "" {
								input.LogGroupNamePrefix = &logGroupNamePrefix
							}

							err := cwlc.DescribeLogGroupsPages(input, func(output *cloudwatchlogs.DescribeLogGroupsOutput, lastPage bool) bool {
								for _, lg := range output.LogGroups {
									bytes, err := json.Marshal(lg)
									if err != nil {
										panic(err)
									}

									fmt.Println(string(bytes))
								}
								return lastPage == false
							})

							return err
						},
					},
				},
			}
		}(),
		func() cli.Command {
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
							sess := session.NewSessionWithSharedProfile(profile)

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
		}(),
		func() cli.Command {
			return cli.Command{
				Name: "secretsmanager",
				Subcommands: []cli.Command{
					{
						Name:        "get-secret-value",
						Description: "Gets secret value",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name: "secret-id",
							},
							cli.StringFlag{
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
							cli.StringFlag{
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
		func() cli.Command {
			return cli.Command{
				Name: "kinesis",
				Subcommands: []cli.Command{
					{
						Name:  "update-shard-count",
						Usage: "update shard count",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "stream-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "count",
								Value: "1",
							},
							cli.StringFlag{
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
		func() cli.Command {
			return cli.Command{
				Name:        "base64",
				Description: "encoding/decoding base64",
				Subcommands: []cli.Command{
					{
						Name:  "encode",
						Usage: "encodes string to base64",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "input",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							input := c.String("input")
							if input == "" {
								return fmt.Errorf("missing --input")
							}

							encode := base64.Encode(input)
							fmt.Println(string(encode))

							return nil
						},
					},
					{
						Name:  "decode",
						Usage: "decodes base64 encoded string",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "input",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							input := c.String("input")
							if input == "" {
								return fmt.Errorf("missing --input")
							}

							bytes, err := base64.Decode(input)
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
		func() cli.Command {
			return cli.Command{
				Name: "s3",
				Subcommands: []cli.Command{
					{
						Name:  "purge",
						Usage: "delete all objects and it versions",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "bucket-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "filter",
								Value: "",
								Usage: "delete only objects matching filter",
							},
							cli.StringFlag{
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
		func() cli.Command {
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
		}(),
		func() cli.Command {
			return cli.Command{
				Name: "kafka",
				Subcommands: []cli.Command{
					{
						Name:  "list-clusters",
						Usage: "",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "profile",
								Value: "",
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
							cli.StringFlag{
								Name:  "cluster-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							clusterName := c.String("cluster-name")
							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)

							clusterArn, err := getClusterArn(clusterName, kc)
							if err != nil {
								return err
							}

							describeClusterInput := kafka.DescribeClusterInput{
								ClusterArn: clusterArn,
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
							cli.StringFlag{
								Name:  "cluster-name",
								Value: "",
							},
							cli.StringFlag{
								Name:  "profile",
								Value: "",
							},
						},
						Action: func(c *cli.Context) error {
							profile := c.String("profile")
							clusterName := c.String("cluster-name")
							sess := session.NewSessionWithSharedProfile(profile)
							kc := kafka.New(sess)

							clusterArn, err := getClusterArn(clusterName, kc)
							if err != nil {
								return err
							}

							getBootstrapBrokersInput := kafka.GetBootstrapBrokersInput{
								ClusterArn: clusterArn,
							}

							getBootstrapBrokersInputOutput, err := kc.GetBootstrapBrokers(&getBootstrapBrokersInput)
							if err != nil {
								return err
							}

							fmt.Printf("%v", getBootstrapBrokersInputOutput)

							return nil
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
