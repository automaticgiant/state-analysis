package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("Loading .env file...")
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	bucket := os.Getenv("S3_BUCKET")
	key := os.Getenv("S3_KEY")
	outputDir := os.Getenv("OUTPUT_DIR")
	awsProfile := os.Getenv("AWS_PROFILE")
	awsRegion := os.Getenv("AWS_REGION")

	fmt.Printf("S3_BUCKET: %s\n", bucket)
	fmt.Printf("S3_KEY: %s\n", key)
	fmt.Printf("OUTPUT_DIR: %s\n", outputDir)
	fmt.Printf("AWS_PROFILE: %s\n", awsProfile)
	fmt.Printf("AWS_REGION: %s\n", awsRegion)

	if bucket == "" || outputDir == "" || awsProfile == "" || awsRegion == "" {
		log.Fatalf("S3_BUCKET, OUTPUT_DIR, AWS_PROFILE, and AWS_REGION must be set in .env file")
	}

	// Set AWS profile and region
	os.Setenv("AWS_PROFILE", awsProfile)
	os.Setenv("AWS_REGION", awsRegion)

	fmt.Println("Creating AWS session...")
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	// Verify credentials with sts.GetCallerIdentity
	fmt.Println("Verifying credentials with sts.GetCallerIdentity...")
	stsSvc := sts.New(sess)
	identity, err := stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatalf("Failed to verify credentials: %v", err)
	}
	fmt.Printf("Verified credentials for AWS account: %s, user: %s\n", *identity.Account, *identity.UserId)

	svc := s3.New(sess)

	var keys []string
	if key == "" {
		// List all objects in the bucket
		fmt.Println("Listing all objects in the bucket...")
		listObjectsInput := &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
		}

		err = svc.ListObjectsV2Pages(listObjectsInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				if strings.HasSuffix(*obj.Key, "state") {
					keys = append(keys, *obj.Key)
				}
			}
			return !lastPage
		})
		if err != nil {
			log.Fatalf("Failed to list objects: %v", err)
		}
	} else {
		keys = append(keys, key)
	}

	for _, key := range keys {
		// Create subdirectory based on the S3 key
		subDir := filepath.Join(outputDir, filepath.Base(key))
		if err := os.MkdirAll(subDir, 0755); err != nil {
			log.Printf("Failed to create subdirectory %s: %v", subDir, err)
			continue
		}

		// List object versions
		fmt.Printf("Listing object versions for key: %s...\n", key)
		listObjectVersionsInput := &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucket),
			Prefix: aws.String(key),
		}

		versions, err := svc.ListObjectVersions(listObjectVersionsInput)
		if err != nil {
			log.Printf("Failed to list object versions for key %s: %v", key, err)
			continue
		}

		for _, version := range versions.Versions {
			versionID := aws.StringValue(version.VersionId)
			lastModified := aws.TimeValue(version.LastModified).Format("20060102T150405Z")
			fmt.Printf("Downloading version %s of key %s...\n", versionID, key)
			getObjectInput := &s3.GetObjectInput{
				Bucket:    aws.String(bucket),
				Key:       aws.String(key),
				VersionId: aws.String(versionID),
			}

			result, err := svc.GetObject(getObjectInput)
			if err != nil {
				log.Printf("Failed to get object version %s of key %s: %v", versionID, key, err)
				continue
			}

			outputFilePath := filepath.Join(subDir, fmt.Sprintf("%s-%s.tfstate", versionID, lastModified))
			outputFile, err := os.Create(outputFilePath)
			if err != nil {
				log.Printf("Failed to create file %s: %v", outputFilePath, err)
				continue
			}

			_, err = outputFile.ReadFrom(result.Body)
			if err != nil {
				log.Printf("Failed to write to file %s: %v", outputFilePath, err)
				outputFile.Close()
				continue
			}

			outputFile.Close()
			fmt.Printf("Downloaded version %s of key %s to %s\n", versionID, key, outputFilePath)
		}
	}
}
