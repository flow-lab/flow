# Flow AWS tooling
Set of tooling commands for AWS development

installation:
```bash
go get -u github.com/flow-lab/flow
cd $GOPATH/src/github.com/flow-lab/flow
go install
```

example usage:
```bash
flow sqs describe --queue-name hello-in --profile dev@flowlab-dev
```