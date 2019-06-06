package main

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app := cli.NewApp()
	app.Name = fmt.Sprintf("development tooling for AWS. Commit: %s, release date: %s", commit, date)
	app.Version = version

	app.Commands = []cli.Command{
		DynamodbCommand(),
		SQSCommand(),
		SNSCommand(),
		CloudwatchCommand(),
		CloudwatchlogsCommand(),
		SSMCommand(),
		SecretsmanagerCommand(),
		KinesisCommand(),
		Base64Command(),
		S3Command(),
		APIGateway(),
		KafkaCommand(),
	}

	app.Action = func(c *cli.Context) error {
		fmt.Println("try: flow --help")
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
