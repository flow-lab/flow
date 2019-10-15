package msk

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/kafka"
	"github.com/aws/aws-sdk-go/service/kafka/kafkaiface"
)

// FlowMSK is a AWS MSK SDK wrapper
type FlowMSK interface {
	GetBootstrapBrokers(clusterName string) (*string, error)
	GetClusterArn(clusterName string) (*string, error)
}

// newClient creates, initializes and returns a new service client instance.
type mskKafka struct {
	kafkaiface.KafkaAPI
}

// New creates a new instance of the MSK Kafka client
func New(kc kafkaiface.KafkaAPI) FlowMSK {
	return &mskKafka{
		kc,
	}
}

// GetBootstrapBrokers gets bootstrap brokers for cluster
func (c *mskKafka) GetBootstrapBrokers(clusterName string) (*string, error) {
	clusterArn, err := getClusterArn(clusterName, c.KafkaAPI)
	if err != nil {
		return nil, fmt.Errorf("unable to getClusterArn: %v", err)
	}
	getBootstrapBrokersInput := kafka.GetBootstrapBrokersInput{
		ClusterArn: clusterArn,
	}
	getBootstrapBrokersInputOutput, err := c.KafkaAPI.GetBootstrapBrokers(&getBootstrapBrokersInput)
	if err != nil {
		return nil, fmt.Errorf("unable to getBootstrapBrokers: %v", err)
	}

	return getBootstrapBrokersInputOutput.BootstrapBrokerString, nil
}

// GetClusterArn return cluster arn
func (c *mskKafka) GetClusterArn(clusterName string) (*string, error) {
	return getClusterArn(clusterName, c.KafkaAPI)
}

func getClusterArn(s string, kc kafkaiface.KafkaAPI) (*string, error) {
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
