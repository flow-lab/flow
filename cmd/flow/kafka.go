package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/kafka"
	"github.com/flow-lab/flow/pkg/session"
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
					sess := session.NewSessionWithSharedProfile(profile)
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
			{
				Name:  "describe-cluster",
				Usage: "",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "cluster-name",
						Value: "",
					},
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					clusterName := c.String("cluster-name")
					sess := session.NewSessionWithSharedProfile(profile)
					kc := kafka.New(sess)

					clusterArn, err := getClusterArn(clusterName, kc)
					if err != nil {
						return err
					}

					describeClusterInput := kafka.DescribeClusterInput{
						ClusterArn: clusterArn,
					}

					describeClusterOutput, err := kc.DescribeCluster(&describeClusterInput)
					if err != nil {
						return err
					}

					fmt.Printf("%v", describeClusterOutput)

					return nil
				},
			},
			{
				Name:  "get-bootstrap-brokers",
				Usage: "",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "cluster-name",
						Value: "",
					},
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					clusterName := c.String("cluster-name")
					sess := session.NewSessionWithSharedProfile(profile)
					kc := kafka.New(sess)

					clusterArn, err := getClusterArn(clusterName, kc)
					if err != nil {
						return err
					}

					getBootstrapBrokersInput := kafka.GetBootstrapBrokersInput{
						ClusterArn: clusterArn,
					}

					getBootstrapBrokersInputOutput, err := kc.GetBootstrapBrokers(&getBootstrapBrokersInput)
					if err != nil {
						return err
					}

					fmt.Printf("%v", getBootstrapBrokersInputOutput)

					return nil
				},
			},
		},
	}
}

func getClusterArn(s string, kc *kafka.Kafka) (*string, error) {
	listClusterInput := kafka.ListClustersInput{}
	listClusterOutput, err := kc.ListClusters(&listClusterInput)
	if err != nil {
		return nil, fmt.Errorf("unable to list clusters: %v", err)
	}

	for _, cluster := range listClusterOutput.ClusterInfoList {
		if s == *cluster.ClusterName {
			return cluster.ClusterArn, nil
		}
	}

	return nil, fmt.Errorf("cluster arn for %s not found", s)
}
