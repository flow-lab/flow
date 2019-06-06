package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/flow-lab/flow/pkg/session"
	"github.com/urfave/cli"
	"io/ioutil"
	"strconv"
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
		},
	}
}
