package main

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"os"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"net/http/httptest"
	"net/http"
	"fmt"
	"time"
	"github.com/aws/aws-sdk-go/aws"
)

func setEnv() {
	os.Setenv("AWS_ACCESS_KEY_ID", "accessKey")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
}

func Test_awsSession(t *testing.T) {
	type args struct {
		profile string
	}

	tests := []struct {
		name             string
		args             args
		wantValidSession bool
		want             *session.Session
		before           func()
		after            func()
		expectToken      string
	}{
		{
			name: "test env provider",
			args: args{
				profile: "test1",
			},
			before: setEnv,
			after:  os.Clearenv,
		},
		{
			name: "test default shared credentials provider",
			before: func() {
				os.Clearenv()
				os.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join("testdata", "credentials"))
			},
			after:       os.Clearenv,
			expectToken: "token",
		}, {
			name: "test env provider",
			args: args{
				profile: "test1",
			},
			before: setEnv,
			after:  os.Clearenv,
		},
		{
			name: "test default shared credentials provider",
			before: func() {
				os.Clearenv()
				os.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join("testdata", "credentials"))
			},
			after:       os.Clearenv,
			expectToken: "token",
		},
		{
			name: "test profile with shared credentials provider",
			before: func() {
				os.Clearenv()
				os.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join("testdata", "credentials"))
			},
			args: args{
				profile: "no_token",
			},
			after:       os.Clearenv,
			expectToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.before != nil {
				tt.before()
			}

			sess := NewSessionWithSharedProfile(tt.args.profile)
			assert.NotNil(t, sess)

			creds, err := sess.Config.Credentials.Get()
			assert.Nil(t, err)

			assert.Equal(t, "accessKey", creds.AccessKeyID, "Expect access key ID to match")
			assert.Equal(t, "secret", creds.SecretAccessKey, "Expect secret access key to match")
			assert.Equal(t, tt.expectToken, creds.SessionToken, "Expect token to match")

			if tt.after != nil {
				tt.after()
			}
		})
	}
}

const assumeRoleRespMsg = `
<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <AssumedRoleUser>
      <Arn>arn:aws:sts::account_id:assumed-role/role/session_name</Arn>
      <AssumedRoleId>AKID:session_name</AssumedRoleId>
    </AssumedRoleUser>
    <Credentials>
      <AccessKeyId>AKID</AccessKeyId>
      <SecretAccessKey>SECRET</SecretAccessKey>
      <SessionToken>SESSION_TOKEN</SessionToken>
      <Expiration>%s</Expiration>
    </Credentials>
  </AssumeRoleResult>
  <ResponseMetadata>
    <RequestId>request-id</RequestId>
  </ResponseMetadata>
</AssumeRoleResponse>
`

func Test_awsSession_mfa(t *testing.T) {
	os.Clearenv()
	defer os.Clearenv()
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join("testdata", "credentials"))
	os.Setenv("AWS_CONFIG_FILE", filepath.Join("testdata", "config"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.FormValue("SerialNumber"), "arn:aws:iam::1111111111:mfa/test")
		assert.Equal(t, r.FormValue("TokenCode"), "tokencode")

		w.Write([]byte(fmt.Sprintf(assumeRoleRespMsg, time.Now().Add(15 * time.Minute).Format("2006-01-02T15:04:05Z"))))
	}))

	customProviderCalled := false

	options := &session.Options{
		Profile: "cloudformation@flowlab-dev",
		Config: aws.Config{
			Region:     aws.String("eu-west-1"),
			Endpoint:   aws.String(server.URL),
			DisableSSL: aws.Bool(true),
		},
		SharedConfigState: session.SharedConfigEnable,
		AssumeRoleTokenProvider: func() (string, error) {
			customProviderCalled = true
			return "tokencode", nil
		},
	}

	sess := NewSessionWithSharedProfileWithOptions("cloudformation@flowlab-dev", options)

	creds, err := sess.Config.Credentials.Get()
	assert.NoError(t, err)
	assert.True(t, customProviderCalled)
	assert.Contains(t, creds.ProviderName, "AssumeRoleProvider")
}
