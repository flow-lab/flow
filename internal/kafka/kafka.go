package kafka

import (
	"context"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
)

// ConfigEntry represents topic configuration.
type ConfigEntry struct {
	Name      string
	Value     string
	ReadOnly  bool
	Default   bool
	Sensitive bool
}

// Topic represents kafka topic.
type Topic struct {
	Name     string         `json:"name,omitempty"`
	Configs  []*ConfigEntry `json:"configs,omitempty"`
	ErrorMsg *string        `json:"errorMsg,omitempty"`
}

// Metadata represents topics details.
type Metadata struct {
	Topics []*Topic
}

// Broker represents kafka broker.
type Broker struct {
	ID   string `json:"id,omitempty"`
	Addr string `json:"addr,omitempty"`
}

type config struct {
	saramaConfig    *sarama.Config
	bootstrapBroker string
	producer        sarama.AsyncProducer
}

// FlowKafka is an interface representing operations that can be executed with Kafka Cluster.
type FlowKafka interface {
	CreateTopic(topic string, numPartitions int, replicationFactor int, retentionMs string) error
	DeleteTopic(topic string) error
	DescribeTopic(topic string) (*Topic, error)
	Produce(ctx context.Context, topic string, msg Message) error
	Read(ctx context.Context, topic string, bufferSize int) <-chan Message
	BrokerInfo(ctx context.Context) ([]Broker, error)
}

type ServiceConfig struct {
	BootstrapBroker string
	producer        sarama.AsyncProducer
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

func (ss *saramaService) Produce(ctx context.Context, topic string, msg Message) error {
	if ss.producer == nil {
		producer, err := sarama.NewAsyncProducer([]string{ss.bootstrapBroker}, ss.saramaConfig)
		if err != nil {
			panic(err)
		}

		ss.producer = producer
	}

	pmsg := sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
	}
	select {
	case ss.producer.Input() <- &pmsg:
	case msg := <-ss.producer.Successes():
		fmt.Printf("ack message/(%s)/(%s)/%d/%d\n", msg.Key, msg.Value, msg.Partition, msg.Offset)
	case e := <-ss.producer.Errors():
		fmt.Printf("error when sending(%s)\n", e.Error())
	case <-ctx.Done():
		err := ss.producer.Close()
		if err != nil {
			return err
		}
	}

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

func (ss *saramaService) DescribeTopic(topic string) (*Topic, error) {
	entries, err := ss.ClusterAdmin.DescribeConfig(sarama.ConfigResource{
		Type: sarama.TopicResource,
		Name: topic,
	})
	if err != nil {
		return nil, err
	}

	var c []*ConfigEntry
	for _, e := range entries {
		c = append(c, &ConfigEntry{
			Name:      e.Name,
			Value:     e.Value,
			ReadOnly:  e.ReadOnly,
			Default:   e.Default,
			Sensitive: e.Sensitive,
		})
	}

	t := Topic{
		Name:    topic,
		Configs: c,
	}
	return &t, nil
}

type Message struct {
	Key, Value []byte
}

func (ss *saramaService) Read(ctx context.Context, topic string, bufferSize int) <-chan Message {
	result := make(chan Message, bufferSize)
	go func(result chan<- Message) {
		// in case ctx is done cancel
		select {
		case <-ctx.Done():
			return
		default:
		}

		consumer, err := sarama.NewConsumer([]string{ss.bootstrapBroker}, ss.saramaConfig)
		if err != nil {
			panic(err)
		}

		partitions, err := consumer.Partitions(topic)
		if err != nil {
			panic(err)
		}

		consumers := make([]sarama.PartitionConsumer, len(partitions))
		for i, p := range partitions {
			c, err := consumer.ConsumePartition(topic, p, 0)
			if err != nil {
				panic(err)
			}

			// in case context is already done
			select {
			case <-ctx.Done():
				return
			default:
			}

			consumers[i] = c
		}

		fmt.Printf("got %d\n", len(partitions))

		for _, c := range consumers {
			go func(pc sarama.PartitionConsumer) {
				for {
					select {
					case msg := <-pc.Messages():
						fmt.Printf("received message/(%s)/(%s)/%d/%d\n", string(msg.Key), string(msg.Value), msg.Partition, msg.Offset)
						result <- Message{
							Key:   msg.Key,
							Value: msg.Value,
						}
					case <-ctx.Done():
						return
					}
				}
			}(c)
		}
	}(result)

	return result
}

func (ss *saramaService) BrokerInfo(_ context.Context) ([]Broker, error) {
	client, err := sarama.NewClient([]string{ss.bootstrapBroker}, ss.saramaConfig)
	if err != nil {
		return nil, errors.Wrap(err, "new client")
	}

	brokers := make([]Broker, 0)
	for _, b := range client.Brokers() {
		brokers = append(brokers, Broker{
			Addr: b.Addr(),
			ID:   fmt.Sprintf("%d", b.ID()),
		})
	}

	return brokers, nil
}
