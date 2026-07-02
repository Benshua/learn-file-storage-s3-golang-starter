package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)

	presignExpiry := s3.WithPresignExpires(expireTime)

	objInput := s3.GetObjectInput {
		Bucket: &bucket,
		Key: &key,
	}
	
	req, err := presignClient.PresignGetObject(context.TODO(), &objInput, presignExpiry)
	if err != nil {
		return "", fmt.Errorf("Error getting presign object: %v", err)
	}

	return req.URL, nil
}