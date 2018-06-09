package main

import (
	"github.com/urfave/cli"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"fmt"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

var cloudwatchCommand = func() cli.Command {
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
					sess := session.Must(session.NewSessionWithOptions(session.Options{
						AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
						SharedConfigState:       session.SharedConfigEnable,
						Profile:                 profile,
					}))

					cwlc := cloudwatch.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

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
}
