package dynamodb

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/stretchr/testify/assert"
	"testing"
)

type dynamoDBMock struct {
	dynamodbiface.DynamoDBAPI
}

func (d *dynamoDBMock) ScanPages(input *dynamodb.ScanInput, callback func(*dynamodb.ScanOutput, bool) bool) error {
	for i := 0; i < 5; i++ {
		output := &dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id": &dynamodb.AttributeValue{
						S: aws.String(fmt.Sprintf("%d", i)),
					},
				},
			},
		}
		callback(output, i == 4)
	}
	return nil
}

func (d *dynamoDBMock) BatchWriteItem(*dynamodb.BatchWriteItemInput) (*dynamodb.BatchWriteItemOutput, error) {
	return &dynamodb.BatchWriteItemOutput{}, nil
}

type dynamoDBErrorMock struct {
	dynamodbiface.DynamoDBAPI
}

func (d *dynamoDBErrorMock) ScanPages(input *dynamodb.ScanInput, callback func(*dynamodb.ScanOutput, bool) bool) error {
	return fmt.Errorf("got an error")
}

func TestScan(t *testing.T) {
	t.Run("Should scan", func(t *testing.T) {
		c := &dynamoDBMock{}
		resultStream := Scan(c, "test")

		for elem := range resultStream {
			assert.Nil(t, elem.error)
			assert.NotNil(t, elem.value)
		}
	})

	t.Run("Should emit error", func(t *testing.T) {
		c := &dynamoDBErrorMock{}
		resultStream := Scan(c, "test")

		for elem := range resultStream {
			assert.NotNil(t, elem.error)
			assert.Nil(t, elem.value)
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("Should delete", func(t *testing.T) {
		c := &dynamoDBMock{}
		scanResultStream := make(chan ScanResult)
		deleteResultStream := Delete(c, "test", scanResultStream)
		scanResultStream <- ScanResult{
			error: nil,
			value: &map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String("1"),
				},
			},
		}

		close(scanResultStream)

		counter := 0
		for r := range deleteResultStream {
			assert.Nil(t, r.error)
			assert.NotNil(t, r.value)
			counter += 1
		}

		assert.Equal(t, 1, counter)
	})
}
