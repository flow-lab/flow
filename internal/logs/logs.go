package logs

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"strconv"
	"time"
)

func SetRetention(logGroupName string, retentionDays int64, c cloudwatchlogsiface.CloudWatchLogsAPI) error {
	input := &cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    &logGroupName,
		RetentionInDays: aws.Int64(retentionDays),
	}
	_, err := c.PutRetentionPolicy(input)
	return err
}

func Describe(ctx context.Context, logGroupNamePrefix *string, c cloudwatchlogsiface.CloudWatchLogsAPI) ([]*cloudwatchlogs.LogGroup, error) {
	input := &cloudwatchlogs.DescribeLogGroupsInput{}

	if logGroupNamePrefix != nil && *logGroupNamePrefix != "" {
		input.LogGroupNamePrefix = logGroupNamePrefix
	}
	var logGroups []*cloudwatchlogs.LogGroup
	err := c.DescribeLogGroupsPagesWithContext(ctx, input, func(output *cloudwatchlogs.DescribeLogGroupsOutput, lastPage bool) bool {
		logGroups = append(logGroups, output.LogGroups...)
		return !lastPage
	})
	if err != nil {
		return nil, err
	}

	return logGroups, nil
}

type LogGroupSummary struct {
	LogGroups       []*cloudwatchlogs.LogGroup
	StoredGigaBytes int64
	StoredMegaBytes int64
	StoredTeraBytes int64
}

func Summary(logGroups []*cloudwatchlogs.LogGroup) *LogGroupSummary {
	storedBytes := int64(0)
	for _, lg := range logGroups {
		storedBytes += *lg.StoredBytes
	}

	var storedMegaBytes int64
	var storedGigaBytes int64
	var storedTeraBytes int64
	if storedBytes != 0 {
		storedMegaBytes = storedBytes / 1e+6
		storedGigaBytes = storedBytes / 1e+9
		storedTeraBytes = storedBytes / 1e+12
	}

	return &LogGroupSummary{
		LogGroups:       logGroups,
		StoredMegaBytes: storedMegaBytes,
		StoredGigaBytes: storedGigaBytes,
		StoredTeraBytes: storedTeraBytes,
	}
}

type LogEvent struct {
	Timestamp     *int64
	Message       *string
	LogStreamName *string
}

func WriteLogEvents(ctx context.Context, logGroupName string, startTime, endTime time.Time, c cloudwatchlogsiface.CloudWatchLogsAPI, writer *csv.Writer) error {
	describeLogStreamsInput := cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: &logGroupName,
		Descending:   aws.Bool(true),
		OrderBy:      aws.String(cloudwatchlogs.OrderByLastEventTime),
	}
	var logStreams []*cloudwatchlogs.LogStream
	err := c.DescribeLogStreamsPagesWithContext(ctx, &describeLogStreamsInput, func(output *cloudwatchlogs.DescribeLogStreamsOutput, lastPage bool) bool {
		logStreams = append(logStreams, output.LogStreams...)
		return !lastPage
	})
	if err != nil {
		return fmt.Errorf("failed to describe log streams: %w", err)
	}

	for _, logStream := range logStreams {
		input := &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  &logGroupName,
			LogStreamName: logStream.LogStreamName,
			StartFromHead: aws.Bool(true),
			StartTime:     aws.Int64(startTime.UnixNano() / int64(time.Millisecond)),
			EndTime:       aws.Int64(endTime.UnixNano() / int64(time.Millisecond)),
		}
		err := c.GetLogEventsPagesWithContext(ctx, input, func(output *cloudwatchlogs.GetLogEventsOutput, lastPage bool) bool {
			for _, le := range output.Events {
				print(".")
				if err := writer.Write([]string{strconv.FormatInt(*le.Timestamp, 10), *le.Message, *logStream.LogStreamName}); err != nil {
					fmt.Printf("failed to write to csv: %v\n", err)
					return false
				}
			}
			writer.Flush()
			return !lastPage
		})
		if err != nil {
			return err
		}
	}

	return nil
}
