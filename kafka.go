package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/kafka"
	"github.com/urfave/cli"
)

var kafkaCommand = func() cli.Command {
	return cli.Command{
		Name: "kafka",
		Subcommands: []cli.Command{
			{
				Name:  "list-clusters",
				Usage: "",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					sess := NewSessionWithSharedProfile(profile)

					kc := kafka.New(sess)

					listClusterInput := kafka.ListClustersInput{}

					listClusterOutput, err := kc.ListClusters(&listClusterInput)
					if err != nil {
						return err
					}

					for _, cluster := range listClusterOutput.ClusterInfoList {
						fmt.Printf("%v", cluster)
					}

					return nil
				},
			},
		},
	}
}
