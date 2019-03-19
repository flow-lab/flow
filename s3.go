package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/urfave/cli"
)

var s3Command = func() cli.Command {
	return cli.Command{
		Name: "s3",
		Subcommands: []cli.Command{
			{
				Name:  "purge",
				Usage: "delete all objects and it versions",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "bucket-name",
						Value: "",
					},
					cli.StringFlag{
						Name:  "profile",
						Value: "",
					},
				},
				Action: func(c *cli.Context) error {
					profile := c.String("profile")
					bucketName := c.String("bucket-name")

					if bucketName == "" {
						return fmt.Errorf("missing --bucket-name parameter")
					}

					sess := NewSessionWithSharedProfile(profile)
					s3c := s3.New(sess)

					input := s3.ListObjectVersionsInput{
						Bucket: aws.String(bucketName),
					}
					err := s3c.ListObjectVersionsPages(&input, func(output *s3.ListObjectVersionsOutput, b bool) bool {
						for _, version := range output.Versions {
							deleteObjectInput := s3.DeleteObjectInput{
								Bucket: output.Name,
								Key:    version.Key,
							}
							_, err := s3c.DeleteObject(&deleteObjectInput)
							if err != nil {
								panic(err)
							}
							o := fmt.Sprintf("%s/%s\n", *output.Name, *version.Key)
							fmt.Printf("deleted key: %v", o)
						}

						return output.NextKeyMarker != nil
					})
					if err != nil {
						return err
					}

					return nil
				},
			},
		},
	}
}
