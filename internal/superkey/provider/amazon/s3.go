package amazon

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// CreateS3Bucket creates an S3 bucket with the given name
func (c *Client) CreateS3Bucket(ctx context.Context, name string) error {
	_, err := c.S3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &name,
	})
	return err
}

// DestroyS3Bucket destroys an S3 bucket, first clearing any objects
func (c *Client) DestroyS3Bucket(ctx context.Context, name string) error {
	// First, list and delete all objects in the bucket
	objects, err := c.S3.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket: &name,
	})
	if err != nil {
		return err
	}

	// Delete each object
	for _, object := range objects.Contents {
		_, err = c.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: &name,
			Key:    object.Key,
		})
		if err != nil {
			return err
		}
	}

	// Now delete the bucket
	_, err = c.S3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: &name,
	})
	return err
}

// AttachBucketPolicy attaches a policy to an S3 bucket
func (c *Client) AttachBucketPolicy(ctx context.Context, bucket, policy string) error {
	_, err := c.S3.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: &bucket,
		Policy: &policy,
	})
	return err
}
