package kafka

import (
	"github.com/Shopify/sarama"
	smocks "github.com/Shopify/sarama/mocks"
	"github.com/flow-lab/flow/pkg/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:generate mockgen -source=kafka.go -destination=../mocks/mock_kafka.go -package=mocks

func TestNewKafkaInit(t *testing.T) {
	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
	})
	assert.NotNil(t, s)
}

func TestSaramaService_Produce(t *testing.T) {
	syncProdMock := smocks.NewSyncProducer(t, nil)
	syncProdMock.ExpectSendMessageAndSucceed()
	s := NewFlowKafka(&ServiceConfig{
		producer:        syncProdMock,
		BootstrapBroker: "localhost:9092",
	})
	assert.NotNil(t, s)

	err := s.Produce("test-topic", []byte("test msg"))

	assert.Nil(t, err)
}

func TestSaramaService_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bm := mocks.NewMockBroker(ctrl)

	bm.EXPECT().
		Open(gomock.Any()).
		Return(nil).
		Times(1)

	bm.EXPECT().
		Close().
		Return(nil).
		Times(1)

	topic := "test-topic"
	resp := &sarama.CreateTopicsResponse{
		TopicErrors: map[string]*sarama.TopicError{
			topic: {
				Err: sarama.ErrNoError,
			},
		},
	}
	bm.EXPECT().
		CreateTopics(gomock.Any()).
		Return(resp, nil).
		Times(1)

	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		broker:          bm,
	})

	err := s.CreateTopic(topic, 1, 1, "-1")

	assert.Nil(t, err)
}

func TestSaramaService_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bm := mocks.NewMockBroker(ctrl)

	bm.EXPECT().
		Open(gomock.Any()).
		Return(nil).
		Times(1)

	bm.EXPECT().
		Close().
		Return(nil).
		Times(1)

	topic := "test-topic"
	resp := &sarama.DeleteTopicsResponse{
		TopicErrorCodes: map[string]sarama.KError{
			"topic": sarama.ErrNoError,
		},
	}
	bm.EXPECT().
		DeleteTopics(gomock.Any()).
		Return(resp, nil).
		Times(1)

	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		broker:          bm,
	})

	err := s.DeleteTopic(topic)

	assert.Nil(t, err)
}

func TestSaramaService_DescribeTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bm := mocks.NewMockBroker(ctrl)

	bm.EXPECT().
		Open(gomock.Any()).
		Return(nil).
		Times(1)

	bm.EXPECT().
		Close().
		Return(nil).
		Times(1)

	topic := "test-topic"
	response := &sarama.DescribeConfigsResponse{
		Resources: []*sarama.ResourceResponse{
			{
				ErrorCode: 0,
				ErrorMsg:  "",
				Type:      sarama.TopicResource,
				Name:      "test-topic",
				Configs: []*sarama.ConfigEntry{
					{
						Name:      "segment.ms",
						Value:     "1000",
						ReadOnly:  false,
						Default:   false,
						Sensitive: false,
					},
				},
			},
		},
	}
	bm.EXPECT().
		DescribeConfigs(gomock.Any()).
		Return(response, nil).
		Times(1)

	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		broker:          bm,
	})

	r, err := s.DescribeTopic(topic)

	assert.Nil(t, err)
	assert.NotNil(t, r)
	assert.Equal(t, len(r), 1)
	assert.Equal(t, r[0].Name, topic)
	assert.Equal(t, len(r[0].Configs), 1)
	assert.Equal(t, r[0].Configs[0].Name, "segment.ms")
	assert.Equal(t, r[0].Configs[0].Value, "1000")
}

func TestSaramaBrokerWrapper_GetMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bm := mocks.NewMockBroker(ctrl)

	bm.EXPECT().
		Open(gomock.Any()).
		Return(nil).
		Times(1)

	bm.EXPECT().
		Close().
		Return(nil).
		Times(1)

	topic := "test-topic"
	resp := &sarama.MetadataResponse{
		Topics: []*sarama.TopicMetadata{
			{
				Name: topic,
			},
		},
	}
	bm.EXPECT().
		GetMetadata(gomock.Any()).
		Return(resp, nil).
		Times(1)

	s := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		broker:          bm,
	})

	metadata, err := s.GetMetadata()

	assert.Nil(t, err)
	assert.Equal(t, topic, metadata.Topics[0].Name)
}
