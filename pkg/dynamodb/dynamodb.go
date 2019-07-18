package dynamodb

import (
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

// ScanResult represents the result of scan operation
type ScanResult struct {
	error error
	value *map[string]*dynamodb.AttributeValue
}

// Scan scans DynamoDB table and sends result to result channel
func Scan(c dynamodbiface.DynamoDBAPI, tableName string) <-chan ScanResult {
	input := &dynamodb.ScanInput{
		TableName: &tableName,
	}
	resultStream := make(chan ScanResult)
	go func(resultStream chan<- ScanResult) {
		defer close(resultStream)
		err := c.ScanPages(input, func(output *dynamodb.ScanOutput, lastPage bool) bool {
			for _, item := range output.Items {
				select {
				case resultStream <- ScanResult{
					error: nil,
					value: &item,
				}:
				}
			}

			return lastPage == false
		})
		if err != nil {
			resultStream <- ScanResult{
				value: nil,
				error: err,
			}
		}
	}(resultStream)

	return resultStream
}

// DeleteResult represents the result of delete operation
type DeleteResult struct {
	error error
	value *dynamodb.BatchWriteItemOutput
}

// Delete deletes the elements given in incoming stream and sends result to output stream
func Delete(c dynamodbiface.DynamoDBAPI, tableName string, scanResultStream <-chan ScanResult) <-chan DeleteResult {
	deleteResultStream := make(chan DeleteResult)

	go func(resultStream <-chan ScanResult) {
		defer close(deleteResultStream)

		for r := range scanResultStream {
			if r.error != nil {
				panic(r.error)
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
			deleteResultStream <- DeleteResult{
				error: err,
				value: output,
			}
			// TODO [grokrz]: error handling and retries
		}
	}(scanResultStream)

	return deleteResultStream
}
