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
	client := &flowDynamoDBClient{d}
	return client, nil
}

// Delete deletes items from table. Will use filterExpression and expressionAttributeValues if given to only delete
// items defined or will delete all otherwise(purged).
//
// Implementation follows pipeline where:
// scan (generator) -> mapToPrimaryKey (stage) -> batch (stage) -> batchDelete (stage) -> client
// Each stage is no blocking and is using channels for communication.
//
// In case of error that program cannot recover the error will propagated to the client
// and processing will be stopped.
func (f *flowDynamoDBClient) Delete(ctx context.Context, tableName string, filterExpression *string, expressionAttributeValues *string) error {
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}
	describeTableOutput, err := f.DynamoDBAPI.DescribeTableWithContext(ctx, describeTableInput)
	if err != nil {
		return err
	}

	sr := scan(ctx, f.DynamoDBAPI, tableName, filterExpression, expressionAttributeValues, 100)
	pkr := mapToPrimaryKey(ctx, describeTableOutput.Table.KeySchema, sr)
	br := batch(ctx, 25, pkr)
	dr := batchDelete(ctx, f.DynamoDBAPI, tableName, br)

	for r := range dr {
		if r.err != nil {
			return r.err
		}
	}

	return nil
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
func scan(ctx context.Context, c dynamodbiface.DynamoDBAPI, tableName string, filterExpression *string, expressionAttributeValues *string, bufferSize int) <-chan scanResult {
	scanResults := make(chan scanResult, bufferSize)
	go func(scanResults chan<- scanResult) {
		defer close(scanResults)
		scanInput := &dynamodb.ScanInput{
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
		err := c.ScanPages(scanInput, func(output *dynamodb.ScanOutput, lastPage bool) bool {
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

type mapToPrimaryKeyResult struct {
	err   error
	value map[string]*dynamodb.AttributeValue
}

// mapToPrimaryKey will map item to only primary keys so it can be used as input to delete.
//
func mapToPrimaryKey(ctx context.Context, keySchemaElement []*dynamodb.KeySchemaElement, scanResults <-chan scanResult) <-chan mapToPrimaryKeyResult {
	mapToPrimaryKeyResults := make(chan mapToPrimaryKeyResult)
	go func(in <-chan scanResult) {
		defer close(mapToPrimaryKeyResults)

		for k := range in {
			if k.err != nil {
				mapToPrimaryKeyResults <- mapToPrimaryKeyResult{
					err: k.err,
				}
				return
			}

			key := make(map[string]*dynamodb.AttributeValue)
			for _, ks := range keySchemaElement {
				an := aws.StringValue(ks.AttributeName)
				key[an] = (k.value)[an]
			}

			mapToPrimaryKeyResults <- mapToPrimaryKeyResult{
				value: key,
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}(scanResults)

	return mapToPrimaryKeyResults
}

type batchResult struct {
	err   error
	value []map[string]*dynamodb.AttributeValue
}

// batch up to batchSize
func batch(ctx context.Context, batchSize int, mapToPrimaryKeyResults <-chan mapToPrimaryKeyResult) <-chan batchResult {
	batchResults := make(chan batchResult)
	go func(mapToPrimaryKeyResults <-chan mapToPrimaryKeyResult) {
		defer close(batchResults)

		b := make([]map[string]*dynamodb.AttributeValue, 0)
		for {
			select {
			case r, ok := <-mapToPrimaryKeyResults:
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
	}(mapToPrimaryKeyResults)

	return batchResults
}

func clone(src []map[string]*dynamodb.AttributeValue) []map[string]*dynamodb.AttributeValue {
	var dest []map[string]*dynamodb.AttributeValue
	for _, i := range src {
		for key, val := range i {
			if val != nil {
				v := *val
				dest = append(dest, map[string]*dynamodb.AttributeValue{
					key: &v,
				})
			}
		}
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

			batchWriteItemInput := &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]*dynamodb.WriteRequest{
					tableName: wr,
				},
			}

			output, err := c.BatchWriteItem(batchWriteItemInput)
			if err != nil {
				deleteResults <- deleteResult{
					err: err,
				}
				return
			}

			ui := output.UnprocessedItems
			for {
				if len(ui) > 0 {
					o, err := c.BatchWriteItem(&dynamodb.BatchWriteItemInput{})
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
