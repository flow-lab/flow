package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/flow-lab/flow/pkg/session"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
	"strings"
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
}
