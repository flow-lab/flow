package sts

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

// AssumeRole in aws account and return credentials when assume worked as expected
func AssumeRole(ctx context.Context, stsapi stsiface.STSAPI, durationSeconds int64, roleSessionName string, roleArn string, serialNr string, tokenCode string) (error, *sts.Credentials) {
	input := sts.AssumeRoleInput{
		DurationSeconds: aws.Int64(durationSeconds),
		RoleSessionName: aws.String(roleSessionName),
		RoleArn:         aws.String(roleArn),
	}

	if serialNr != "" {
		input.SerialNumber = aws.String(serialNr)
	}

	if tokenCode != "" {
		input.TokenCode = aws.String(tokenCode)
	}

	res, err := stsapi.AssumeRoleWithContext(ctx, &input)
	if err != nil {
		return err, nil
	}

	return nil, res.Credentials
}
