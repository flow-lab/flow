package kafka

import (
	"testing"

	"github.com/Shopify/sarama"
	"github.com/flow-lab/flow/internal/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

//go:generate mockgen -package=mocks -destination ../mocks/mock_cluster_admin.go github.com/Shopify/sarama ClusterAdmin

func TestNewFlowKafka_EmptyBroker(t *testing.T) {
	_, err := NewFlowKafka(&ServiceConfig{BootstrapBroker: ""})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrapBrokers is required")
}

func TestNewFlowKafka_WithClusterAdmin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks.NewMockClusterAdmin(ctrl)

	fk, err := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})

	assert.NoError(t, err)
	assert.NotNil(t, fk)
}

func TestSaramaService_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks.NewMockClusterAdmin(ctrl)
	ca.EXPECT().
		CreateTopic(gomock.Eq("test-topic"), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)

	fk, err := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})
	assert.NoError(t, err)

	err = fk.CreateTopic("test-topic", 1, 1, "-1")

	assert.NoError(t, err)
}

func TestSaramaService_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks.NewMockClusterAdmin(ctrl)
	ca.EXPECT().
		DeleteTopic(gomock.Eq("test-topic")).
		Return(nil).
		Times(1)

	fk, err := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})
	assert.NoError(t, err)

	err = fk.DeleteTopic("test-topic")

	assert.NoError(t, err)
}

func TestSaramaService_DescribeTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ca := mocks.NewMockClusterAdmin(ctrl)
	entries := []sarama.ConfigEntry{{Name: "retention.ms", Value: "-1"}}
	ca.EXPECT().
		DescribeConfig(gomock.Any()).
		Return(entries, nil).
		Times(1)

	fk, err := NewFlowKafka(&ServiceConfig{
		BootstrapBroker: "localhost:9092",
		clusterAdmin:    ca,
	})
	assert.NoError(t, err)

	topic, err := fk.DescribeTopic("test-topic")

	assert.NoError(t, err)
	assert.Equal(t, "test-topic", topic.Name)
	assert.Equal(t, 1, len(topic.Configs))
	assert.Equal(t, "retention.ms", topic.Configs[0].Name)
	assert.Equal(t, "-1", topic.Configs[0].Value)
}
