package testutil

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestLocalStack struct {
	Container *localstack.LocalStackContainer
	Config    aws.Config
	SES       *ses.Client
	S3        *s3.Client
}

func NewTestLocalStack(t *testing.T) *TestLocalStack {
	ctx := context.Background()

	container, err := localstack.Run(ctx,
		"localstack/localstack:3.0",
		testcontainers.WithReuseByName("cv-backend-test-localstack"),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Env: map[string]string{
					"SERVICES": "ses,s3",
				},
			},
		}),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("Ready.").
					WithOccurrence(1).
					WithStartupTimeout(60*time.Second),
				wait.ForListeningPort("4566/tcp").
					WithStartupTimeout(60*time.Second),
			),
		),
	)
	require.NoError(t, err, "Failed to start LocalStack container")

	endpoint, err := container.PortEndpoint(ctx, "4566/tcp", "")
	require.NoError(t, err, "Failed to get LocalStack endpoint")

	credentialsProvider := aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
		return aws.Credentials{
			AccessKeyID:     "test",
			SecretAccessKey: "test",
			SessionToken:    "test",
			Source:          "HardcodedCredentials",
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentialsProvider),
	)
	require.NoError(t, err, "Failed to load AWS config")

	sesClient := ses.NewFromConfig(cfg, func(o *ses.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	ls := &TestLocalStack{
		Container: container,
		Config:    cfg,
		SES:       sesClient,
		S3:        s3Client,
	}

	t.Cleanup(func() {
		ls.Close()
	})

	return ls
}

func (ls *TestLocalStack) Close() {
	if ls.Container != nil {
		ls.Container.Terminate(context.Background())
	}
}

func (ls *TestLocalStack) Cleanup(t *testing.T) {
	ctx := context.Background()

	listOut, err := ls.SES.ListIdentities(ctx, &ses.ListIdentitiesInput{})
	if err != nil {
		t.Logf("Failed to list identities: %v", err)
		return
	}

	for _, identity := range listOut.Identities {
		_, err := ls.SES.DeleteIdentity(ctx, &ses.DeleteIdentityInput{
			Identity: &identity,
		})
		if err != nil {
			t.Logf("Failed to delete identity %s: %v", identity, err)
		}
	}

	ls.S3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String("cv-backend-test-bucket"),
	})
}

func (ls *TestLocalStack) SendEmail(ctx context.Context, to, subject, body string) error {
	input := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Message: &types.Message{
			Body: &types.Body{
				Text: &types.Content{
					Data: aws.String(body),
				},
			},
			Subject: &types.Content{
				Data: aws.String(subject),
			},
		},
		Source: aws.String("test@example.com"),
	}

	_, err := ls.SES.SendEmail(ctx, input)
	return err
}

func (ls *TestLocalStack) PutObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	_, err := ls.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String("cv-backend-test-bucket"),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

func (ls *TestLocalStack) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := ls.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("cv-backend-test-bucket"),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func (ls *TestLocalStack) GeneratePresignedURL(ctx context.Context, method string, key string, duration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(ls.S3)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("cv-backend-test-bucket"),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(duration))

	if err != nil {
		return "", err
	}
	return req.URL, nil
}
