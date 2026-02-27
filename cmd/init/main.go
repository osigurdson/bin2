package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"bin2.io/internal/db"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var destructive bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Postgres database and run migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context(), destructive)
		},
	}
	cmd.Flags().BoolVar(&destructive, "destructive", false, "drop and recreate database before migrations")

	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Initialize Postgres database and run migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context(), destructive)
		},
	}
	migrateCmd.Flags().BoolVar(&destructive, "destructive", false, "drop and recreate database before migrations")

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Delete all data from the configured R2 bucket",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runR2Clean(cmd.Context())
		},
	}

	cmd.AddCommand(migrateCmd, cleanCmd)
	return cmd
}

func runInit(ctx context.Context, destructive bool) error {
	cfg, err := db.NewConfigFromEnv()
	if err != nil {
		return fmt.Errorf("could not read postgres configuration: %w", err)
	}

	exists, err := db.AdminCheckIfDBExists(ctx, cfg)
	if err != nil {
		return fmt.Errorf("could not check database existence: %w", err)
	}

	if exists && destructive {
		log.Printf("database %s exists; dropping and recreating", cfg.Database)
		if err := db.AdminCreateDB(ctx, cfg, true); err != nil {
			return fmt.Errorf("could not recreate database: %w", err)
		}
	} else if !exists {
		log.Printf("database %s does not exist; creating", cfg.Database)
		if err := db.AdminCreateDB(ctx, cfg, false); err != nil {
			return fmt.Errorf("could not create database: %w", err)
		}
	} else {
		log.Printf("database %s already exists; skipping create", cfg.Database)
	}

	log.Println("running migrations")
	if err := db.RunMigrations(ctx, cfg); err != nil {
		return fmt.Errorf("could not run migrations: %w", err)
	}
	log.Println("database initialization complete")
	return nil
}

func runR2Clean(ctx context.Context) error {
	bucket := strings.TrimSpace(os.Getenv("R2_BUCKET"))
	if bucket == "" {
		return fmt.Errorf("R2_BUCKET must be set")
	}

	endpoint, err := r2EndpointFromEnv()
	if err != nil {
		return err
	}

	accessKeyID := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
	secretAccessKey := strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
	if accessKeyID == "" || secretAccessKey == "" {
		return fmt.Errorf("R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY must be set")
	}

	region := strings.TrimSpace(os.Getenv("R2_REGION"))
	if region == "" {
		region = "auto"
	}

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		),
	)
	if err != nil {
		return fmt.Errorf("could not load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	var totalDeleted int
	var continuationToken *string

	for {
		listOut, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return fmt.Errorf("could not list objects: %w", err)
		}

		objects := make([]types.ObjectIdentifier, 0, len(listOut.Contents))
		for _, obj := range listOut.Contents {
			if obj.Key == nil {
				continue
			}
			objects = append(objects, types.ObjectIdentifier{Key: obj.Key})
		}

		if len(objects) > 0 {
			delOut, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucket),
				Delete: &types.Delete{
					Objects: objects,
					Quiet:   aws.Bool(true),
				},
			})
			if err != nil {
				return fmt.Errorf("could not delete objects: %w", err)
			}
			if len(delOut.Errors) > 0 {
				first := delOut.Errors[0]
				return fmt.Errorf("delete failed for key %q: %s", aws.ToString(first.Key), aws.ToString(first.Message))
			}
			totalDeleted += len(objects)
		}

		if listOut.IsTruncated != nil && *listOut.IsTruncated {
			continuationToken = listOut.NextContinuationToken
			continue
		}
		break
	}

	log.Printf("deleted %d objects from bucket %s", totalDeleted, bucket)
	return nil
}

func r2EndpointFromEnv() (string, error) {
	endpoint := strings.TrimSpace(os.Getenv("R2_ENDPOINT"))
	if endpoint != "" {
		return endpoint, nil
	}

	accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	if accountID == "" {
		return "", fmt.Errorf("R2_ACCOUNT_ID or R2_ENDPOINT must be set")
	}
	return fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID), nil
}
