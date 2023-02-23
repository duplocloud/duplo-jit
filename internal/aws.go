package internal

import (
	"fmt"
	"os"
	"time"

	"github.com/duplocloud/duplo-aws-jit/duplocloud"
)

type AwsConfigOutput struct {
	Version         int    `json:"Version"`
	ConsoleUrl      string `json:"ConsoleUrl"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken,omitempty"`
	Expiration      string `json:"Expiration,omitempty"`
}

func ConvertAwsCreds(creds *duplocloud.AwsJitCredentials) *AwsConfigOutput {
	// Calculate the expiration time.
	now := time.Now().UTC()
	validity := creds.Validity
	if validity <= 0 {
		validity = 3600 // default is one hour
	}
	expiration := now.Add(time.Duration(validity) * time.Second)

	// Build the resulting credentials to be output.
	return &AwsConfigOutput{
		Version:         1,
		ConsoleUrl:      creds.ConsoleURL,
		AccessKeyId:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		Expiration:      expiration.Format(time.RFC3339),
	}
}

func OutputAwsCreds(creds *AwsConfigOutput, cacheKey string) {

	// Write the creds to the cache.
	cacheFile := fmt.Sprintf("%s,aws-creds.json", cacheKey)
	json := cacheWriteMustMarshal(cacheFile, creds)

	// Write the creds to the output.
	os.Stdout.Write(json)
	os.Stdout.WriteString("\n")
}
