# Flow AWS tooling ![test](https://github.com/flow-lab/flow/workflows/test/badge.svg) ![build](https://github.com/flow-lab/flow/workflows/build/badge.svg) ![goreleaser](https://github.com/flow-lab/flow/workflows/goreleaser/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/flow-lab/flow)](https://goreportcard.com/report/github.com/flow-lab/flow)

Flow is a CLI tooling that helps developing applications on AWS. Is meant to be used only in dev environments
and not in production.

NOTE: Flow CLI is under development, and may occasionally make backwards-incompatible changes.

## Requirements

- Go 1.26+

## Installation

* [homebrew](https://github.com/flow-lab/homebrew-tap)

    ```sh
    brew install flow-lab/tap/flow
    ```

* latest version from github releases

    ```sh
    curl --fail https://raw.githubusercontent.com/flow-lab/flow/master/bin/get-flow.sh --output get-flow.sh
    chmod +x get-flow.sh
    ./get-flow.sh
    ```

## Authentication

### AWS

Flow uses the standard AWS SDK credential chain with [shared config][session] enabled. Credentials are resolved in this order:

1. **Environment variables** — `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
2. **Shared credentials file** — `~/.aws/credentials` (supports profiles via `--profile`)
3. **Shared config file** — `~/.aws/config` (role assumption, MFA, SSO)
4. **Container credentials** — `AWS_CONTAINER_CREDENTIALS_FULL_URI`
5. **EC2/ECS instance roles**

When credentials are provided via environment variables (e.g. by `aws-vault`), the `--profile` flag is ignored and env credentials take precedence.

### aws-vault

Flow works with [aws-vault](https://github.com/99designs/aws-vault) out of the box:

```sh
aws-vault exec my-profile -- flow dynamodb describe-table --table-name TestTable
```

Both credential injection modes are supported:
- **Environment variables** (default) — aws-vault sets `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN`
- **Credential server** — aws-vault sets `AWS_CONTAINER_CREDENTIALS_FULL_URI`

### GCP

For Google Cloud Platform (GCP) commands set the _PUBSUB_EMULATOR_HOST_ environment variable. Set also the _PUBSUB_PROJECT_ID_
environment variable for the project you want to use for the emulator. For example:

```sh
export PUBSUB_EMULATOR_HOST=localhost:8432
export PUBSUB_PROJECT_ID=my-project-id
```

## Commands

### dynamodb

* delete all items from DynamoDB "TestTable" (using scan operation)

    `flow dynamodb delete --table-name TestTable`

* delete all items from DynamoDB "TestTable" with amount > 100

    `flow dynamodb delete --table-name TestTable --filter-expression "amount > :amount" --expression-attribute-values '{":amount":{"N":"100"}}'`

* change table capacity for **Provisioned** capacity mode

    `flow dynamodb capacity --table-name TestTable --write 10 --read 10`

* describe table

    `flow dynamodb describe-table --table-name test`

* count items using scan operation

    `flow dynamodb count-item --table-name TestTable`

* put item(s) from file

    `flow dynamodb put-item --input input.json --table-name TestTable`

    where, file input.json contains a list of json objects in dynamodb item format:
    ```json
    [
        {
          "id": { "S": "1" },
          "val": { "S": "1" }
        }
    ]
    ```

* delete item(s) from file

    `flow dynamodb delete-item --input input.json --table-name TestTable`

* search for records using scan operation

    `flow dynamodb search --table-name TestTable --filter-expression "amount > :amount" --expression-attribute-values '{":amount":{"N":"100"}}'`

* search and write results to file

    `flow dynamodb search --table-name TestTable --file-name output.json`

* map GSI keys to primary keys using Query

    `flow dynamodb map-to-primary-key --table-name TestTable --keys keys.json --secondary-index my-gsi`

* restore table to point in time

    `flow dynamodb restore-table-to-point-in-time --source-table-name TestTable --target-table-name TestTable-restored`

* delete backups older than 30 days

    `flow dynamodb delete-backup --table-name TestTable --older-than 30`

### s3

* purge all items and its versions from the bucket, items with delete markers will also be removed

    `flow s3 purge --bucket-name "test-bucket"`

* purge only items matching filter

    `flow s3 purge --bucket-name "test-bucket" --filter "prefix/"`

### sqs

* send message to the queue

    `flow sqs send --queue-name apud --input '{"id":"1","status":"ACTIVE"}' --message-attributes '{"eventType":{"DataType":"String","StringValue":"STATUS_UPDATED"}}'`

* send message from file

    `flow sqs send --queue-name apud --input-file-name message.json`

* receive messages from the queue

    `flow sqs receive-message --queue-name apud`

* receive messages with custom duration

    `flow sqs receive-message --queue-name apud --duration 30`

* purge all messages

    `flow sqs purge --queue-name apud`

* delete message by receipt handle

    `flow sqs delete-message --queue-name test --receipt-handle "receipt-handle-id"`

* describe sqs queue

    `flow sqs describe --queue-name apud`

### sns

* publish message to a topic

    `flow sns publish --topic-name my-topic --message '{"key":"value"}'`

* publish message multiple times with delay

    `flow sns publish --topic-name my-topic --message '{"key":"value"}' --times 10 --delay 100`

### cloudwatch

* delete cloudwatch alarm(s)

    `flow cloudwatch delete-alarm --name "my-alarm-1" --name "my-alarm-2"`

### cloudwatchlogs

* set log group retention in days

    `flow cloudwatchlogs retention --log-group-name "/aws/lambda/my-function" --days 30`

* describe log groups

    `flow cloudwatchlogs describe --log-group-name-prefix "/aws/lambda/"`

* summary of log groups (storage usage)

    `flow cloudwatchlogs summary --log-group-name-prefix "/aws/lambda/"`

* write log events to file

    `flow cloudwatchlogs write-to-file --log-group-name "/aws/lambda/my-function" --file-name output.json`

* get log events and export to CSV

    `flow cloudwatchlogs get-log-events --log-group-name-prefix "aws-waf-logs-" --hours 24`

* delete subscription filter

    `flow cloudwatchlogs delete-subscription-filter --log-group-name "/aws/lambda/my-function" --filter-name "my-filter"`

* delete all subscription filters

    `flow cloudwatchlogs delete-all-subscription-filters --log-group-name "/aws/lambda/my-function"`

### ssm

* export all ssm parameters and their values to json file

    `flow ssm export`

* export to custom file name

    `flow ssm export --output-file-name my-params.json`

### secretsmanager

* get secret value

    `flow secretsmanager get-secret-value --secret-id "my-secret"`

* export all secrets and their values to json

    `flow secretsmanager export`

* create secrets from file

    `flow secretsmanager create-secrets --input-file-name secrets.json`

* update secrets from file

    `flow secretsmanager update --input-file-name secrets.json`

* delete all secrets

    `flow secretsmanager delete-all`

* restore all secrets from file

    `flow secretsmanager restore-all --input-file-name secrets.json`

### kinesis

* update shard count

    `flow kinesis update-shard-count --stream-name "my-stream" --count 2`

### apigateway

* export all API specifications in swagger or oas3 specification and saves to file(s)

    `flow apigateway export`

### kafka (MSK) - Amazon Managed Streaming for Apache Kafka

* list MSK clusters in the account

    `flow kafka list-clusters`

* describe MSK cluster

    `flow kafka describe-cluster --cluster-name "MSK-Dev"`

* get bootstrap brokers

    `flow kafka get-bootstrap-brokers --cluster-name "MSK-Dev"`

* get broker info

    `flow kafka broker-info --bootstrap-broker localhost:9092`

* send message to topic

    `flow kafka send --cluster-name "MSK-Dev" --topic "topic-name" --message "test"`

* pipe messages from source to destination topic

    `flow kafka pipe --sbb localhost:9092 --st src-topic --dbb localhost:9092 --dt dst-topic`

* create topic

    `flow kafka create-topic --cluster-name "MSK-Dev" --topic "topic-name" --num-partitions 1 --replication-factor 1 --retention-ms "-1"`

* delete topic

    `flow kafka delete-topic --cluster-name "MSK-Dev" --topic "topic-name"`

* describe topic

    `flow kafka describe-topic --cluster-name "MSK-Dev" --topic "topic-name-0" --topic "topic-name-1"`

### sts

* assume role and generate env variables

    `flow sts assume-role --role-arn "arn:aws:iam::111111111111:role/my-role" --serial-number "arn:aws:iam::123456789:mfa/terraform" --token-code "123456"`
    output:
    ```
    AWS_REGION=eu-west-1
    AWS_ACCESS_KEY_ID=A..B
    AWS_SECRET_ACCESS_KEY=1..F
    AWS_SESSION_TOKEN=F..g
    ```

* get session token

    `flow sts get-session-token --serial-number "arn:aws:iam::123456789:mfa/terraform" --token-code "123456"`

* unset envs

    `flow sts clean`
    output:
    ```
    unset AWS_REGION
    unset AWS_ACCESS_KEY_ID
    unset AWS_SECRET_ACCESS_KEY
    unset AWS_SESSION_TOKEN
    ```

### cloudtrail

* find cloud trail events between two dates

    `flow cloudtrail find --start-time="2020-03-12T17:00:00Z" --end-time="2020-03-12T20:00:00Z"`

* find events containing specific text

    `flow cloudtrail find --contains "unauthorized" --contains "forbidden" --start-time="2020-03-12T17:00:00Z" --end-time="2020-03-12T20:00:00Z"`

### eks

* open EKS Kubernetes dashboard and start kubectl proxy

    `flow eks dashboard --cluster cluster-0`
    output:
    ```
    token: e..g

    running command: /usr/local/bin/aws eks update-kubeconfig --name cluster-0
    running command: /usr/local/bin/kubectl proxy
    open in browser(use token from above):  http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/#!/login
    ```

### pubsub

These commands are designed to use with [Flow Pub/Sub emulator](https://github.com/flow-lab/flow-pubsub) running on Kubernetes.

* create topic

    `flow pubsub create-topic -t user`

* create subscription

    `flow pubsub create-subscription -t user --sub user`

* publish file content with attributes

    `cat message.json | flow pubsub publish -t user -attr "{\"event-type\":\"user-created\"}"`

### base64

* encode base64

    `flow base64 encode --input "hello"`

* encode base64 file

    `flow base64 encode --file input.json`

* decode base64

    `flow base64 decode --input "aGVsbG8="`

### test

* load test http endpoint

    ```sh
    flow test http \
     --url "https://test1.com" \
     --url "https://test2.com" \
     --frequency 100 \
     --duration "180s" \
     --authorization "Bearer TOKEN"
    ```

### github

* get tag for GitHub SHA

    ```sh
    flow github get-tag --owner flow-lab --repo auxospore --sha d3d8a8803f6ecb4b091667ba61b9945da7af3cf2
    ```

### misc

* split a CSV file into multiple files

    `flow misc chunk-csv --input data.csv --chunk-size 1000`

* convert JSON array to JSONL (one object per line, useful for BigQuery imports)

    `flow misc to-jsonl --input data.json --output data.jsonl`

### crypto

* generate RSA key pair and self-signed x509 certificate

    `flow crypto genrsa`

## Help

```
NAME:
   flow - Development CLI

USAGE:
   flow [global options] command [command options]

VERSION:
   dev

COMMANDS:
   dynamodb        AWS DynamoDB
   sqs             AWS SQS
   sns             AWS SNS
   cloudwatch      AWS CloudWatch
   cloudwatchlogs  AWS CloudWatch Logs
   ssm             AWS SSM
   secretsmanager  AWS SecretsManager
   kinesis         AWS Kinesis
   base64          encoding/decoding base64
   s3              AWS S3
   apigateway      AWS API Gateway
   test            HTTP load testing commands
   kafka           AWS MSK
   sts             AWS STS
   pubsub          GCP Pub/Sub for testing locally with emulator
   cloudtrail      AWS CloudTrail
   eks             AWS EKS
   github          GitHub helpers
   misc            Miscellaneous helpers
   crypto          Encryption helpers. Generate keys
   help, h         Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

[session]: https://docs.aws.amazon.com/sdk-for-go/api/aws/session/
