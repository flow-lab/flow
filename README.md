# Flow AWS tooling

[![Snap Status](https://build.snapcraft.io/badge/flow-lab/flow.svg)](https://build.snapcraft.io/user/flow-lab/flow)

Set of tooling commands for AWS development

For installation with homebrew go to https://github.com/flow-lab/homebrew-tap

example usage:
```bash
flow sqs describe --queue-name hello-in --profile dev@flowlab-dev
```

help:
```bash
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

```bash
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

### for local installation:
```bash
go get -u github.com/flow-lab/flow
cd $GOPATH/src/github.com/flow-lab/flow
go install
```
