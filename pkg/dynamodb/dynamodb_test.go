package dynamodb

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/stretchr/testify/assert"
	"testing"
)

const nrOfResults = 5

type dynamoDBMock struct {
	dynamodbiface.DynamoDBAPI
}

func (d *dynamoDBMock) ScanPages(input *dynamodb.ScanInput, callback func(*dynamodb.ScanOutput, bool) bool) error {
	for i := 0; i < nrOfResults; i++ {
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

		ctx := context.TODO()
		resultStream := scan(ctx, c, "test", 10)
		counter := 0
		for elem := range resultStream {
			assert.Nil(t, elem.error)
			assert.NotNil(t, elem.value)
			counter++
		}

		assert.Equal(t, nrOfResults, counter)
	})

	t.Run("Should emit error", func(t *testing.T) {
		c := &dynamoDBErrorMock{}
		resultStream := scan(context.TODO(), c, "test", 0)

		for elem := range resultStream {
			assert.NotNil(t, elem.error)
			assert.Nil(t, elem.value)
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("Should delete", func(t *testing.T) {
		c := &dynamoDBMock{}
		scanResultStream := make(chan scanResult)
		ctx := context.TODO()
		deleteResultStream := delete(ctx, c, "test", scanResultStream)
		scanResultStream <- scanResult{
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

func TestBatch(t *testing.T) {
	t.Run("Should batch", func(t *testing.T) {
		scanResultStream := make(chan scanResult)
		ctx := context.TODO()
		batchResultStream := batch(ctx, 25, scanResultStream)

		scanResult := scanResult{
			error: nil,
			value: &map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String("1"),
				},
			},
		}

		go func() {
			for i := 0; i < 60; i++ {
				scanResultStream <- scanResult
			}
			close(scanResultStream)
		}()

		counter := 0
		for r := range batchResultStream {
			assert.True(t, len(r.value) <= 25 && len(r.value) > 0)
			counter++
		}

		assert.Equal(t, 3, counter)
	})
}
