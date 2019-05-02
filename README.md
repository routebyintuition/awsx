# awsx

---

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
