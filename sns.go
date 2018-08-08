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
	"time"
	"sync"
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
					sess := session.Must(session.NewSessionWithOptions(session.Options{
						AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
						SharedConfigState:       session.SharedConfigEnable,
						Profile:                 profile,
					}))

					sqsc := sns.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

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
}
