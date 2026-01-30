package amazon

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// CreateS3Bucket creates an S3 bucket with the given name
func (a *Client) CreateS3Bucket(ctx context.Context, name string) error {
	_, err := a.S3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &name,
	})

	return err
}

// DestroyS3Bucket deletes an S3 bucket and all its contents
func (a *Client) DestroyS3Bucket(ctx context.Context, name string) error {
	// First clear any objects in the bucket
	objects, err := a.S3.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket: &name,
	})
	if err != nil {
		return err
	}

	for _, object := range objects.Contents {
		_, err = a.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: &name,
			Key:    object.Key,
		})
		if err != nil {
			return err
		}
	}

	_, err = a.S3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: &name,
	})

	return err
}

// AttachBucketPolicy attaches a policy to an S3 bucket
func (a *Client) AttachBucketPolicy(ctx context.Context, bucket, policy string) error {
	_, err := a.S3.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: &bucket,
		Policy: &policy,
	})

	return err
}
