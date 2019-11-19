package sqs

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/stretchr/testify/assert"
	"testing"
)

type SQSMock struct {
	sqsiface.SQSAPI
}

func (f SQSMock) ListQueues(*sqs.ListQueuesInput) (*sqs.ListQueuesOutput, error) {
	return &sqs.ListQueuesOutput{QueueUrls: []*string{aws.String("	https://sqs.eu-west-1.amazonaws.com/111111111111/test-queue-name")}}, nil
}

func (f SQSMock) DeleteMessage(*sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
	return nil, nil
}

func TestNewFlowDynamoDBClient(t *testing.T) {
	t.Run("Should create client", func(t *testing.T) {
		client, err := NewSQSClient(SQSMock{})

		assert.NotNil(t, client)
		assert.Nil(t, err)
	})
}

func Test_flowSQSClient_Delete(t *testing.T) {
	t.Run("Should delete message", func(t *testing.T) {
		client, err := NewSQSClient(SQSMock{})
		assert.Nil(t, err)

		err = client.Delete(context.Background(), "test-queue-name", []string{"test"})

		assert.Nil(t, err)
	})
}
