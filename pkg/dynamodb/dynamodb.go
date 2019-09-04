package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

// FlowDynamoDBClient is a client for interacting with DynamoDB API and wraps the standard client.
type FlowDynamoDBClient interface {
	Delete(ctx context.Context, tableName string, filterExpression *string, expressionAttributeValues *string) error
}

type flowDynamoDBClient struct {
	dynamodbiface.DynamoDBAPI
}

// NewFlowDynamoDBClient creates a new flow dynamoDB client.
func NewFlowDynamoDBClient(d dynamodbiface.DynamoDBAPI) (FlowDynamoDBClient, error) {
	client := flowDynamoDBClient{d}
	return &client, nil
}

// Delete deletes items from table. Will use filterExpression and expressionAttributeValues if given to only delete
// items defined or will delete all otherwise(purged).
//
// Implementation follows pipeline where:
// scan (generator) -> batch (stage) -> batchDelete (stage) -> client
// Each stage is no blocking and is using channels for communication.
//
// In case of error that program cannot recover the error will be propagated to the client
// and processing will be stopped.
func (f *flowDynamoDBClient) Delete(ctx context.Context, tableName string, filterExpression *string, expressionAttributeValues *string) error {
	describeTableInput := dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}
	describeTableOutput, err := f.DynamoDBAPI.DescribeTableWithContext(ctx, &describeTableInput)
	if err != nil {
		return err
	}

	attr := projectionExpression(describeTableOutput.Table.KeySchema)
	sr := scan(ctx, f.DynamoDBAPI, tableName, filterExpression, expressionAttributeValues, attr, 100)
	br := batch(ctx, 25, sr)
	dr := batchDelete(ctx, f.DynamoDBAPI, tableName, br)

	for r := range dr {
		if r.err != nil {
			return r.err
		}
	}

	return nil
}

func projectionExpression(keySchemaElements []*dynamodb.KeySchemaElement) *string {
	var attr string
	for i, e := range keySchemaElements {
		attr = attr + *e.AttributeName
		if i < len(keySchemaElements)-1 {
			attr = attr + ","
		}
	}
	return &attr
}

// TODO [grokrz]: refactor
func projectionExpressionNew(keySchemaElements []*dynamodb.KeySchemaElement) (projectionExpression *string, expressionAttributeNames *string) {
	var attr string
	m := make(map[string]string)
	for i, e := range keySchemaElements {
		a := fmt.Sprintf("#%s-%d", *e.AttributeName, i)
		attr = attr + a
		if i < len(keySchemaElements)-1 {
			attr = attr + ","
		}
		m[a] = *e.AttributeName
	}
	bytes, _ := json.Marshal(m)
	s := string(bytes)
	return &attr, &s
}

// scanResult represents the result of scan operation.
type scanResult struct {
	err   error
	value map[string]*dynamodb.AttributeValue
}

// Scan scans DynamoDB table and sends result to result channel.
//
// Result channel is initialized with the specified
// buffer capacity if bufferSize > 0. If zero, the channel is unbuffered.
func scan(ctx context.Context, c dynamodbiface.DynamoDBAPI, tableName string, filterExpression *string, expressionAttributeValues *string, projectionExpression *string, bufferSize int) <-chan scanResult {
	scanResults := make(chan scanResult, bufferSize)
	go func(scanResults chan<- scanResult) {
		defer close(scanResults)
		scanInput := dynamodb.ScanInput{
			TableName: &tableName,
		}
		if filterExpression != nil {
			scanInput.FilterExpression = filterExpression
		}
		if expressionAttributeValues != nil {
			var m map[string]*dynamodb.AttributeValue
			err := json.Unmarshal([]byte(*expressionAttributeValues), &m)
			if err != nil {
				scanResults <- scanResult{
					err: fmt.Errorf("unable to unmarshal expressionAttributes: %v", expressionAttributeValues),
				}
				return
			}
			scanInput.ExpressionAttributeValues = m
		}
		if projectionExpression != nil {
			scanInput.ProjectionExpression = projectionExpression
		}
		err := c.ScanPages(&scanInput, func(output *dynamodb.ScanOutput, lastPage bool) bool {
			for _, item := range output.Items {
				scanResults <- scanResult{
					value: item,
				}

				// in case ctx is done lets cancel processing
				select {
				case <-ctx.Done():
					return true
				default:
				}
			}

			return lastPage == false
		})
		if err != nil {
			scanResults <- scanResult{
				err: fmt.Errorf("error during scanPages: %v", err),
			}
		}
	}(scanResults)

	return scanResults
}

type batchResult struct {
	err   error
	value []map[string]*dynamodb.AttributeValue
}

// batch up to batchSize
func batch(ctx context.Context, batchSize int, scanResults <-chan scanResult) <-chan batchResult {
	batchResults := make(chan batchResult)
	go func(scanResults <-chan scanResult) {
		defer close(batchResults)

		b := make([]map[string]*dynamodb.AttributeValue, 0)
		for {
			select {
			case r, ok := <-scanResults:
				if r.err != nil {
					batchResults <- batchResult{
						err: r.err,
					}
					return
				}
				if ok == false {
					// channel has been closed, emit and close the batchResults channel
					if len(b) > 0 {
						batchResults <- batchResult{
							value: clone(b),
						}
						b = b[:0]
					}
					return
				}

				b = append(b, r.value)
				if len(b) == batchSize {
					batchResults <- batchResult{
						value: clone(b),
					}
					b = b[:0]
				}
			case <-ctx.Done():
				return
			default:
			}
		}
	}(scanResults)

	return batchResults
}

func clone(src []map[string]*dynamodb.AttributeValue) []map[string]*dynamodb.AttributeValue {
	var dest []map[string]*dynamodb.AttributeValue
	for _, i := range src {
		m := map[string]*dynamodb.AttributeValue{}
		for key, val := range i {
			if val != nil {
				v := *val
				m[key] = &v
			}
		}
		dest = append(dest, m)
	}

	return dest
}

// deleteResult represents the result of delete operation
type deleteResult struct {
	err error
}

// batchDelete deletes the elements given in incoming channel and sends result to output channel
func batchDelete(ctx context.Context, c dynamodbiface.DynamoDBAPI, tableName string, batchResults <-chan batchResult) <-chan deleteResult {
	deleteResults := make(chan deleteResult)
	go func(batchResults <-chan batchResult) {
		defer close(deleteResults)
		for r := range batchResults {
			if r.err != nil {
				deleteResults <- deleteResult{
					err: r.err,
				}
				return
			}
			var wr []*dynamodb.WriteRequest
			for _, i := range r.value {
				wr = append(wr, &dynamodb.WriteRequest{
					DeleteRequest: &dynamodb.DeleteRequest{
						Key: i,
					},
				})
			}

			batchWriteItemInput := dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]*dynamodb.WriteRequest{
					tableName: wr,
				},
			}

			output, err := c.BatchWriteItem(&batchWriteItemInput)
			if err != nil {
				deleteResults <- deleteResult{
					err: err,
				}
				return
			}

			ui := output.UnprocessedItems
			for {
				if len(ui) > 0 {
					delUi := dynamodb.BatchWriteItemInput{
						RequestItems: ui,
					}
					o, err := c.BatchWriteItem(&delUi)
					if err != nil {
						deleteResults <- deleteResult{
							err: err,
						}
						return
					}
					ui = o.UnprocessedItems
				} else {
					break
				}
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}(batchResults)

	return deleteResults
}
