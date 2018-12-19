# Flow AWS tooling [![Build Status](https://travis-ci.org/flow-lab/flow.svg?branch=master)](https://travis-ci.org/flow-lab/flow) [![Snap Status](https://build.snapcraft.io/badge/flow-lab/flow.svg)](https://build.snapcraft.io/user/flow-lab/flow)

Set of tooling commands for AWS development

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
    go install
    ```

## example usage:

```sh
flow dynamodb purge --table-name TestTable --key tableId --profile cloudformation@flowlab-dev
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
