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
						var objectIdentifiers []*s3.ObjectIdentifier
						for _, version := range output.Versions {
							objectIdentifiers = append(objectIdentifiers, &s3.ObjectIdentifier{
								Key:       version.Key,
								VersionId: version.VersionId,
							})
						}

						for _, deleteMarker := range output.DeleteMarkers {
							objectIdentifiers = append(objectIdentifiers, &s3.ObjectIdentifier{
								Key:       deleteMarker.Key,
								VersionId: deleteMarker.VersionId,
							})
						}

						if len(objectIdentifiers) > 0 {
							deleteObjectsInput := s3.DeleteObjectsInput{
								Bucket: output.Name,
								Delete: &s3.Delete{
									Objects: objectIdentifiers,
								},
							}
							_, err := s3c.DeleteObjects(&deleteObjectsInput)
							if err != nil {
								panic(err)
							}
							fmt.Printf("deleted: %v\n", objectIdentifiers)
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
