package main

import (
	"encoding/base64"
	"fmt"
	"github.com/urfave/cli"
)

var base64Command = func() cli.Command {
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

					dst := make([]byte, base64.StdEncoding.EncodedLen(len(input)))
					base64.StdEncoding.Encode(dst, []byte(input))
					fmt.Println(string(dst))
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

					dst := make([]byte, base64.StdEncoding.DecodedLen(len(input)))
					base64.StdEncoding.Decode(dst, []byte(input))
					fmt.Println(string(dst))
					return nil
				},
			},
		},
	}
}
