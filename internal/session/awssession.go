package session

import (
	"os"

	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
)

func NewSessionWithSharedProfile(profile string) *session.Session {
	opts := session.Options{
		SharedConfigState:       session.SharedConfigEnable,
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
	}
	if profile != "" && !hasEnvCredentials() {
		opts.Profile = profile
	}
	return session.Must(session.NewSessionWithOptions(opts))
}

func NewSessionWithSharedProfileWithOptions(profile string, options *session.Options) *session.Session {
	if options == nil {
		return NewSessionWithSharedProfile(profile)
	}
	if options.SharedConfigState == session.SharedConfigDisable {
		options.SharedConfigState = session.SharedConfigEnable
	}
	if profile != "" && options.Profile == "" {
		options.Profile = profile
	}
	return session.Must(session.NewSessionWithOptions(*options))
}

func hasEnvCredentials() bool {
	return (os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != "") ||
		os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI") != "" ||
		os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != ""
}
