package tos_tool

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

// ListFilesWithEnvConfig lists files in TOS bucket with a specific prefix using environment variables for configuration
func ListFilesWithEnvConfig(prefix string) ([]map[string]interface{}, error) {
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
		return nil, fmt.Errorf("TOS_ACCESS_KEY environment variable is required")
	}
	if config.SecretKey == "" {
		return nil, fmt.Errorf("TOS_SECRET_KEY environment variable is required")
	}
	if config.BucketName == "" {
		return nil, fmt.Errorf("TOS_BUCKET_NAME environment variable is required")
	}

	return ListFiles(config, prefix)
}

// ListFiles lists files in TOS bucket with a specific prefix
func ListFiles(config UploadConfig, prefix string) ([]map[string]interface{}, error) {
	ctx := context.Background()

	// Initialize TOS client
	client, err := tos.NewClientV2(config.Endpoint,
		tos.WithRegion(config.Region),
		tos.WithCredentials(tos.NewStaticCredentials(config.AccessKey, config.SecretKey)))
	if err != nil {
		return nil, fmt.Errorf("failed to create TOS client: %w", err)
	}

	var files []map[string]interface{}
	var continuationToken string

	for {
		// List objects with prefix
		input := &tos.ListObjectsType2Input{
			Bucket:            config.BucketName,
			Prefix:            prefix,
			ContinuationToken: continuationToken,
			MaxKeys:           1000, // Maximum number of keys to return
		}

		output, err := client.ListObjectsType2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Process each object
		for _, object := range output.Contents {
			// Skip the prefix directory itself
			if object.Key == prefix {
				continue
			}

			// Extract filename from the full path
			filename := object.Key
			if prefix != "" {
				// Remove the prefix from the key to get just the filename
				if len(object.Key) > len(prefix) {
					filename = object.Key[len(prefix):]
					// Remove leading slash if present
					if len(filename) > 0 && filename[0] == '/' {
						filename = filename[1:]
					}
				}
			}

			// Generate pre-signed URL for the object
			preSignedURL, err := client.PreSignedURL(&tos.PreSignedURLInput{
				HTTPMethod: enum.HttpMethodGet,
				Bucket:     config.BucketName,
				Key:        object.Key,
			})
			if err != nil {
				fmt.Printf("Warning: failed to generate pre-signed URL for %s: %v\n", object.Key, err)
				continue
			}

			files = append(files, map[string]interface{}{
				"name":    filename,
				"size":    object.Size,
				"modTime": object.LastModified.Format("2006-01-02T15:04:05Z07:00"),
				"key":     object.Key,
				"url":     preSignedURL.SignedUrl,
			})
		}

		// Check if there are more objects to fetch
		if !output.IsTruncated {
			break
		}
		continuationToken = output.NextContinuationToken
	}

	return files, nil
}

// DeleteFile deletes a file from TOS
func DeleteFile(config UploadConfig, objectKey string) error {
	ctx := context.Background()

	// Initialize TOS client
	client, err := tos.NewClientV2(config.Endpoint,
		tos.WithRegion(config.Region),
		tos.WithCredentials(tos.NewStaticCredentials(config.AccessKey, config.SecretKey)))
	if err != nil {
		return fmt.Errorf("failed to create TOS client: %w", err)
	}

	// Delete object from TOS
	_, err = client.DeleteObjectV2(ctx, &tos.DeleteObjectV2Input{
		Bucket: config.BucketName,
		Key:    objectKey,
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	fmt.Printf("File deleted successfully from TOS: %s\n", objectKey)
	return nil
}

// DeleteFileWithEnvConfig deletes a file from TOS using environment variables for configuration
func DeleteFileWithEnvConfig(objectKey string) error {
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
		return fmt.Errorf("TOS_ACCESS_KEY environment variable is required")
	}
	if config.SecretKey == "" {
		return fmt.Errorf("TOS_SECRET_KEY environment variable is required")
	}
	if config.BucketName == "" {
		return fmt.Errorf("TOS_BUCKET_NAME environment variable is required")
	}

	return DeleteFile(config, objectKey)
}

//func main() {
//	// Example usage with environment variables
//	preSignedURL, err := UploadFileWithEnvConfig("../README.md", "example_dir/README2.md")
//	checkErr(err)
//	fmt.Printf("Pre-signed URL: %s\n", preSignedURL)
//}
