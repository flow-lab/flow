package kafka

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/flow-lab/flow/pkg"
)

type config struct {
	saramaConfig    *sarama.Config
	bootstrapBroker string
	producer        sarama.SyncProducer
}

// FlowKafka is an interface representing operations that can be executed with Kafka Cluster
type FlowKafka interface {
	CreateTopic(topic string, numPartitions int, replicationFactor int, retentionMs string) error
	DeleteTopic(topic string) error
	DescribeTopic(topic string) (*pkg.Topic, error)
	Produce(topic string, msg []byte) error
}

type ServiceConfig struct {
	BootstrapBroker string
	producer        sarama.SyncProducer
	clusterAdmin    sarama.ClusterAdmin
}

// NewFlowKafka create new instance of service
func NewFlowKafka(c *ServiceConfig) FlowKafka {
	if c.BootstrapBroker == "" {
		panic("bootstrapBrokers is required")
	}

	kv := sarama.V2_3_0_0

	sConfig := sarama.NewConfig()
	sConfig.Version = kv
	sConfig.Producer.RequiredAcks = sarama.WaitForAll
	sConfig.Producer.Return.Successes = true

	ser := &saramaService{
		config: config{
			saramaConfig:    sConfig,
			producer:        c.producer,
			bootstrapBroker: c.BootstrapBroker,
		},
	}

	if c.clusterAdmin == nil {
		config := sarama.NewConfig()
		config.Version = kv
		var addr []string
		addr = append(addr, c.BootstrapBroker)
		admin, err := sarama.NewClusterAdmin(addr, config)
		if err != nil {
			panic(err)
		}
		ser.ClusterAdmin = admin
	} else {
		ser.ClusterAdmin = c.clusterAdmin
	}

	return ser
}

type saramaService struct {
	config
	sarama.ClusterAdmin
}

func (ss *saramaService) Produce(topic string, msg []byte) error {
	if ss.producer == nil {
		producer, err := sarama.NewSyncProducer([]string{ss.bootstrapBroker}, ss.saramaConfig)
		if err != nil {
			panic(err)
		}

		ss.producer = producer
	}

	pmsg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(msg),
	}
	partitionNr, offset, err := ss.producer.SendMessage(pmsg)
	if err != nil {
		return err
	}

	fmt.Printf("message sent to topic(%s)/partition(%d)/offset(%d)\n", topic, partitionNr, offset)
	return nil
}

func (ss *saramaService) CreateTopic(topic string, numPartitions int, replicationFactor int, retentionMs string) error {
	td := &sarama.TopicDetail{
		ConfigEntries:     map[string]*string{"retention.ms": &retentionMs},
		NumPartitions:     int32(numPartitions),
		ReplicationFactor: int16(replicationFactor),
	}
	return ss.ClusterAdmin.CreateTopic(topic, td, false)
}

func (ss *saramaService) DeleteTopic(topic string) error {
	return ss.ClusterAdmin.DeleteTopic(topic)
}

func (ss *saramaService) DescribeTopic(topic string) (*pkg.Topic, error) {
	entries, err := ss.ClusterAdmin.DescribeConfig(sarama.ConfigResource{
		Type: sarama.TopicResource,
		Name: topic,
	})
	if err != nil {
		return nil, err
	}

	var c []*pkg.ConfigEntry
	for _, e := range entries {
		c = append(c, &pkg.ConfigEntry{
			Name:      e.Name,
			Value:     e.Value,
			ReadOnly:  e.ReadOnly,
			Default:   e.Default,
			Sensitive: e.Sensitive,
		})
	}

	t := pkg.Topic{
		Name:    topic,
		Configs: c,
	}
	return &t, nil
}
