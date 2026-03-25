package kafka

import (
	"context"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/pkg/errors"
)

type ConfigEntry struct {
	Name      string
	Value     string
	ReadOnly  bool
	Default   bool
	Sensitive bool
}

type Topic struct {
	Name     string         `json:"name,omitempty"`
	Configs  []*ConfigEntry `json:"configs,omitempty"`
	ErrorMsg *string        `json:"errorMsg,omitempty"`
}

type Metadata struct {
	Topics []*Topic
}

type Broker struct {
	ID   string `json:"id,omitempty"`
	Addr string `json:"addr,omitempty"`
}

type config struct {
	saramaConfig    *sarama.Config
	bootstrapBroker string
	producer        sarama.AsyncProducer
}

type FlowKafka interface {
	CreateTopic(topic string, numPartitions int, replicationFactor int, retentionMs string) error
	DeleteTopic(topic string) error
	DescribeTopic(topic string) (*Topic, error)
	Pipe(ctx context.Context, c <-chan Message, topic string) error
	Produce(ctx context.Context, topic string, msg Message) error
	Read(ctx context.Context, topic string, bufferSize int) (<-chan Message, error)
	BrokerInfo(ctx context.Context) ([]Broker, error)
}

type ServiceConfig struct {
	BootstrapBroker string
	producer        sarama.AsyncProducer
	clusterAdmin    sarama.ClusterAdmin
}

func NewFlowKafka(c *ServiceConfig) (FlowKafka, error) {
	if c.BootstrapBroker == "" {
		return nil, fmt.Errorf("bootstrapBrokers is required")
	}

	kv := sarama.V2_4_0_0

	sConfig := sarama.NewConfig()
	sConfig.Version = kv
	sConfig.Producer.RequiredAcks = sarama.WaitForAll
	sConfig.Producer.Return.Successes = true
	sConfig.Producer.Return.Errors = true

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
		admin, err := sarama.NewClusterAdmin([]string{c.BootstrapBroker}, config)
		if err != nil {
			return nil, fmt.Errorf("new cluster admin: %w", err)
		}
		ser.ClusterAdmin = admin
	} else {
		ser.ClusterAdmin = c.clusterAdmin
	}

	return ser, nil
}

type saramaService struct {
	config
	sarama.ClusterAdmin
}

func (ss *saramaService) ensureProducer() error {
	if ss.producer != nil {
		return nil
	}
	producer, err := sarama.NewAsyncProducer([]string{ss.bootstrapBroker}, ss.saramaConfig)
	if err != nil {
		return fmt.Errorf("new async producer: %w", err)
	}
	ss.producer = producer
	return nil
}

func (ss *saramaService) Produce(ctx context.Context, topic string, msg Message) error {
	if err := ss.ensureProducer(); err != nil {
		return err
	}

	pmsg := sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
	}

	for {
		select {
		case ss.producer.Input() <- &pmsg:
		case msg := <-ss.producer.Successes():
			fmt.Printf("ack message/(%s)/(%s)/%d/%d\n", msg.Key, msg.Value, msg.Partition, msg.Offset)
			return nil
		case e := <-ss.producer.Errors():
			fmt.Printf("error when sending(%s)\n", e.Error())
			return errors.Wrapf(e, "producer err")
		case <-ctx.Done():
			return ss.producer.Close()
		}
	}
}

func (ss *saramaService) Pipe(ctx context.Context, c <-chan Message, topic string) error {
	if err := ss.ensureProducer(); err != nil {
		return err
	}

	go func() {
		defer ss.producer.AsyncClose()
		for {
			select {
			case msg := <-c:
				pmsg := sarama.ProducerMessage{
					Topic: topic,
					Key:   sarama.ByteEncoder(msg.Key),
					Value: sarama.ByteEncoder(msg.Value),
				}
				ss.producer.Input() <- &pmsg
			case msg := <-ss.producer.Successes():
				fmt.Printf("ack message/(%s)/%d/%d\n", msg.Key, msg.Partition, msg.Offset)
			case err := <-ss.producer.Errors():
				fmt.Printf("error when sending(%s)\n", err.Error())
			case <-ctx.Done():
				return
			}
		}
	}()

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
	entries, err := ss.DescribeConfig(sarama.ConfigResource{
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

func (ss *saramaService) Read(ctx context.Context, topic string, bufferSize int) (<-chan Message, error) {
	result := make(chan Message, bufferSize)

	select {
	case <-ctx.Done():
		close(result)
		return result, nil
	default:
	}

	consumer, err := sarama.NewConsumer([]string{ss.bootstrapBroker}, ss.saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("new consumer: %w", err)
	}

	partitions, err := consumer.Partitions(topic)
	if err != nil {
		_ = consumer.Close()
		return nil, fmt.Errorf("get partitions: %w", err)
	}

	for _, p := range partitions {
		pc, err := consumer.ConsumePartition(topic, p, 0)
		if err != nil {
			_ = consumer.Close()
			return nil, fmt.Errorf("consume partition %d: %w", p, err)
		}

		go func(pc sarama.PartitionConsumer) {
			for {
				select {
				case msg := <-pc.Messages():
					result <- Message{
						Key:   msg.Key,
						Value: msg.Value,
					}
				case <-ctx.Done():
					return
				}
			}
		}(pc)
	}

	return result, nil
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
