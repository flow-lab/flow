package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/flow-lab/flow/pkg/session"
	"github.com/urfave/cli"
	"strconv"
)

var KinesisCommand = func() cli.Command {
	return cli.Command{
		Name: "kinesis",
		Subcommands: []cli.Command{
			{
				Name:  "update-shard-count",
				Usage: "update shard count",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "stream-name",
						Value: "",
					},
					cli.StringFlag{
						Name:  "count",
						Value: "1",
					},
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					streamName := c.String("stream-name")
					count, err := strconv.ParseInt(c.String("count"), 10, 0)
					if err != nil {
						return err
					}
					profile := c.String("profile")

					sess := session.NewSessionWithSharedProfile(profile)

					kinesisc := kinesis.New(sess)
					updateShardCountInput := kinesis.UpdateShardCountInput{
						StreamName:       &streamName,
						TargetShardCount: aws.Int64(count),
						ScalingType:      aws.String(kinesis.ScalingTypeUniformScaling),
					}
					_, err = kinesisc.UpdateShardCount(&updateShardCountInput)
					if err != nil {
						fmt.Println("Unable to update shard count", err.Error())
					}

					fmt.Print("done")

					return nil
				},
			},
		},
	}
}
