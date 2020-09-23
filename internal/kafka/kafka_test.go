package kafka

import (
	"context"
	"github.com/Shopify/sarama"
	smocks "github.com/Shopify/sarama/mocks"
	mocks2 "github.com/flow-lab/flow/internal/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate mockgen -package=mocks -destination ../mocks/mock_cluster_admin.go github.com/Shopify/sarama ClusterAdmin

func TestNewMimiroKafka(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks2.NewMockClusterAdmin(ctrl)

	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})
	assert.NotNil(t, s)
}

func TestSaramaService_Produce(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks2.NewMockClusterAdmin(ctrl)
	producer := smocks.NewAsyncProducer(t, nil)
	producer.ExpectInputAndSucceed()
	s := NewFlowKafka(&ServiceConfig{
		producer:        producer,
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})
	assert.NotNil(t, s)

	err := s.Produce(context.Background(), "test-topic", Message{Value: []byte("test msg")})

	assert.Nil(t, err)
}

func TestSaramaService_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks2.NewMockClusterAdmin(ctrl)
	topic := "test-topic"
	_ = &sarama.CreateTopicsResponse{
		TopicErrors: map[string]*sarama.TopicError{
			topic: {
				Err: sarama.ErrNoError,
			},
		},
	}
	ca.EXPECT().
		CreateTopic(gomock.Eq(topic), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)
	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})

	err := s.CreateTopic(topic, 1, 1, "-1")

	assert.Nil(t, err)
}

func TestSaramaService_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks2.NewMockClusterAdmin(ctrl)
	topic := "test-topic"
	ca.EXPECT().
		DeleteTopic(gomock.Eq(topic)).
		Return(nil).
		Times(1)
	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})

	err := s.DeleteTopic(topic)

	assert.Nil(t, err)
}

func TestSaramaService_DescribeTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks2.NewMockClusterAdmin(ctrl)
	topic := "test-topic"
	entries := []sarama.ConfigEntry{{Name: "test"}}
	ca.EXPECT().
		DescribeConfig(gomock.Any()).
		Return(entries, nil).
		Times(1)
	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})

	tc, err := s.DescribeTopic(topic)

	assert.Nil(t, err)
	assert.NotNil(t, tc)
}
