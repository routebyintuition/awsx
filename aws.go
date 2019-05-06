package awsx

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/service/elasticache"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
)

// Config is the configuration definition for our AWS services.
type Config struct {
	Region       string // should set AWS region used or a default is used
	Role         string // optional: only if using to assume an AWS role
	AccessKey    string // optional: only used if requiring AWS access key/secret key authentication
	SecretKey    string // optional: only used if requiring AWS access key/secret key authentication
	SessionToken string // optional: only used if requiring AWS access key/secret key authentication
	Endpoint     string // optional: use a specified endpoint for calls
	CredFile     string // optional: credentials file to use
	Profile      string // optional: which credential profile to utilize
	Providers    []credentials.Provider
	Session      *session.Session
	Service      *Services
	ServiceSts   *Services
	panicOnErr   bool // Should we panic the app or proceed if we can't publish to CWL
}

// Services stores the used client types so I don't have to remember to do that.
type Services struct {
	Rds *rds.RDS
	Ec  *elasticache.ElastiCache
}

// NewAWS creates a new Config struct and populates it with an empty provider chain
func NewAWS() *Config {
	p := make([]credentials.Provider, 0)
	return &Config{Providers: p, Service: &Services{}, ServiceSts: &Services{}}
}

// WithStatic adds a static credential provider to the provider chain
func (a *Config) WithStatic() *Config {
	// If the static credentials are provided and who knows why but maybe
	// a token is included in there?
	if a.AccessKey != "" && a.SecretKey != "" {
		v := credentials.Value{
			AccessKeyID:     a.AccessKey,
			SecretAccessKey: a.SecretKey,
		}
		if a.SessionToken != "" {
			v.SessionToken = a.SessionToken

		}

		a.Providers = append(a.Providers, &credentials.StaticProvider{Value: v})

	} else {
		fmt.Println("No static AWS credebtials found")
	}

	return a
}

// EnablePanic will set panicOnErr to true so that if we hit any errors, the application
// will panic out
func (a *Config) EnablePanic() *Config {
	a.panicOnErr = true
	return a
}

// DisablePanic will set panicOnErr to false so that we do not panic the application
// in care there is an error
func (a *Config) DisablePanic() *Config {
	a.panicOnErr = false
	return a
}

// SetRegion sets the AWS region to use with the service calls
func (a *Config) SetRegion(region string) *Config {
	if len(region) > 0 {
		a.Region = region
	} else {
		fmt.Println("No region specified in call to SetRegion(region string)")
	}
	return a
}

// SetProfile sets the profile name to be used with authentication
func (a *Config) SetProfile(profile string) *Config {
	if len(profile) > 0 {
		a.Profile = profile
	} else {
		fmt.Println("No profile specified in call to SetProfile(profile string)")
	}
	return a
}

// SetEndpoint sets the endpoint to use if this is a custom value
func (a *Config) SetEndpoint(endpoint string) *Config {
	if len(endpoint) > 0 {
		a.Endpoint = endpoint
	} else {
		fmt.Println("No Endpoint specified in all to SetEndpoint(endpoint string)")
	}
	return a
}

// WithEnv adds the environment provider to the credential chain so that
// if AWS environment credentials are available, they will be used for auth
func (a *Config) WithEnv() *Config {
	a.Providers = append(a.Providers, &credentials.EnvProvider{})
	return a
}

// WithFile adds the credentials file to the provider chain so that during
// authentication, the application will look for the credentials file inthe default locations.
// In addition to the default locations, if a.awsCredFile is set, we will look at that
// location for the AWS credentials.
// If a.awsProfile is set, this will enable the use of a profile name other than default
func (a *Config) WithFile() *Config {
	// Path to the shared credentials file.
	if a.CredFile != "" {
		cfile := &credentials.SharedCredentialsProvider{
			Filename: a.CredFile,
		}
		if a.Profile != "" {
			cfile.Profile = a.Profile
		}
		a.Providers = append(a.Providers, cfile)
	} else {
		a.Providers = append(a.Providers, &credentials.SharedCredentialsProvider{})
	}

	return a
}

// WithInstanceRole adds the credentials from the EC2 instance obtained from the
// metadata service to the provider list.
func (a *Config) WithInstanceRole() *Config {
	lowTimeoutClient := &http.Client{Timeout: 3 * time.Second} // low timeout to ec2 metadata service

	// RemoteCredProvider for default remote endpoints such as EC2 or ECS IAM Roles
	def := defaults.Get()
	def.Config.HTTPClient = lowTimeoutClient
	a.Providers = append(a.Providers, defaults.RemoteCredProvider(*def.Config, def.Handlers))

	// EC2RoleProvider retrieves credentials from the EC2 service, and keeps track if those credentials are expired
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("Error on connecting to AWS: ", err)
		if a.panicOnErr {
			fmt.Println("panicOnError is enabled so exiting...")
			os.Exit(1)
		}
		return nil
	}
	a.Providers = append(a.Providers, &ec2rolecreds.EC2RoleProvider{
		Client: ec2metadata.New(sess, &aws.Config{
			HTTPClient: lowTimeoutClient,
		}),
		ExpiryWindow: 3,
	})

	return a
}

//WithAllProviders provides a chain of credentials for connectivity
func (a *Config) WithAllProviders() *Config {

	// If the static credentials are provided and who knows why but maybe
	// a token is included in there?
	if a.AccessKey != "" && a.SecretKey != "" {
		v := credentials.Value{
			AccessKeyID:     a.AccessKey,
			SecretAccessKey: a.SecretKey,
		}
		if a.SessionToken != "" {
			v.SessionToken = a.SessionToken

		}

		a.Providers = append(a.Providers, &credentials.StaticProvider{Value: v})
	}

	// Check the local ENV variables for the right credentials
	a.Providers = append(a.Providers, &credentials.EnvProvider{})

	// Path to the shared credentials file.
	if a.CredFile != "" {
		cfile := &credentials.SharedCredentialsProvider{
			Filename: a.CredFile,
		}
		if a.Profile != "" {
			cfile.Profile = a.Profile
		}
		a.Providers = append(a.Providers, cfile)
	} else {
		a.Providers = append(a.Providers, &credentials.SharedCredentialsProvider{})
	}

	httpTimeout := &http.Client{Timeout: 3 * time.Second} // low timeout to ec2 metadata service

	// RemoteCredProvider for default remote endpoints such as EC2 or ECS IAM Roles
	def := defaults.Get()
	def.Config.HTTPClient = httpTimeout
	a.Providers = append(a.Providers, defaults.RemoteCredProvider(*def.Config, def.Handlers))

	// EC2RoleProvider retrieves credentials from the EC2 service, and keeps track if those credentials are expired
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("Error on connecting to AWS: ", err)
		if a.panicOnErr {
			fmt.Println("panicOnError is enabled so exiting...")
			os.Exit(1)
		}
		return nil
	}
	a.Providers = append(a.Providers, &ec2rolecreds.EC2RoleProvider{
		Client: ec2metadata.New(sess, &aws.Config{
			HTTPClient: httpTimeout,
		}),
		ExpiryWindow: 3,
	})

	return a
}

// SetSession calls GetSession and sets the session return as a struct param
func (a *Config) SetSession() *Config {
	a.Session = a.GetSession()
	return a
}

// GetSession creates a new session based upon the Config struct we built using the
// above functions
func (a *Config) GetSession() *session.Session {
	if len(a.Providers) == 0 {
		fmt.Println("Calling GetSession() without initializing a credential provider using With*() methods.")
		if a.panicOnErr {
			panic("No credential providers specified")
		}
	}

	Config := defaults.Config()

	if a.Region != "" {
		Config.WithRegion(a.Region)
	} else if val, ok := os.LookupEnv("AWS_DEFAULT_REGION"); ok {
		Config.WithRegion(val)
	} else {
		Config.WithRegion("us-east-1")
	}

	if a.Endpoint != "" {
		Config.WithEndpoint(a.Endpoint)
	}

	Config.WithCredentials(
		credentials.NewChainCredentials(a.Providers),
	)

	// create new session with config
	sess, err := session.NewSessionWithOptions(
		session.Options{
			Config: *Config,
		},
	)
	if err != nil {
		return nil
	}

	return sess
}
