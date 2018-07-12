# Flow AWS tooling

Set of tooling commands for AWS development

For installation with homebrew go to https://github.com/flow-lab/homebrew-tap

example usage:
```bash
flow sqs describe --queue-name hello-in --profile dev@flowlab-dev
```

help:
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
