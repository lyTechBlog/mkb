package main

import (
	"context"
	"fmt"
	"os"

	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
)

// checkErr handles error checking and provides detailed error information
func checkErr(err error) {
	if err != nil {
		if serverErr, ok := err.(*tos.TosServerError); ok {
			fmt.Println("Error:", serverErr.Error())
			fmt.Println("Request ID:", serverErr.RequestID)
			fmt.Println("Response Status Code:", serverErr.StatusCode)
			fmt.Println("Response Header:", serverErr.Header)
			fmt.Println("Response Err Code:", serverErr.Code)
			fmt.Println("Response Err Msg:", serverErr.Message)
		} else if clientErr, ok := err.(*tos.TosClientError); ok {
			fmt.Println("Error:", clientErr.Error())
			fmt.Println("Client Cause Err:", clientErr.Cause.Error())
		} else {
			fmt.Println("Error:", err)
		}
		panic(err)
	}
}

// UploadConfig holds the configuration for TOS upload
type UploadConfig struct {
	AccessKey  string
	SecretKey  string
	Endpoint   string
	Region     string
	BucketName string
}

// UploadFile uploads a file to TOS and returns a pre-signed URL for the uploaded object
func UploadFile(config UploadConfig, localFilePath, objectKey string) (string, error) {
	ctx := context.Background()

	// Initialize TOS client
	client, err := tos.NewClientV2(config.Endpoint,
		tos.WithRegion(config.Region),
		tos.WithCredentials(tos.NewStaticCredentials(config.AccessKey, config.SecretKey)))
	if err != nil {
		return "", fmt.Errorf("failed to create TOS client: %w", err)
	}

	// Open local file
	file, err := os.Open(localFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", localFilePath, err)
	}
	defer file.Close()

	// Upload file to TOS
	output, err := client.PutObjectV2(ctx, &tos.PutObjectV2Input{
		PutObjectBasicInput: tos.PutObjectBasicInput{
			Bucket: config.BucketName,
			Key:    objectKey,
		},
		Content: file,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	fmt.Printf("File uploaded successfully! Request ID: %s\n", output.RequestID)

	// Generate pre-signed URL for the uploaded object
	preSignedURL, err := client.PreSignedURL(&tos.PreSignedURLInput{
		HTTPMethod: enum.HttpMethodGet,
		Bucket:     config.BucketName,
		Key:        objectKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate pre-signed URL: %w", err)
	}

	return preSignedURL.SignedUrl, nil
}

// UploadFileWithEnvConfig uploads a file using environment variables for configuration and returns pre-signed URL
func UploadFileWithEnvConfig(localFilePath, objectKey string) (string, error) {
	Ak := "AKLTZmRkY2Q1N2ZkZDlhNDIxNWIzNzUzZmRiNzY5ZGYwM2M"
	Sk := "T1RkaU56WTBPR1k0WldRek5EVTJOV0UwTldNNVptWTBNVEU1WmpWaE5ETQ=="
	mkb_bucket := "mkb-test"
	config := UploadConfig{
		AccessKey:  Ak,
		SecretKey:  Sk,
		Endpoint:   "https://tos-cn-beijing.volces.com", // Default endpoint
		Region:     "cn-beijing",                        // Default region
		BucketName: mkb_bucket,
	}

	// Validate required environment variables
	if config.AccessKey == "" {
		return "", fmt.Errorf("TOS_ACCESS_KEY environment variable is required")
	}
	if config.SecretKey == "" {
		return "", fmt.Errorf("TOS_SECRET_KEY environment variable is required")
	}
	if config.BucketName == "" {
		return "", fmt.Errorf("TOS_BUCKET_NAME environment variable is required")
	}

	return UploadFile(config, localFilePath, objectKey)
}

func main() {
	// Example usage with environment variables
	preSignedURL, err := UploadFileWithEnvConfig("../README.md", "example_dir/README2.md")
	checkErr(err)
	fmt.Printf("Pre-signed URL: %s\n", preSignedURL)
}
