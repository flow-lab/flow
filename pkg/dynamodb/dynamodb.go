package dynamodb

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

// scanResult represents the result of scan operation
type scanResult struct {
	value *map[string]*dynamodb.AttributeValue
}

// Scan scans DynamoDB table and sends result to result channel
func scan(ctx context.Context, c dynamodbiface.DynamoDBAPI, tableName string, bufferSize int) <-chan scanResult {
	input := &dynamodb.ScanInput{
		TableName: &tableName,
	}
	resultStream := make(chan scanResult, bufferSize)
	go func(resultStream chan<- scanResult) {
		defer close(resultStream)
		err := c.ScanPages(input, func(output *dynamodb.ScanOutput, lastPage bool) bool {
			for _, item := range output.Items {
				resultStream <- scanResult{
					value: &item,
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
			// TODO [grokrz]: error handling
			resultStream <- scanResult{
				value: nil,
			}
		}
	}(resultStream)

	return resultStream
}

type item struct {
	value map[string]*dynamodb.AttributeValue
}

// mapToPrimaryKey will map item to only primary keys so it can be used as input to delete
func mapToPrimaryKey(ctx context.Context, keySchemaElement []*dynamodb.KeySchemaElement, in chan scanResult) <-chan item {
	out := make(chan item)
	go func(in <-chan scanResult) {
		defer close(out)

		for k := range in {
			select {
			case <-ctx.Done():
				return
			default:
			}

			key := make(map[string]*dynamodb.AttributeValue)
			for _, ks := range keySchemaElement {
				an := aws.StringValue(ks.AttributeName)
				key[an] = (*k.value)[an]
			}

			out <- item{
				value: key,
			}
		}
	}(in)

	return out
}

type batchResult struct {
	value []*map[string]*dynamodb.AttributeValue
}

// batch up to batchSize
func batch(ctx context.Context, batchSize int, scanResultStream <-chan scanResult) <-chan batchResult {
	out := make(chan batchResult)
	go func(in <-chan scanResult) {
		b := make([]*map[string]*dynamodb.AttributeValue, 0)

	ReadFromChannel:
		for {
			select {
			case r, ok := <-in:
				if ok == false {
					// channel has been closed, emit and close the out channel
					if len(b) > 0 {
						out <- batchResult{
							value: b,
						}
						b = b[:0]
					}
					close(out)
					break ReadFromChannel
				}

				b = append(b, r.value)
				if len(b) == batchSize {
					out <- batchResult{
						value: b,
					}
					b = b[:0]
				}
			case <-ctx.Done():
				return
			default:
			}
		}
	}(scanResultStream)

	return out
}

// DeleteResult represents the result of delete operation
type deleteResult struct {
	error error
	value *dynamodb.BatchWriteItemOutput
}

// Delete deletes the elements given in incoming stream and sends result to output stream
func delete(ctx context.Context, c dynamodbiface.DynamoDBAPI, tableName string, scanResultStream <-chan scanResult) <-chan deleteResult {
	deleteResultStream := make(chan deleteResult)

	go func(resultStream <-chan scanResult) {
		defer close(deleteResultStream)
		for r := range scanResultStream {

			// in case the context has been canceled
			select {
			case <-ctx.Done():
				return
			default:
			}

			input := &dynamodb.WriteRequest{
				DeleteRequest: &dynamodb.DeleteRequest{
					Key: *r.value,
				},
			}

			batchWriteItemInput := &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]*dynamodb.WriteRequest{
					tableName: {
						input,
					},
				},
			}

			output, err := c.BatchWriteItem(batchWriteItemInput)
			deleteResultStream <- deleteResult{
				error: err,
				value: output,
			}
			// TODO [grokrz]: error handling and retries
		}
	}(scanResultStream)

	return deleteResultStream
}
