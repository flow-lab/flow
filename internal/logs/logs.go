package logs

import (
	"context"
	"encoding/csv"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/pkg/errors"
	"log"
	"strconv"
	"time"
)

// SetRetention sets retention for CloudWatch log group
// retentionDays - the number of days to retain the log events in the specified log group. Possible
// values are: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731,
// 1827, and 3653.
func SetRetention(logGroupName string, retentionDays int64, c cloudwatchlogsiface.CloudWatchLogsAPI) error {
	input := &cloudwatchlogs.PutRetentionPolicyInput{
		LogGroupName:    &logGroupName,
		RetentionInDays: aws.Int64(retentionDays),
	}

	_, err := c.PutRetentionPolicy(input)
	if err != nil {
		return err
	}

	return nil
}

// Describe returns details for log groups
func Describe(ctx context.Context, logGroupNamePrefix *string, c cloudwatchlogsiface.CloudWatchLogsAPI) ([]*cloudwatchlogs.LogGroup, error) {
	input := &cloudwatchlogs.DescribeLogGroupsInput{}

	if logGroupNamePrefix != nil && *logGroupNamePrefix != "" {
		input.LogGroupNamePrefix = logGroupNamePrefix
	}
	var logGroups []*cloudwatchlogs.LogGroup
	err := c.DescribeLogGroupsPagesWithContext(ctx, input, func(output *cloudwatchlogs.DescribeLogGroupsOutput, lastPage bool) bool {
		for _, lg := range output.LogGroups {
			logGroups = append(logGroups, lg)
		}
		return lastPage == false
	})
	if err != nil {
		return nil, err
	}

	return logGroups, nil
}

type LogGroupSummary struct {
	// The log groups.
	LogGroups []*cloudwatchlogs.LogGroup

	// The number of gigabytes stored.
	StoredGigaBytes *int64
	// The number of gigabytes stored.
	StoredMegaBytes *int64
	// The number of terabytes stored.
	StoredTeraBytes *int64
}

// Summary sums up stored bytes and converts them to GB
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
		StoredMegaBytes: &storedMegaBytes,
		StoredGigaBytes: &storedGigaBytes,
		StoredTeraBytes: &storedTeraBytes,
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
		for _, ls := range output.LogStreams {
			logStreams = append(logStreams, ls)
		}
		return lastPage == false
	})
	if err != nil {
		return errors.Wrap(err, "failed to describe log streams")
	}

	for _, logStream := range logStreams {
		input := &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  &logGroupName,
			LogStreamName: logStream.LogStreamName,
			StartFromHead: aws.Bool(true),
			StartTime:     aws.Int64(startTime.UnixNano() / int64(time.Millisecond)),
			EndTime:       aws.Int64(endTime.UnixNano() / int64(time.Millisecond)),
		}
		var logEvents []*LogEvent
		err := c.GetLogEventsPagesWithContext(ctx, input, func(output *cloudwatchlogs.GetLogEventsOutput, lastPage bool) bool {
			for _, le := range output.Events {
				print(".")
				event := LogEvent{
					Timestamp:     le.Timestamp,
					Message:       le.Message,
					LogStreamName: logStream.LogStreamName,
				}
				logEvents = append(logEvents, &event)
			}

			for _, le := range logEvents {
				if err := writer.Write([]string{strconv.FormatInt(*le.Timestamp, 10), *le.Message, *le.LogStreamName}); err != nil {
					// not much we can do here unfortunately
					log.Fatalf("failed to write to csv: %v", err)
				}
			}
			writer.Flush()
			logEvents = nil
			return lastPage == false
		})
		if err != nil {
			return err
		}
	}

	return err
}
