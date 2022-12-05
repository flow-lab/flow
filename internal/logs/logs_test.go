package logs

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/stretchr/testify/assert"
	"testing"
)

type cloudwatchLogsMock struct {
	cloudwatchlogsiface.CloudWatchLogsAPI
}

func (c *cloudwatchLogsMock) PutRetentionPolicy(input *cloudwatchlogs.PutRetentionPolicyInput) (*cloudwatchlogs.PutRetentionPolicyOutput, error) {
	return nil, nil
}

func (c *cloudwatchLogsMock) DescribeLogGroupsPages(input *cloudwatchlogs.DescribeLogGroupsInput, f func(*cloudwatchlogs.DescribeLogGroupsOutput, bool) bool) error {
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

func TestSetRetention(t *testing.T) {
	cloudwatchLogsMock := &cloudwatchLogsMock{}
	err := SetRetention("test", int64(2), cloudwatchLogsMock)

	assert.Nil(t, err)
}

func TestDescribe(t *testing.T) {
	cloudwatchLogsMock := &cloudwatchLogsMock{}
	output, err := Describe(context.Background(), nil, cloudwatchLogsMock)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(output))
}

func TestSummary(t *testing.T) {
	logGroupSummary := Summary([]*cloudwatchlogs.LogGroup{
		{
			StoredBytes: aws.Int64(1000000000000),
		},
		{
			StoredBytes: aws.Int64(1000000000000),
		},
	})

	assert.Equal(t, int64(2000000), *logGroupSummary.StoredMegaBytes)
	assert.Equal(t, int64(2000), *logGroupSummary.StoredGigaBytes)
	assert.Equal(t, int64(2), *logGroupSummary.StoredTeraBytes)
}
