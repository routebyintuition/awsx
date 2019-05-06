package awsx

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elasticache"
)

// RedisEndpoints provides an identifier for a primary endpoint
// and a slice of read endpoints
type RedisEndpoints struct {
	// The primary endpoint string
	Primary          *RedisEndpoint
	ClusterConfig    *RedisEndpoint
	ReadEndpoints    []*RedisEndpoint
	ReplicationGroup bool
	ReadReplicas     bool
	ClusterEnabled   bool
}

// RedisEndpoint provides the structure of each endpoint entry
type RedisEndpoint struct {
	Host  string
	Port  string
	Slots string
}

// PrimaryString provides the string representation of the host and port for use
// in libraries like redigo and go-redis of the primary endpoint
func (res *RedisEndpoints) PrimaryString() string {
	return res.Primary.Host + ":" + res.Primary.Port
}

// Readers returns a string slice of each read associated with the redis cluster
// These are each endpoints that can be used for read connections
func (res *RedisEndpoints) Readers() []string {
	str := make([]string, len(res.ReadEndpoints))
	for _, v := range res.ReadEndpoints {
		buff := v.Host + ":" + v.Port
		str = append(str, buff)
	}
	return str
}

// ClusterConfigString provides the cluster configuration endpoint for Redis Cluster
// if it is in use. Otherwise, an empty string
func (res *RedisEndpoints) ClusterConfigString() string {
	if res.ClusterEnabled {
		return res.ClusterConfig.Host + ":" + res.ClusterConfig.Port
	}

	return ""
}

// String provides the string representation of the host and port for use
// in libraries like redigo and go-redis
func (re *RedisEndpoint) String() string {
	return re.Host + ":" + re.Port
}

// String provides the string representation of all endpoints in JSON format
func (res *RedisEndpoints) String() string {
	jsonByte, _ := json.Marshal(res)
	return string(jsonByte)
}

// GetECReplicationGroup gathers information about the elasticache replication groups
func (a *Config) GetECReplicationGroup(cluster string) (*elasticache.DescribeReplicationGroupsOutput, int) {

	if a.Service.Ec == nil {
		a.SetECClient()
	}

	input := &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(cluster),
	}

	result, err := a.Service.Ec.DescribeReplicationGroups(input)
	if err != nil {
		return nil, 0
	}

	count := len(result.ReplicationGroups)
	if count == 0 {
		return nil, 0
	}

	return result, count
}

// GetRedisAllEndpoints returns type RedisEndpoints populated with either a single
// primary redis endpoint or also including a slice of endpoints for the read replica
// list
func (a *Config) GetRedisAllEndpoints(cluster string) (*RedisEndpoints, error) {
	var err error
	res := &RedisEndpoints{
		ReplicationGroup: false,
		ReadReplicas:     false,
	}
	res.ReadEndpoints = make([]*RedisEndpoint, 0)

	res, err = a.GetRedisPrimaryEndpoint(cluster)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// GetRedisPrimaryEndpoint returns a string representation of the cluster
// endpoint host and port for use with redigo and go-redis
// This ONLY returns the primary endpoint used for read/write operations
func (a *Config) GetRedisPrimaryEndpoint(cluster string) (*RedisEndpoints, error) {
	var err error

	res := &RedisEndpoints{
		ReplicationGroup: false,
		ReadReplicas:     false,
		ClusterEnabled:   false,
	}
	res.Primary = &RedisEndpoint{}
	if cluster == "" {
		return res, errors.New("no cluster name provided")
	}
	result, count := a.GetECReplicationGroup(cluster)
	if count == 0 {
		res.ReplicationGroup = false
	} else if count > 1 {
		res.ReplicationGroup = true
		return res, errors.New("more than one cluster matches the name provided")
	} else {
		res.ReplicationGroup = true
		if *result.ReplicationGroups[0].ClusterEnabled {
			res.ClusterEnabled = true
			res.ClusterConfig, err = a.GetRedisClusterEndpoint(cluster)
			if err != nil {
				return res, err
			}
		} else {
			res.ClusterEnabled = false
			res.Primary.Host = *result.ReplicationGroups[0].NodeGroups[0].PrimaryEndpoint.Address
			res.Primary.Port = strconv.FormatInt(*result.ReplicationGroups[0].NodeGroups[0].PrimaryEndpoint.Port, 10)
			if len(result.ReplicationGroups[0].NodeGroups[0].NodeGroupMembers) > 1 {
				res.ReadReplicas = true
				for _, v := range result.ReplicationGroups[0].NodeGroups[0].NodeGroupMembers {
					entry := &RedisEndpoint{
						Host: *v.ReadEndpoint.Address,
						Port: strconv.FormatInt(*v.ReadEndpoint.Port, 10),
					}
					res.ReadEndpoints = append(res.ReadEndpoints, entry)
				}
			}
		}
	}

	if !res.ReplicationGroup {
		list, _ := a.GetECClusterDetails(cluster)

		if len(list.CacheClusters) == 0 {
			return nil, errors.New("no replication groups or cache clusters associated with this cluster name")
		}
		if len(list.CacheClusters) > 1 {
			res.ReadReplicas = true
			return nil, errors.New("more than one cache cluster associated with this name")
		}
		if list.CacheClusters[0].CacheNodes[0].Endpoint != nil {
			res.Primary.Host = *list.CacheClusters[0].CacheNodes[0].Endpoint.Address
			res.Primary.Port = strconv.FormatInt(*list.CacheClusters[0].CacheNodes[0].Endpoint.Port, 10)
		} else {
			return nil, errors.New("no cache cluster endpoint or replication group associated with this custer name")
		}
	}

	return res, nil
}

// GetRedisClusterEndpoint returns a string representation of the cluster
// endpoint host ane port for use with Redigo and go-redis as host:port
// This value is the configuration endpoint from elasticache
func (a *Config) GetRedisClusterEndpoint(cluster string) (*RedisEndpoint, error) {
	re := &RedisEndpoint{}
	if cluster == "" {
		return re, errors.New("no cluster name provided")
	}
	result, count := a.GetECReplicationGroup(cluster)
	if count == 0 {
		return re, errors.New("no cluster existing matching provided name")
	}
	if count > 1 {
		return re, errors.New("more than one cluster matches the name provided")
	}

	if result.ReplicationGroups[0].ConfigurationEndpoint == nil {
		return re, errors.New("no cluster endpoint found, perhaps this is not a cluster configuration")
	}

	re.Host = *result.ReplicationGroups[0].ConfigurationEndpoint.Address
	re.Port = strconv.FormatInt(*result.ReplicationGroups[0].ConfigurationEndpoint.Port, 10)

	return re, nil
}

// GetECClusterDetails provides the initial call to describe the identified cluster
func (a *Config) GetECClusterDetails(cluster string) (*elasticache.DescribeCacheClustersOutput, error) {

	if cluster == "" {
		if a.panicOnErr {
			panic("panicOnErr enabled, must provide a cluster string to (a *Config) GetClusterDetails(cluster string)")
		}
		return nil, errors.New("did not provide a cluster name for the RDS describe call")
	}

	input := &elasticache.DescribeCacheClustersInput{
		CacheClusterId:    aws.String(cluster),
		ShowCacheNodeInfo: aws.Bool(true),
	}

	result, err := a.Service.Ec.DescribeCacheClusters(input)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetECClient returns a client for use with AWS Elasticache
func (a *Config) GetECClient() *elasticache.ElastiCache {
	return a.Service.Ec
}

// SetECClient returns a client for use with AWS Elasticache
func (a *Config) SetECClient() *Config {
	if a.Service == nil {
		panic("Must initialize Service struct with NewRDS()")
	}
	a.Service.Ec = elasticache.New(a.Session)

	return a
}
