package logs

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/stretchr/testify/assert"
)

type cloudwatchLogsMock struct {
	cloudwatchlogsiface.CloudWatchLogsAPI
}

func (c *cloudwatchLogsMock) PutRetentionPolicy(input *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	return nil, nil
}

func (c *cloudwatchLogsMock) DescribeLogGroupsPagesWithContext(ctx aws.Context, input *cloudwatchlogs.DescribeLogGroupsInput, f func(*cloudwatchlogs.DescribeLogGroupsOutput, bool) bool, req ...request.Option) error {
	output := &cloudwatchlogs.DescribeLogGroupsOutput{
		LogGroups: []*cloudwatchlogs.LogGroup{
			{
				LogGroupName: aws.String("test"),
				StoredBytes:  aws.Int64(int64(1)),
			},
		},
	}
	f(output, true)
	return nil
}

func (c *cloudwatchLogsMock) DescribeLogStreamsPagesWithContext(ctx aws.Context, input *cloudwatchlogs.DescribeLogStreamsInput, f func(*cloudwatchlogs.DescribeLogStreamsOutput, bool) bool, req ...request.Option) error {
	output := &cloudwatchlogs.DescribeLogStreamsOutput{
		LogStreams: []*cloudwatchlogs.LogStream{
			{
				LogStreamName: aws.String("stream-1"),
			},
		},
	}
	f(output, true)
	return nil
}

func (c *cloudwatchLogsMock) GetLogEventsPagesWithContext(ctx aws.Context, input *cloudwatchlogs.GetLogEventsInput, f func(*cloudwatchlogs.GetLogEventsOutput, bool) bool, req ...request.Option) error {
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	output := &cloudwatchlogs.GetLogEventsOutput{
		Events: []*cloudwatchlogs.OutputLogEvent{
			{
				Timestamp: &ts,
				Message:   aws.String("test message"),
			},
		},
	}
	f(output, true)
	return nil
}

type cloudwatchLogsErrorMock struct {
	cloudwatchlogsiface.CloudWatchLogsAPI
}

func (c *cloudwatchLogsErrorMock) DescribeLogStreamsPagesWithContext(ctx aws.Context, input *cloudwatchlogs.DescribeLogStreamsInput, f func(*cloudwatchlogs.DescribeLogStreamsOutput, bool) bool, req ...request.Option) error {
	return fmt.Errorf("describe log streams failed")
}

func TestSetRetention(t *testing.T) {
	err := SetRetention("test", int64(2), &cloudwatchLogsMock{})
	assert.Nil(t, err)
}

func TestDescribe(t *testing.T) {
	t.Run("should describe log groups", func(t *testing.T) {
		output, err := Describe(context.Background(), nil, &cloudwatchLogsMock{})

		assert.Nil(t, err)
		assert.Equal(t, 1, len(output))
	})

	t.Run("should describe log groups with prefix", func(t *testing.T) {
		prefix := "test"
		output, err := Describe(context.Background(), &prefix, &cloudwatchLogsMock{})

		assert.Nil(t, err)
		assert.Equal(t, 1, len(output))
	})

	t.Run("should skip empty prefix", func(t *testing.T) {
		prefix := ""
		output, err := Describe(context.Background(), &prefix, &cloudwatchLogsMock{})

		assert.Nil(t, err)
		assert.Equal(t, 1, len(output))
	})
}

func TestSummary(t *testing.T) {
	t.Run("should calculate storage summary", func(t *testing.T) {
		s := Summary([]*cloudwatchlogs.LogGroup{
			{StoredBytes: aws.Int64(1000000000000)},
			{StoredBytes: aws.Int64(1000000000000)},
		})

		assert.Equal(t, int64(2000000), s.StoredMegaBytes)
		assert.Equal(t, int64(2000), s.StoredGigaBytes)
		assert.Equal(t, int64(2), s.StoredTeraBytes)
	})

	t.Run("should handle zero bytes", func(t *testing.T) {
		s := Summary([]*cloudwatchlogs.LogGroup{
			{StoredBytes: aws.Int64(0)},
		})

		assert.Equal(t, int64(0), s.StoredMegaBytes)
		assert.Equal(t, int64(0), s.StoredGigaBytes)
		assert.Equal(t, int64(0), s.StoredTeraBytes)
	})

	t.Run("should handle empty list", func(t *testing.T) {
		s := Summary([]*cloudwatchlogs.LogGroup{})

		assert.Equal(t, int64(0), s.StoredMegaBytes)
		assert.Equal(t, 0, len(s.LogGroups))
	})
}

func TestWriteLogEvents(t *testing.T) {
	t.Run("should write log events to csv", func(t *testing.T) {
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)

		err := WriteLogEvents(
			context.Background(),
			"test-group",
			time.Now().Add(-1*time.Hour),
			time.Now(),
			&cloudwatchLogsMock{},
			writer,
		)

		writer.Flush()
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "test message")
		assert.Contains(t, buf.String(), "stream-1")
	})

	t.Run("should return error when describe log streams fails", func(t *testing.T) {
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)

		err := WriteLogEvents(
			context.Background(),
			"test-group",
			time.Now().Add(-1*time.Hour),
			time.Now(),
			&cloudwatchLogsErrorMock{},
			writer,
		)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to describe log streams")
	})
}
