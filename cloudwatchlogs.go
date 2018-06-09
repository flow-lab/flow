package main

import (
	"github.com/urfave/cli"
	"strconv"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"fmt"
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
					sess := session.Must(session.NewSessionWithOptions(session.Options{
						AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
						SharedConfigState:       session.SharedConfigEnable,
						Profile:                 profile,
					}))

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
		},
	}
}
