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
	for i := range nrOfResults {
		output := dynamodb.ScanOutput{
			Items: []map[string]*dynamodb.AttributeValue{
				{
					"id": &dynamodb.AttributeValue{
						S: new(fmt.Sprintf("%d", i)),
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
					AttributeName: new("id"),
					KeyType:       new("S"),
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
					AttributeName: new("id"),
					KeyType:       new("S"),
				},
			},
		},
	}, nil
}

func TestFlowDynamoDBClient_Delete(t *testing.T) {
	t.Run("Should delete - happy path", func(t *testing.T) {
		c := NewFlowDynamoDBClient(&dynamoDBMock{})

		err := c.Delete(context.TODO(), "test", nil, nil)

		assert.Nil(t, err)
	})

	t.Run("Should stop - not so happy path", func(t *testing.T) {
		c := NewFlowDynamoDBClient(&dynamoDBErrorMock{})

		err := c.Delete(context.TODO(), "test", nil, nil)

		assert.NotNil(t, err)
	})
}

func TestScan(t *testing.T) {
	t.Run("Should scan", func(t *testing.T) {
		c := dynamoDBMock{}

		ctx := context.TODO()
		scanResults := scan(ctx, &c, "test", new("test"), nil, nil, nil, 10)
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
		scanResults := scan(ctx, &c, "test", nil, nil, nil, nil, 10)

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
				S: new("1"),
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
		batchResults := make(chan batchResult)
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
					S: new("1"),
				},
			},
		}

		go func() {
			for range 60 {
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
					S: new("test0"),
				},
				"test": &dynamodb.AttributeValue{
					S: new("test1"),
				},
			},
		}

		dst := clone(src)

		src[0]["id"].S = new("test1")

		assert.Equal(t, 1, len(dst))
		assert.Equal(t, "test0", aws.StringValue(dst[0]["id"].S))
		assert.Equal(t, "test1", aws.StringValue(dst[0]["test"].S))
	})
}

func TestProjectionExpression(t *testing.T) {
	t.Run("Should crate projection expression from many keys with expression attribute names", func(t *testing.T) {
		keySchemas := []*dynamodb.KeySchemaElement{
			{
				AttributeName: new("id"),
				KeyType:       new("S"),
			},
			{
				AttributeName: new("date"),
				KeyType:       new("S"),
			},
		}

		expression, expressionAttributeNames := projectionExpression(keySchemas)

		assert.Equal(t, "#id0, #date1", *expression)

		assert.Equal(t, "date", *expressionAttributeNames["#date1"])
		assert.Equal(t, "id", *expressionAttributeNames["#id0"])
	})
}
