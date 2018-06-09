package main

import (
	"github.com/urfave/cli"
	"strconv"
	"strings"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/sns"
)

var snsCommand = func() cli.Command {
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
					sess := session.Must(session.NewSessionWithOptions(session.Options{
						AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
						SharedConfigState:       session.SharedConfigEnable,
						Profile:                 profile,
					}))

					sqsc := sns.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

					listTopicsParams := sns.ListTopicsInput{}
					out, err := sqsc.ListTopics(&listTopicsParams)
					for _, topic := range out.Topics {
						if strings.Contains(*topic.TopicArn, topicName) {
							for i := 0; i < times; i++ {
								params := sns.PublishInput{
									Message:  &message,
									TopicArn: topic.TopicArn,
								}
								_, err := sqsc.Publish(&params)
								if err != nil {
									return err
								}
								fmt.Print(".")
							}
						}
					}

					return nil
				},
			},
		},
	}
}
