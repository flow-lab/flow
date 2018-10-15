package main

import (
	"fmt"
	"github.com/urfave/cli"
	"log"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "development tooling for AWS"
	app.Version = "0.1.30"

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
