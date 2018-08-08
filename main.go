package main

import (
	"log"
	"os"
	"github.com/urfave/cli"
	"fmt"
)

func main() {
	app := cli.NewApp()
	app.Name = "development tooling for AWS"
	app.Version = "0.1.11"

	app.Commands = []cli.Command{
		dynamodbCommand(),
		sqsCommand(),
		snsCommand(),
		cloudwatchCommand(),
		cloudwatchlogsCommand(),
		ssmCommand(),
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
