# awsx

Package provides Go AWS utilities that I typically have to repeat and use often.

## Install

    go get -u github.com/routebyintuition/awsx

## Usage

For standard AWS connectivity, we have the capability to create a new session based upon the various authentication
mechanisms built into the AWS Go SDK.

In the below example, we are using the Amazon EC2 credentials to create a new session:

    input := &s3.ListBucketsInput{}

    a := awsx.NewAWS().EnablePanic().WithInstanceRole()
    a.SetRegion("us-east-1")
    a.SetSession()

    s3 := s3.New(a.Session)

    result, err := s3.ListBuckets(input)
    if err != nil {
        fmt.Println("Error: ", err)
        os.Exit(1)
    }

    fmt.Println(result)

### RediGo

If you would like to grab the Redis ElastiCache endpoints for the primary Redis endpoint, the read-only endpoints, or the cluster configuration endpoint for use with a library like go-redis, you can see examples below:

    a := awsx.NewAWS().EnablePanic().WithFile()
    a.SetRegion("us-west-2")
    a.SetSession()

    endpoint, err := a.GetRedisPrimaryEndpoint("cluster-name")
    if err != nil {
        fmt.Println(err)
    }

    redisPool := redis.NewPool(func() (redis.Conn, error) {
        c, err := redis.Dial("tcp", endpoint.PrimaryString())

        if err != nil {
            return nil, err
        }

        return c, err
    }, 100)

The above example works for standard clusters that have Redis Cluster Mode disabled. You can check for the cluster mode with:

    fmt.Println("Primary Endpoint: ", endpoint.PrimaryString())

    if endpoint.ClusterEnabled {
        fmt.Println("Cluster Mode Enabled")
    } else {
        fmt.Println("Cluster Mode Disabled")
    }

We can also pull out the read replicas for their own connections to read:

    endpoint, err := a.GetRedisPrimaryEndpoint("redis-cluster")

    if err != nil {
        fmt.Println(err)
    }

    if endpoint.ClusterEnabled {
        fmt.Println("Cluster Mode Enabled")
        fmt.Println("Cluster Configuration Endpoint: ", endpoint.ClusterConfigString())
    } else {
        fmt.Println("Cluster Mode Disabled")
        fmt.Println("Primary Endpoint: ", endpoint.PrimaryString())
    }

    if endpoint.ReadReplicas {
        fmt.Println("Read Replicas available: ")
        for _, v := range endpoint.ReadEndpoints {
            fmt.Println("Read Replica: ", v)
        }
    }

    /*
    Outputs:
    Cluster Mode Disabled
    Primary Endpoint:  redis-cluster.XXXXXX.ng.0001.XXXX.cache.amazonaws.com:6379
    Read Replicas available:
    Read Replica:  redis-cluster-001.XXXXXX.0001.XXXX.cache.amazonaws.com:6379
    Read Replica:  redis-cluster-002.XXXXXX.0001.XXXX.cache.amazonaws.com:6379
    */

### Redis Cluster

You can also use this tool to find your cluster configuration endpoint for use with Redis cluster:

    endpoint, err := a.GetRedisPrimaryEndpoint("cluster-name")
    if err != nil {
        fmt.Println(err)
    }

    if endpoint.ClusterEnabled {
        fmt.Println("Cluster Mode Enabled")
        fmt.Println("Cluster Configuration Endpoint: ", endpoint.ClusterConfigString())
    } else {
        fmt.Println("Cluster Mode Disabled")
        fmt.Println("Primary Endpoint: ", endpoint.PrimaryString())
    }

## Additional Information

Original connection methods used from https://github.com/C2FO/vfs with the AWS connection implementation for the S3 io.Writer.

## To Do

* Add cloudwatch IO writer
* Add RDS master/replica lookups - MySQL
* Add RDS MySQL IAM auth
* Add RDS master/replica lookups - PostgreSQL
* Add PostgreSQL IAM auth
* Add Redis cluster init
* Add Memcached server lookup
