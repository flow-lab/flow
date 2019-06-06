package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/flow-lab/flow/pkg/session"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/urfave/cli"
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
}
