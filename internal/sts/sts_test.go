package sts

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/stretchr/testify/assert"
)

type stsMock struct {
	stsiface.STSAPI
}

func (m *stsMock) AssumeRoleWithContext(_ aws.Context, input *sts.AssumeRoleInput, _ ...request.Option) (*sts.AssumeRoleOutput, error) {
	return &sts.AssumeRoleOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     aws.String("AKID"),
			SecretAccessKey: aws.String("SECRET"),
			SessionToken:    aws.String("TOKEN"),
		},
	}, nil
}

func (m *stsMock) GetSessionTokenWithContext(_ aws.Context, input *sts.GetSessionTokenInput, _ ...request.Option) (*sts.GetSessionTokenOutput, error) {
	return &sts.GetSessionTokenOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     aws.String("AKID"),
			SecretAccessKey: aws.String("SECRET"),
			SessionToken:    aws.String("TOKEN"),
		},
	}, nil
}

type stsErrorMock struct {
	stsiface.STSAPI
}

func (m *stsErrorMock) AssumeRoleWithContext(_ aws.Context, _ *sts.AssumeRoleInput, _ ...request.Option) (*sts.AssumeRoleOutput, error) {
	return nil, fmt.Errorf("assume role failed")
}

func (m *stsErrorMock) GetSessionTokenWithContext(_ aws.Context, _ *sts.GetSessionTokenInput, _ ...request.Option) (*sts.GetSessionTokenOutput, error) {
	return nil, fmt.Errorf("get session token failed")
}

func TestAssumeRole(t *testing.T) {
	t.Run("should return credentials", func(t *testing.T) {
		cred, err := AssumeRole(context.Background(), &stsMock{}, 3600, "sess", "arn:aws:iam::111:role/test", "", "")

		assert.NoError(t, err)
		assert.Equal(t, "AKID", *cred.AccessKeyId)
		assert.Equal(t, "SECRET", *cred.SecretAccessKey)
		assert.Equal(t, "TOKEN", *cred.SessionToken)
	})

	t.Run("should return credentials with MFA", func(t *testing.T) {
		cred, err := AssumeRole(context.Background(), &stsMock{}, 3600, "sess", "arn:aws:iam::111:role/test", "arn:aws:iam::111:mfa/user", "123456")

		assert.NoError(t, err)
		assert.NotNil(t, cred)
	})

	t.Run("should return error", func(t *testing.T) {
		cred, err := AssumeRole(context.Background(), &stsErrorMock{}, 3600, "sess", "arn:aws:iam::111:role/test", "", "")

		assert.Error(t, err)
		assert.Nil(t, cred)
	})
}

func TestGetSessionToken(t *testing.T) {
	t.Run("should return credentials", func(t *testing.T) {
		cred, err := GetSessionToken(context.Background(), &stsMock{}, 3600, "", "")

		assert.NoError(t, err)
		assert.Equal(t, "AKID", *cred.AccessKeyId)
		assert.Equal(t, "SECRET", *cred.SecretAccessKey)
		assert.Equal(t, "TOKEN", *cred.SessionToken)
	})

	t.Run("should return credentials with MFA", func(t *testing.T) {
		cred, err := GetSessionToken(context.Background(), &stsMock{}, 3600, "arn:aws:iam::111:mfa/user", "123456")

		assert.NoError(t, err)
		assert.NotNil(t, cred)
	})

	t.Run("should return error", func(t *testing.T) {
		cred, err := GetSessionToken(context.Background(), &stsErrorMock{}, 3600, "", "")

		assert.Error(t, err)
		assert.Nil(t, cred)
	})
}
