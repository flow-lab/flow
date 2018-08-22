package main

import (
	"github.com/urfave/cli"
	"strings"
	"fmt"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
)

var sqsCommand = func() cli.Command {
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
					sess := NewSessionWithSharedProfile(profile)

					sqsc := sqs.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

					params := sqs.ListQueuesInput{
					}
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

					sess := NewSessionWithSharedProfile(profile)

					sqsc := sqs.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

					params := sqs.ListQueuesInput{
					}
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
					sess := NewSessionWithSharedProfile(profile)

					sqsc := sqs.New(sess, &aws.Config{
						Region: aws.String(endpoints.EuWest1RegionID),
					})

					params := sqs.ListQueuesInput{
					}
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
}
