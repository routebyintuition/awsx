package awsx

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

// AWSConfig is the configuration definition for our AWS services.
type AWSConfig struct {
	awsRegion       string // should set AWS region used or a default is used
	awsRole         string // optional: only if using to assume an AWS role
	awsAccessKey    string // optional: only used if requiring AWS access key/secret key authentication
	awsSecretKey    string // optional: only used if requiring AWS access key/secret key authentication
	awsSessionToken string // optional: only used if requiring AWS access key/secret key authentication
	awsEndpoint     string // optional: use a specified endpoint for calls
	awsCredFile     string // optional: credentials file to use
	awsProfile      string // optional: which credential profile to utilize
	awsProviders    []credentials.Provider
	panicOnErr      bool // Should we panic the app or proceed if we can't publish to CWL
}

func NewAWS() *AWSConfig {
	p := make([]credentials.Provider, 0)
	return &AWSConfig{awsProviders: p}
}

func (a *AWSConfig) WithStatic() *AWSConfig {
	// If the static credentials are provided and who knows why but maybe
	// a token is included in there?
	if a.awsAccessKey != "" && a.awsSecretKey != "" {
		v := credentials.Value{
			AccessKeyID:     a.awsAccessKey,
			SecretAccessKey: a.awsSecretKey,
		}
		if a.awsSessionToken != "" {
			v.SessionToken = a.awsSessionToken

		}

		a.awsProviders = append(a.awsProviders, &credentials.StaticProvider{Value: v})

	} else {
		fmt.Println("No static AWS credebtials found")
	}

	return a
}

// EnablePanic will set panicOnErr to true so that if we hit any errors, the application
// will panic out
func (a *AWSConfig) EnablePanic() *AWSConfig {
	a.panicOnErr = true
	return a
}

// DisablePanic will set panicOnErr to false so that we do not panic the application
// in care there is an error
func (a *AWSConfig) DisablePanic() *AWSConfig {
	a.panicOnErr = false
	return a
}

// WithEnv adds the environment provider to the credential chain so that
// if AWS environment credentials are available, they will be used for auth
func (a *AWSConfig) WithEnv() *AWSConfig {
	a.awsProviders = append(a.awsProviders, &credentials.EnvProvider{})
	return a
}

// WithFile adds the credentials file to the provider chain so that during
// authentication, the application will look for the credentials file inthe default locations.
// In addition to the default locations, if a.awsCredFile is set, we will look at that
// location for the AWS credentials.
// If a.awsProfile is set, this will enable the use of a profile name other than default
func (a *AWSConfig) WithFile() *AWSConfig {
	// Path to the shared credentials file.
	if a.awsCredFile != "" {
		cfile := &credentials.SharedCredentialsProvider{
			Filename: a.awsCredFile,
		}
		if a.awsProfile != "" {
			cfile.Profile = a.awsProfile
		}
		a.awsProviders = append(a.awsProviders, cfile)
	} else {
		a.awsProviders = append(a.awsProviders, &credentials.SharedCredentialsProvider{})
	}

	return a
}

//auth provides a chain of credentials for connectivity
//func (c *CwConfig) auth() []credentials.Provider {
func (a *AWSConfig) auth() []credentials.Provider {
	p := make([]credentials.Provider, 0)

	// If the static credentials are provided and who knows why but maybe
	// a token is included in there?
	if a.awsAccessKey != "" && a.awsSecretKey != "" {
		v := credentials.Value{
			AccessKeyID:     a.awsAccessKey,
			SecretAccessKey: a.awsSecretKey,
		}
		if a.awsSessionToken != "" {
			v.SessionToken = a.awsSessionToken

		}

		p = append(p, &credentials.StaticProvider{Value: v})

	}

	// Check the local ENV variables for the right credentials
	p = append(p, &credentials.EnvProvider{})

	// Path to the shared credentials file.
	if a.awsCredFile != "" {
		cfile := &credentials.SharedCredentialsProvider{
			Filename: a.awsCredFile,
		}
		if a.awsProfile != "" {
			cfile.Profile = a.awsProfile
		}
		p = append(p, cfile)
	} else {
		p = append(p, &credentials.SharedCredentialsProvider{})
	}

	lowTimeoutClient := &http.Client{Timeout: 1 * time.Second} // low timeout to ec2 metadata service

	// RemoteCredProvider for default remote endpoints such as EC2 or ECS IAM Roles
	def := defaults.Get()
	def.Config.HTTPClient = lowTimeoutClient
	p = append(p, defaults.RemoteCredProvider(*def.Config, def.Handlers))

	// EC2RoleProvider retrieves credentials from the EC2 service, and keeps track if those credentials are expired
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("Error on connecting to AWS: ", err)
		if a.panicOnErr {
			fmt.Println("CloudWatchLogs panicOnError is enabled so existing...")
			os.Exit(1)
		}
		return nil
	}
	p = append(p, &ec2rolecreds.EC2RoleProvider{
		Client: ec2metadata.New(sess, &aws.Config{
			HTTPClient: lowTimeoutClient,
		}),
		ExpiryWindow: 3,
	})

	return p
}

func (a *AWSConfig) Session() *session.Session {
	awsConfig := defaults.Config()

	if a.awsRegion != "" {
		awsConfig.WithRegion(a.awsRegion)
	} else if val, ok := os.LookupEnv("AWS_DEFAULT_REGION"); ok {
		awsConfig.WithRegion(val)
	} else {
		awsConfig.WithRegion("us-east-1")
	}

	if a.awsEndpoint != "" {
		awsConfig.WithEndpoint(a.awsEndpoint)
	}

	a.auth()

	awsConfig.WithCredentials(
		credentials.NewChainCredentials(a.awsProviders),
	)

	// create new session with config
	sess, err := session.NewSessionWithOptions(
		session.Options{
			Config: *awsConfig,
		},
	)
	if err != nil {
		return nil
	}

	return sess
}
