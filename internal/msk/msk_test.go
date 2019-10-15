package msk

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kafka"
	"github.com/aws/aws-sdk-go/service/kafka/kafkaiface"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate mockgen -source=msk.go -destination=../mocks/mock_msk.go -package=mocks

type kafkaMock struct {
	kafkaiface.KafkaAPI
}

func (kc *kafkaMock) ListClusters(*kafka.ListClustersInput) (*kafka.ListClustersOutput, error) {
	return &kafka.ListClustersOutput{
		ClusterInfoList: []*kafka.ClusterInfo{
			{
				ClusterArn:  aws.String("test-arn"),
				ClusterName: aws.String("test-cluster"),
			},
		},
		NextToken: nil,
	}, nil
}

func (kc *kafkaMock) GetBootstrapBrokers(*kafka.GetBootstrapBrokersInput) (*kafka.GetBootstrapBrokersOutput, error) {
	return &kafka.GetBootstrapBrokersOutput{
		BootstrapBrokerString: aws.String("localhost:9092,localhost:9093"),
	}, nil
}

func TestMskKafka_GetBootstrapBrokers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c := New(&kafkaMock{})

	bootstrapBrokers, err := c.GetBootstrapBrokers("test-cluster")

	assert.Nil(t, err)
	assert.Equal(t, "localhost:9092,localhost:9093", *bootstrapBrokers)
}

func TestMskKafka_GetClusterArn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	c := New(&kafkaMock{})

	arn, err := c.GetClusterArn("test-cluster")

	assert.Nil(t, err)
	assert.Equal(t, "test-arn", *arn)
}
