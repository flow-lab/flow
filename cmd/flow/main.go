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
		dynamodbCommand(),
		sqsCommand(),
		snsCommand(),
		cloudwatchCommand(),
		cloudwatchlogsCommand(),
		ssmCommand(),
		secretsmanagerCommand(),
		kinesisCommand(),
		base64Command(),
		s3Command(),
		apiGateway(),
		kafkaCommand(),
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
