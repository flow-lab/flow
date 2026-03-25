package creds

import "fmt"

func GenerateConfigAndCredentials(key string, secret string, region string) (string, string, error) {
	config := fmt.Sprintf("[default]\noutput=json\nregion=%s", region)
	credentials := fmt.Sprintf("[default]\naws_access_key_id = %s\naws_secret_access_key = %s", key, secret)
	return config, credentials, nil
}

func GenerateEnv(region string, keyId string, accessKey string, sessToken string) (string, error) {
	env := fmt.Sprintf("AWS_REGION=%s\nAWS_ACCESS_KEY_ID=%s\nAWS_SECRET_ACCESS_KEY=%s\nAWS_SESSION_TOKEN=%s", region, keyId, accessKey, sessToken)
	return env, nil
}
