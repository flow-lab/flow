package sqs

import (
	"context"
	"errors"
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

	for _, rh := range receiptHandles {
		dmi := sqs.DeleteMessageInput{
			QueueUrl:      aws.String(qUrl),
			ReceiptHandle: aws.String(rh),
		}
		_, err = f.DeleteMessage(&dmi)
		if err != nil {
			return err
		}
	}

	return nil
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
