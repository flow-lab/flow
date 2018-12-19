package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/urfave/cli"
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
					sess := NewSessionWithSharedProfile(profile)

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
}
