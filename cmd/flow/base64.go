package main

import (
	"fmt"
	"github.com/flow-lab/flow/pkg/base64"
	"github.com/urfave/cli"
)

var Base64Command = func() cli.Command {
	return cli.Command{
		Name:        "base64",
		Description: "encoding/decoding base64",
		Subcommands: []cli.Command{
			{
				Name:  "encode",
				Usage: "encodes string to base64",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "input",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					input := c.String("input")
					if input == "" {
						return fmt.Errorf("missing --input")
					}

					encode := base64.Encode(input)
					fmt.Println(string(encode))

					return nil
				},
			},
			{
				Name:  "decode",
				Usage: "decodes base64 encoded string",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "input",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					input := c.String("input")
					if input == "" {
						return fmt.Errorf("missing --input")
					}

					bytes, err := base64.Decode(input)
					if err != nil {
						return fmt.Errorf("call to Decode failed: %s", err)
					}
					fmt.Println(string(bytes))

					return nil
				},
			},
		},
	}
}
