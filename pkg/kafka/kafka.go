package kafka

import (
	"fmt"
	"github.com/Shopify/sarama"
	"time"
)

// ConfigEntry represents topic configuration
type ConfigEntry struct {
	Name  string
	Value string
}

// Topic represents kafka topic
type Topic struct {
	Name     string         `json:"name,omitempty"`
	Configs  []*ConfigEntry `json:"configs,omitempty"`
	ErrorMsg *string        `json:"errorMsg,omitempty"`
}

// Metadata represents topics details
type Metadata struct {
	Topics []*Topic
}

type config struct {
	saramaConfig    *sarama.Config
	bootstrapBroker string
	producer        sarama.SyncProducer
	broker          Broker
}

// FlowKafka is an interface representing operations that can be make with Kafka Cluster
type FlowKafka interface {
	CreateTopic(topic string, numPartitions int, replicationFactor int, retentionMs string) error
	DeleteTopic(topic string) error
	DescribeTopic(topic ...string) ([]*Topic, error)
	GetMetadata() (*Metadata, error)
	Produce(topic string, msg []byte) error
}

type ServiceConfig struct {
	BootstrapBroker string
	producer        sarama.SyncProducer
	broker          Broker
}

// Broker needs to be wrapped so it can be tested
type Broker interface {
	CreateTopics(request *sarama.CreateTopicsRequest) (*sarama.CreateTopicsResponse, error)
	DeleteTopics(request *sarama.DeleteTopicsRequest) (*sarama.DeleteTopicsResponse, error)
	DescribeConfigs(request *sarama.DescribeConfigsRequest) (*sarama.DescribeConfigsResponse, error)
	GetMetadata(request *sarama.MetadataRequest) (*sarama.MetadataResponse, error)
	Open(conf *sarama.Config) error
	Close() error
}

type saramaBrokerWrapper struct {
	broker sarama.Broker
}

func (sb *saramaBrokerWrapper) CreateTopics(request *sarama.CreateTopicsRequest) (*sarama.CreateTopicsResponse, error) {
	return sb.broker.CreateTopics(request)
}

func (sb *saramaBrokerWrapper) DeleteTopics(request *sarama.DeleteTopicsRequest) (*sarama.DeleteTopicsResponse, error) {
	return sb.broker.DeleteTopics(request)
}

func (sb *saramaBrokerWrapper) Open(conf *sarama.Config) error {
	return sb.broker.Open(conf)
}

func (sb *saramaBrokerWrapper) Close() error {
	return sb.broker.Close()
}

func (sb *saramaBrokerWrapper) DescribeConfigs(request *sarama.DescribeConfigsRequest) (*sarama.DescribeConfigsResponse, error) {
	return sb.broker.DescribeConfigs(request)
}

func (sb *saramaBrokerWrapper) GetMetadata(request *sarama.MetadataRequest) (*sarama.MetadataResponse, error) {
	return sb.broker.GetMetadata(request)
}

// NewFlowKafka create new instance of service
func NewFlowKafka(c *ServiceConfig) FlowKafka {
	if c.BootstrapBroker == "" {
		panic("bootstrapBrokers is required")
	}

	sConfig := sarama.NewConfig()
	sConfig.Version = sarama.V2_1_0_0
	sConfig.Producer.RequiredAcks = sarama.WaitForAll
	sConfig.Producer.Return.Successes = true

	ser := &saramaService{
		config{
			saramaConfig:    sConfig,
			producer:        c.producer,
			bootstrapBroker: c.BootstrapBroker,
		},
	}

	if c.broker == nil {
		c.broker = &saramaBrokerWrapper{
			broker: *sarama.NewBroker(c.BootstrapBroker),
		}
	}
	ser.broker = c.broker
	return ser
}

type saramaService struct {
	config
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
	err := ss.broker.Open(ss.config.saramaConfig)
	if err != nil {
		return fmt.Errorf("unable to connect to broker: %v", err)
	}
	defer func() {
		err := ss.broker.Close()
		if err != nil {
			panic(err)
		}
	}()

	topicDetail := &sarama.TopicDetail{
		ConfigEntries: map[string]*string{
			"retention.ms": &retentionMs,
		},
	}
	topicDetail.NumPartitions = int32(numPartitions)
	topicDetail.ReplicationFactor = int16(replicationFactor)
	topicDetails := make(map[string]*sarama.TopicDetail)
	topicDetails[topic] = topicDetail
	request := sarama.CreateTopicsRequest{
		Timeout:      time.Second * 15,
		TopicDetails: topicDetails,
	}

	response, err := ss.broker.CreateTopics(&request)
	if err != nil {
		fmt.Printf("unable to create topic: %v", err)
		return err
	}

	topicError := response.TopicErrors[topic]
	if topicError != nil && topicError.Err != sarama.ErrNoError {
		return fmt.Errorf("createTopic failed with %s", response.TopicErrors[topic])
	} else {
		fmt.Printf("topic %s created", topic)
	}

	return nil
}

func (ss *saramaService) DeleteTopic(topic string) error {
	err := ss.broker.Open(ss.config.saramaConfig)
	if err != nil {
		return fmt.Errorf("unable to connect to broker: %v", err)
	}
	defer func() {
		err := ss.broker.Close()
		if err != nil {
			panic(err)
		}
	}()

	dr := &sarama.DeleteTopicsRequest{
		Topics: []string{
			topic,
		},
		Timeout: time.Second * 15,
	}

	response, err := ss.broker.DeleteTopics(dr)
	if err != nil {
		fmt.Printf("unable to delete topic: %v", err)
		return err
	}

	kError := response.TopicErrorCodes[topic]
	if kError != sarama.ErrNoError {
		return fmt.Errorf("deleteTopic failed: %s", kError)
	} else {
		fmt.Printf("topic %s deleted", topic)
	}
	return nil
}

func (ss *saramaService) DescribeTopic(topics ...string) ([]*Topic, error) {
	err := ss.broker.Open(ss.config.saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to broker: %v", err)
	}
	defer func() {
		err := ss.broker.Close()
		if err != nil {
			panic(err)
		}
	}()

	var configs []*sarama.ConfigResource
	for _, t := range topics {
		configs = append(configs, &sarama.ConfigResource{
			Name: t,
			Type: sarama.TopicResource,
		})
	}

	request := &sarama.DescribeConfigsRequest{
		Version:   0,
		Resources: configs,
	}

	response, err := ss.broker.DescribeConfigs(request)
	if err != nil {
		return nil, fmt.Errorf("unable to get config. Err: %s", err)
	}

	var resTopics []*Topic
	for _, r := range response.Resources {
		if r.ErrorCode != 0 {
			resTopics = append(resTopics, &Topic{
				Name: r.Name,
				ErrorMsg: func() *string {
					p := sarama.KError(r.ErrorCode).Error()
					return &p
				}(),
			})

			continue
		}

		var configEntries []*ConfigEntry
		for _, ce := range r.Configs {
			configEntries = append(configEntries, &ConfigEntry{
				Name:  ce.Name,
				Value: ce.Value,
			})
		}

		resTopics = append(resTopics, &Topic{
			Name:    r.Name,
			Configs: configEntries,
		})
	}

	return resTopics, nil
}

func (ss *saramaService) GetMetadata() (*Metadata, error) {
	err := ss.broker.Open(ss.config.saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to broker: %v", err)
	}
	defer func() {
		err := ss.broker.Close()
		if err != nil {
			panic(err)
		}
	}()

	response, err := ss.broker.GetMetadata(&sarama.MetadataRequest{})
	if err != nil {
		return nil, fmt.Errorf("unable to get config. Err: %s", err)
	}

	var resTopics []*Topic
	for _, r := range response.Topics {
		resTopics = append(resTopics, &Topic{
			Name: r.Name,
		})
	}

	return &Metadata{
		Topics: resTopics,
	}, nil
}
