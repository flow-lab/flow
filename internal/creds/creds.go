package creds

import (
	"bytes"
	"text/template"
)

const (
	configTemplate      = "[default]\noutput=json\nregion={{.}}"
	credentialsTemplate = "[default]\naws_access_key_id = {{.Key}}\naws_secret_access_key = {{.Secret}}"
	envTemplate         = "AWS_REGION={{.Region}}\nAWS_ACCESS_KEY_ID={{.KeyId}}\nAWS_SECRET_ACCESS_KEY={{.AccessKey}}\nAWS_SESSION_TOKEN={{.SessToken}}"
)

// GenerateConfigAndCredentials generate config and credentials file representation.
func GenerateConfigAndCredentials(key string, secret string, region string) (error, string, string) {
	t, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return err, "", ""
	}
	var cb bytes.Buffer
	err = t.ExecuteTemplate(&cb, "config", region)

	c, err := template.New("credentials").Parse(credentialsTemplate)
	if err != nil {
		return err, "", ""
	}
	var credB bytes.Buffer
	err = c.ExecuteTemplate(&credB, "credentials", struct {
		Key    string
		Secret string
	}{
		Key:    key,
		Secret: secret,
	})

	return err, string(cb.Bytes()), string(credB.Bytes())
}

// GenerateEnv generates credentials as env vars.
func GenerateEnv(region string, keyId string, accessKey string, sessToken string) (error, string) {
	t, err := template.New("env").Parse(envTemplate)
	if err != nil {
		return err, ""
	}
	var b bytes.Buffer
	err = t.ExecuteTemplate(&b, "env", struct {
		Region    string
		KeyId     string
		AccessKey string
		SessToken string
	}{
		Region:    region,
		KeyId:     keyId,
		AccessKey: accessKey,
		SessToken: sessToken,
	})

	return err, b.String()
}
