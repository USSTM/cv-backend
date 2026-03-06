package aws

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/USSTM/cv-backend/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Service struct {
	client            *s3.Client
	bucket            string
	publicEndpointURL string
}

func NewS3Service(cfg config.AWSConfig) (*S3Service, error) {
	awsCfg, err := LoadAWSConfig(cfg)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
			o.UsePathStyle = true // required for localstack
		}
	})

	return &S3Service{
		client:            client,
		bucket:            cfg.Bucket,
		publicEndpointURL: cfg.PublicEndpointURL,
	}, nil
}

func (s *S3Service) PutObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}
	return nil
}

func (s *S3Service) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file from S3: %w", err)
	}

	return output.Body, nil
}

func (s *S3Service) GeneratePresignedURL(ctx context.Context, method string, key string, duration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	var req *v4.PresignedHTTPRequest
	var err error

	switch method {
	case http.MethodPut:
		req, err = presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(duration))
	case http.MethodGet:
		req, err = presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		}, s3.WithPresignExpires(duration))
	default:
		return "", fmt.Errorf("unsupported method: %s", method)
	}

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	presignedURL := req.URL
	if s.publicEndpointURL != "" {
		presignedURL, err = rewriteHost(presignedURL, s.publicEndpointURL)
		if err != nil {
			return "", fmt.Errorf("failed to rewrite presigned URL host: %w", err)
		}
	}

	return presignedURL, nil
}

// in dev: rewrites "localstack" in urls to "localhost" so that you can access URL outside of docker network (which user by default is not in)
func rewriteHost(presignedURL, publicBase string) (string, error) {
	parsed, err := url.Parse(presignedURL)
	if err != nil {
		return "", err
	}
	pub, err := url.Parse(publicBase)
	if err != nil {
		return "", err
	}
	parsed.Scheme = pub.Scheme
	parsed.Host = pub.Host
	return parsed.String(), nil
}

func (s *S3Service) CreateBucket(ctx context.Context) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *S3Service) ListBuckets(ctx context.Context) ([]types.Bucket, error) {
	output, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	return output.Buckets, nil
}

func (s *S3Service) ListObjects(ctx context.Context) ([]types.Object, error) {
	output, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	return output.Contents, nil
}

func (s *S3Service) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}
	return nil
}
