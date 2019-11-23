package sqs

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"strings"
)

// FlowSQSClient is a client for interacting with SQS API and wraps the standard client.
type FlowSQSClient interface {
	Delete(ctx context.Context, queueName string, receiptHandles []string) error
}

// NewSQSClient creates a new flow sqs client.
func NewSQSClient(d sqsiface.SQSAPI) (FlowSQSClient, error) {
	client := flowSQSClient{d}
	return &client, nil
}

type flowSQSClient struct {
	sqsiface.SQSAPI
}

func (f flowSQSClient) Delete(ctx context.Context, queueName string, receiptHandles []string) error {
	err, qUrl := f.resolveSQSURL(ctx, queueName)
	if err != nil {
		return err
	}

	var entries []*sqs.DeleteMessageBatchRequestEntry
	for i, rh := range receiptHandles {
		entries = append(entries, &sqs.DeleteMessageBatchRequestEntry{
			Id:            aws.String(fmt.Sprintf("%d", i)),
			ReceiptHandle: aws.String(rh),
		})
	}

	dmi := sqs.DeleteMessageBatchInput{
		QueueUrl: aws.String(qUrl),
		Entries:  entries,
	}

	_, err = f.DeleteMessageBatch(&dmi)
	return err
}

func (f flowSQSClient) resolveSQSURL(ctx context.Context, queueName string) (error, string) {
	resp, err := f.ListQueues(&sqs.ListQueuesInput{})
	if err != nil {
		return err, ""
	}

	for _, elem := range resp.QueueUrls {
		qUrl := aws.StringValue(elem)
		split := strings.Split(qUrl, "/")
		qName := split[len(split)-1]
		if strings.EqualFold(qName, queueName) {
			return nil, qUrl
		}
	}

	return errors.New("sqs url not found"), ""
}
