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
		scanResults := scan(ctx, c, "test", 10)
		counter := 0
		for elem := range scanResults {
			assert.NotNil(t, elem.value)
			counter++
		}

		assert.Equal(t, nrOfResults, counter)
	})

	// TODO [grokrz]: fix me bro
	//t.Run("Should panic when scan error", func(t *testing.T) {
	//	c := &dynamoDBErrorMock{}
	//})
}

func TestDelete(t *testing.T) {
	t.Run("Should delete", func(t *testing.T) {
		c := &dynamoDBMock{}
		batchResults := make(chan batchResult)
		ctx := context.TODO()
		batchDeleteResults := batchDelete(ctx, c, "test", batchResults)

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
			assert.Nil(t, r.error)
			counter += 1
		}

		assert.Equal(t, 1, counter)
	})
}

func TestMapToPrimaryKey(t *testing.T) {
	t.Run("Should map to primary key", func(t *testing.T) {
		scanResults := make(chan scanResult)
		ctx := context.TODO()

		dto := dynamodb.DescribeTableOutput{
			Table: &dynamodb.TableDescription{
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("id"),
						KeyType:       aws.String("S"),
					},
				},
			},
		}

		out := mapToPrimaryKey(ctx, dto.Table.KeySchema, scanResults)

		scanResult := scanResult{
			value: map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String("1"),
				},
				"apud": {
					S: aws.String("test"),
				},
			},
		}

		go func() {
			scanResults <- scanResult
			close(scanResults)
		}()

		counter := 0
		for r := range out {
			assert.Equal(t, "1", aws.StringValue(r.value["id"].S))
			assert.Nil(t, r.value["apud"])
			counter++
		}

		assert.Equal(t, 1, counter)
	})
}

func TestBatch(t *testing.T) {
	t.Run("Should batch", func(t *testing.T) {
		mapToPrimaryKeyResults := make(chan mapToPrimaryKeyResult)
		ctx := context.TODO()
		batchResults := batch(ctx, 25, mapToPrimaryKeyResults)

		mapToPrimaryKeyResu := mapToPrimaryKeyResult{
			value: map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String("1"),
				},
			},
		}

		go func() {
			for i := 0; i < 60; i++ {
				mapToPrimaryKeyResults <- mapToPrimaryKeyResu
			}
			close(mapToPrimaryKeyResults)
		}()

		counter := 0
		for r := range batchResults {
			assert.True(t, len(r.value) <= 25 && len(r.value) > 0)
			counter++
		}

		assert.Equal(t, 3, counter)
	})
}

func TestClone(t *testing.T) {
	t.Run("Should clone array", func(t *testing.T) {
		src := []map[string]*dynamodb.AttributeValue{
			{
				"id": &dynamodb.AttributeValue{
					S: aws.String("test0"),
				},
			},
		}

		dst := clone(src)

		src[0]["id"].S = aws.String("test1")

		assert.Equal(t, "test0", aws.StringValue(dst[0]["id"].S))
	})
}
