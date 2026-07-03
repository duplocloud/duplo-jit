package internal

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/duplocloud/duplo-jit/duplocloud"
)

type AwsConfigOutput struct {
	Version         int    `json:"Version"`
	ConsoleUrl      string `json:"ConsoleUrl"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	Region          string `json:"Region"`
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
		Region:          creds.Region,
		SessionToken:    creds.SessionToken,
		Expiration:      expiration.Format(time.RFC3339),
	}
}

// OverrideConsoleRegion returns the AWS federation console URL with the region of
// its Destination target replaced. Admin / duplo-ops JIT URLs open in the master
// account's default region; when a tenant is selected we want the console to open
// in that tenant's region instead. The SigninToken is left untouched.
func OverrideConsoleRegion(consoleUrl, region string) string {
	if consoleUrl == "" || region == "" {
		return consoleUrl
	}
	u, err := url.Parse(consoleUrl)
	if err != nil {
		return consoleUrl
	}
	q := u.Query()
	dest := q.Get("Destination")
	if dest == "" {
		return consoleUrl
	}
	d, err := url.Parse(dest)
	if err != nil {
		return consoleUrl
	}
	dq := d.Query()
	if dq.Get("region") == "" {
		return consoleUrl
	}
	dq.Set("region", region)
	d.RawQuery = dq.Encode()
	q.Set("Destination", d.String())
	u.RawQuery = q.Encode()
	return u.String()
}

// ApplyTenantRegion overrides the region reported by admin / duplo-ops JIT
// credentials so that both the Region field and the console URL point at the
// selected tenant's region.
func ApplyTenantRegion(creds *AwsConfigOutput, region string) {
	if creds == nil || region == "" {
		return
	}
	creds.ConsoleUrl = OverrideConsoleRegion(creds.ConsoleUrl, region)
	creds.Region = region
}

func OutputAwsCreds(creds *AwsConfigOutput, cacheKey string) {

	// Write the creds to the cache.
	cacheFile := fmt.Sprintf("%s,aws-creds.json", cacheKey)
	json := cacheWriteMustMarshal(cacheFile, creds)

	// Write the creds to the output.
	_, _ = os.Stdout.Write(json)
	_, _ = os.Stdout.WriteString("\n")
}

func PingAWSCreds(creds *AwsConfigOutput) error {
	credsProvider := aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(creds.AccessKeyId, creds.SecretAccessKey, creds.SessionToken))

	// Create an AWS config using the creds.
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(creds.Region),
		config.WithCredentialsProvider(credsProvider),
	)
	if err != nil {
		return err
	}

	// Create an STS client with the AWS config.
	stsClient := sts.NewFromConfig(cfg)

	// Call the STS client API for get-caller-identity to test cred validity.
	_, err = stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return err
	}

	return nil
}
