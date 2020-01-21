package creds

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreds(t *testing.T) {

	t.Run("Should generate Config and Credentials", func(t *testing.T) {
		err, config, credentials := GenerateConfigAndCredentials("A", "B", "eu-west-1")

		assert.Nil(t, err)
		expectedConfig := "[default]\noutput=json\nregion=eu-west-1"
		assert.Equal(t, expectedConfig, config)

		expectedCredentials := "[default]\naws_access_key_id = A\naws_secret_access_key = B"
		assert.Equal(t, expectedCredentials, credentials)
	})

	t.Run("Should generate Credentials anv for Intellij", func(t *testing.T) {
		err, env := GenerateEnv("eu-west-1", "a", "b", "c")

		assert.Nil(t, err)
		expectedEnv := "AWS_REGION=eu-west-1\nAWS_ACCESS_KEY_ID=a\nAWS_SECRET_ACCESS_KEY=b\nAWS_SESSION_TOKEN=c"
		assert.Equal(t, expectedEnv, env)
	})

}
