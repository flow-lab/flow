# Flow AWS tooling [![Build Status](https://travis-ci.org/flow-lab/flow.svg?branch=master)](https://travis-ci.org/flow-lab/flow) [![Snap Status](https://build.snapcraft.io/badge/flow-lab/flow.svg)](https://build.snapcraft.io/user/flow-lab/flow)

Flow is a CLI tooling that helps developing applications on AWS. Is meant to be used only in dev environments
and not in production.

NOTE: Flow CLI is under development, and may occasionally make backwards-incompatible changes.

## Installation

* [homebrew](https://github.com/flow-lab/homebrew-tap)

    ```sh
    brew install flow-lab/tap/flow
    ```

* [snap](https://snapcraft.io/flow)

    ```sh
    sudo snap install flow --classic
    ``` 

* latest version from github releases

    ```sh
    curl https://raw.githubusercontent.com/flow-lab/flow/master/bin/get-latest.sh --output get-latest.sh
    chmod +x get-latest.sh
    ./get-latest.sh
    ```
    
* local with go get

    ```sh
    go get -u github.com/flow-lab/flow
    cd $GOPATH/src/github.com/flow-lab/flow
    go install ./...
    ```

## example usage:

### dynamodb

* delete all items from DynamoDB "TestTable" (using scan operation)

    `flow dynamodb delete --table-name TestTable`

* delete all items from DynamoDB "TestTable" with amount > 100

    `flow dynamodb delete --table-name TestTable --filter-expression "amount > :amount" --expression-attribute-values '{":amount":{"N":"100"}}'`
    
* change table capacity for **Provisioned** capacity mode

    `flow dynamodb capacity --table-name TestTable --write 10 --read 10`
    
* describe table

    `flow dynamodb describe-table --table-name test`
    
* put item(s) from file

    `flow dynamodb put-item --input input.json --table-name TestTable`

    where, file input.json contains a list of json objects in dynamodb item format:
    ```json
    [
        {
          "id": {
            "S": "1"
          },
          "val": {
            "S": "1"
          }
        }
    ]
    ```
    
* delete item(s) from file

    `flow dynamodb delete-item --input input.json --table-name TestTable`

    where, file input.json contains a list of json objects in dynamodb item format:
    ```json
    [
        {
          "id": {
            "S": "1"
          },
          "val": {
            "S": "1"
          }
        }
    ]
    ``` 
    
* count items using scan operation

    `flow dynamodb count-item --table-name TestTable`    
    
### s3

* purge all items and its versions from the bucket, items with delete markers will also be removed

    `flow s3 purge --bucket-name "test-bucket"`

### ssm

* export all ssm parameters and their values to json file

    `flow ssm export`
    
### sqs

* send message to the queue

    `flow sqs send --queue-name apud --input '{"id":"1","status":"ACTIVE"}' --message-attributes '{"eventType":{"DataType":"String","StringValue":"STATUS_UPDATED"}}'`

* receive message from the queue

    `flow sqs receive-message --queue-name apud`

* purge all messages

    `flow sqs purge --queue-name apud`
    
* delete message

    `flow sqs delete-message --queue-name test --receipt-handle "31451c88-bd89-4d75-b3dc-252d2ab583d1" --receipt-handle "AQEB5Ox73VW9r+893F2ET+Nuw20ASW2tDol3eQgiptmDUDb2Ombya1amTXco5vuhoSmxnCxY6cpeUJMrzjW2IjBoPTLI/5Fp4QP8GTnYpcqTSWIMYc9eZ/UELmJ5IIfiDjr3eCeJmdU96CiYQ8PaeA+r3x5zJkVmmOGsw1v/OUK+iVJiyrPWBRQaBjuawh34iS4h51NO8Nweu4Ctnf7q18c7dfRWzdljWPAHB7OKAzYYBSU6Q9pxaTFK7sgoC0DJIGswMDYA3yVW7zsDhOWHm4bRWnW8pGbltDSnhpihL2OisaNQn+EL0R0iYFKXF14PRpEaeG8MlkBndz76nf8tlDYvBbtFL9xubmQFJYGrZ5OHorEs4mnuZbmgftTOuejnlLBt"`
    
* describe sqs queue

    `flow sqs describe --queue-name apud`

### apigateway

* exports all API specifications in swagger or oas3 specification and saves to file(s)
    
    `flow apigateway export`
  
### kafka (MSK) - Amazon Managed Streaming for Apache Kafka

* list MSK clusters in the account

    `flow kafka list-clusters`

* describe MSK cluster

    `flow kafka describe-cluster --cluster-name "MSK-Dev"`

* get bootstrap brokers

    `flow kafka get-bootstrap-brokers --cluster-name "MSK-Dev"`

* send message to topic

     `flow kafka send --cluster-name "MSK-Dev" --topic "topic-name" --message "test"`

* create topic

     `flow kafka create-topic --cluster-name "MSK-Dev" --topic "topic-name" --num-partitions 1 --replication-factor 1 --retention-ms "-1"`

* delete topic

     `flow kafka delete-topic --cluster-name "MSK-Dev" --topic "topic-name"`
     
* describe topic

     `flow kafka describe-topic --cluster-name "MSK-Dev" --topic "topic-name-0" --topic "topic-name-1"`

### base64
    
* encode base64

    `flow base64 encode --input "hello"`

* decode base64

    `flow base64 decode --input "aGVsbG8="`
    
### test
    
* load test http endpoint that runs for 180 seconds with 100 requests per second and will hit 2 urls

    ```sh
    flow test http \
     --url "https://test1.com \
     --url "https://test2.com \
     --frequency 100 \
     --duration "180s" \
     --authorization "Bearer TOKEN"
     ```

help:
```sh
flow --help
NAME:
   development tooling for AWS - A new cli application

USAGE:
   flow [global options] command [command options] [arguments...]

VERSION:
   0.1.0

COMMANDS:
     dynamodb        
     sqs             
     sns             
     cloudwatch      
     cloudwatchlogs  
     ssm             
     help, h         Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

or

```sh
flow dynamodb --help
NAME:
   development tooling for AWS dynamodb -

USAGE:
   development tooling for AWS dynamodb command [command options] [arguments...]

COMMANDS:
     purge     fast purge dynamodb using scan operation
     capacity  update read and write capacity

OPTIONS:
   --help, -h  show help
```

## authentication

Flow will be configured and will authenticate using the same methods defined by [aws-cli][auth].

Currently it supports authentication with:

* A [EnvProvider][EnvProvider] which retrieves credentials from the environment variables of the
  running process. Environment credentials never expire.
  Environment variables used:
  
  * Access Key ID:     AWS_ACCESS_KEY_ID or AWS_ACCESS_KEY

  * Secret Access Key: AWS_SECRET_ACCESS_KEY or AWS_SECRET_KEY
  
* A [SharedCredentialsProvider][SharedCredentialsProvider] which retrieves credentials from the current user's home
  directory, and keeps track if those credentials are expired.
  
  Profile ini file example: $HOME/.aws/credentials
  
* A AssumeRoleTokenProvider with enabled SharedConfigState which uses MFA prompting for token code on stdin.
  Go to [session doc][session] for more details.

[auth]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html
[envProvider]: https://docs.aws.amazon.com/sdk-for-go/api/aws/credentials/#EnvProvider
[sharedCredentialsProvider]: https://docs.aws.amazon.com/sdk-for-go/api/aws/credentials/#SharedCredentialsProvider
[session]: https://docs.aws.amazon.com/sdk-for-go/api/aws/session/
