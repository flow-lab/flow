package dynamodb

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
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
		output := dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id": &dynamodb.AttributeValue{
						S: aws.String(fmt.Sprintf("%d", i)),
					},
				},
			},
		}
		callback(&output, i == 4)
	}
	return nil
}

func (d *dynamoDBMock) BatchWriteItem(*dynamodb.BatchWriteItemInput) (*dynamodb.BatchWriteItemOutput, error) {
	return &dynamodb.BatchWriteItemOutput{}, nil
}

func (d *dynamoDBMock) DescribeTableWithContext(aws.Context, *dynamodb.DescribeTableInput, ...request.Option) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{
		Table: &dynamodb.TableDescription{
			KeySchema: []*dynamodb.KeySchemaElement{
				{
					AttributeName: aws.String("id"),
					KeyType:       aws.String("S"),
				},
			},
		},
	}, nil
}

type dynamoDBErrorMock struct {
	dynamodbiface.DynamoDBAPI
}

func (d *dynamoDBErrorMock) ScanPages(input *dynamodb.ScanInput, callback func(*dynamodb.ScanOutput, bool) bool) error {
	return fmt.Errorf("got an error")
}

func (d *dynamoDBErrorMock) DescribeTableWithContext(aws.Context, *dynamodb.DescribeTableInput, ...request.Option) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{
		Table: &dynamodb.TableDescription{
			KeySchema: []*dynamodb.KeySchemaElement{
				{
					AttributeName: aws.String("id"),
					KeyType:       aws.String("S"),
				},
			},
		},
	}, nil
}

func TestFlowDynamoDBClient_Delete(t *testing.T) {
	t.Run("Should delete - happy path", func(t *testing.T) {
		c, err := NewFlowDynamoDBClient(&dynamoDBMock{})
		assert.Nil(t, err)

		err = c.Delete(context.TODO(), "test", nil, nil)

		assert.Nil(t, err)
	})

	t.Run("Should stop - not so happy path", func(t *testing.T) {
		c, err := NewFlowDynamoDBClient(&dynamoDBErrorMock{})
		assert.Nil(t, err)

		err = c.Delete(context.TODO(), "test", nil, nil)

		assert.NotNil(t, err)
	})
}

func TestScan(t *testing.T) {
	t.Run("Should scan", func(t *testing.T) {
		c := dynamoDBMock{}

		ctx := context.TODO()
		scanResults := scan(ctx, &c, "test", aws.String("test"), nil, nil, 10)
		counter := 0
		for elem := range scanResults {
			assert.NotNil(t, elem.value)
			counter++
		}

		assert.Equal(t, nrOfResults, counter)
	})

	t.Run("Should send error to result channel", func(t *testing.T) {
		c := dynamoDBErrorMock{}

		ctx := context.TODO()
		scanResults := scan(ctx, &c, "test", nil, nil, nil, 10)

		counter := 0
		for elem := range scanResults {
			assert.NotNil(t, elem.err)
			counter++
		}

		assert.Equal(t, 1, counter)
	})
}

func TestBatchDelete(t *testing.T) {
	t.Run("Should delete", func(t *testing.T) {
		c := dynamoDBMock{}
		batchResults := make(chan batchResult)
		ctx := context.TODO()
		batchDeleteResults := batchDelete(ctx, &c, "test", batchResults)

		var m []map[string]*dynamodb.AttributeValue
		m = append(m, map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String("1"),
			},
		})
		batchResults <- batchResult{
			value: m,
		}

		close(batchResults)

		counter := 0
		for r := range batchDeleteResults {
			assert.Nil(t, r.err)
			counter += 1
		}

		// it only sends error, it does not send empty messages
		assert.Equal(t, 0, counter)
	})

	t.Run("Should send error to result channel", func(t *testing.T) {
		c := dynamoDBErrorMock{}

		ctx := context.TODO()
		batchResults := make(chan batchResult, 0)
		scanResults := batchDelete(ctx, &c, "test", batchResults)

		batchResults <- batchResult{
			err: fmt.Errorf("test error"),
		}

		counter := 0
		for elem := range scanResults {
			assert.NotNil(t, elem.err)
			counter++
		}

		assert.Equal(t, 1, counter)
	})
}

func TestBatch(t *testing.T) {
	t.Run("Should batch", func(t *testing.T) {
		scanResults := make(chan scanResult)
		ctx := context.TODO()
		batchResults := batch(ctx, 25, scanResults)

		scanResult := scanResult{
			value: map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String("1"),
				},
			},
		}

		go func() {
			for i := 0; i < 60; i++ {
				scanResults <- scanResult
			}
			close(scanResults)
		}()

		counter := 0
		for r := range batchResults {
			assert.True(t, len(r.value) <= 25 && len(r.value) > 0)
			counter++
		}

		assert.Equal(t, 3, counter)
	})

	t.Run("Should send error to result channel", func(t *testing.T) {
		ctx := context.TODO()
		scanResults := make(chan scanResult)
		batchResults := batch(ctx, 1, scanResults)

		scanResults <- scanResult{
			err: fmt.Errorf("test error"),
		}

		counter := 0
		for elem := range batchResults {
			assert.NotNil(t, elem.err)
			counter++
		}

		assert.Equal(t, 1, counter)
	})
}

func TestClone(t *testing.T) {
	t.Run("Should clone array", func(t *testing.T) {
		src := []map[string]*dynamodb.AttributeValue{
			{
				"id": &dynamodb.AttributeValue{
					S: aws.String("test0"),
				},
				"test": &dynamodb.AttributeValue{
					S: aws.String("test1"),
				},
			},
		}

		dst := clone(src)

		src[0]["id"].S = aws.String("test1")

		assert.Equal(t, 1, len(dst))
		assert.Equal(t, "test0", aws.StringValue(dst[0]["id"].S))
		assert.Equal(t, "test1", aws.StringValue(dst[0]["test"].S))
	})
}

func TestProjectionExpression(t *testing.T) {
	t.Run("Should crate projection expression from one keys", func(t *testing.T) {
		keySchemas := []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("S"),
			},
		}

		exp := projectionExpression(keySchemas)

		assert.Equal(t, "id", *exp)
	})

	t.Run("Should crate projection expression from many keys", func(t *testing.T) {
		keySchemas := []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("S"),
			},
			{
				AttributeName: aws.String("date"),
				KeyType:       aws.String("S"),
			},
		}

		exp := projectionExpression(keySchemas)

		assert.Equal(t, "id,date", *exp)
	})
}
