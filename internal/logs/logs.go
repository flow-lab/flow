package logs

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
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
func Describe(logGroupNamePrefix *string, c cloudwatchlogsiface.CloudWatchLogsAPI) ([]*cloudwatchlogs.LogGroup, error) {
	input := &cloudwatchlogs.DescribeLogGroupsInput{}

	if logGroupNamePrefix != nil && *logGroupNamePrefix != "" {
		input.LogGroupNamePrefix = logGroupNamePrefix
	}
	var logGroups []*cloudwatchlogs.LogGroup
	err := c.DescribeLogGroupsPages(input, func(output *cloudwatchlogs.DescribeLogGroupsOutput, lastPage bool) bool {
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

	// The number of giga bytes stored.
	StoredGigaBytes *int64
	// The number of giga bytes stored.
	StoredMegaBytes *int64
	// The number of tera bytes stored.
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
