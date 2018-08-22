package main

import (
	"github.com/urfave/cli"
	"strconv"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"fmt"
	"io/ioutil"
	"encoding/json"
)

var cloudwatchlogsCommand = func() cli.Command {
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
					sess := NewSessionWithSharedProfile(profile)

					cwlc := cloudwatchlogs.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

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

					sess := NewSessionWithSharedProfile(profile)

					cwlc := cloudwatchlogs.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

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
		},
	}
}
