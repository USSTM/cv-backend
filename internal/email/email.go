package email

import (
	"context"
	"fmt"

	"github.com/USSTM/cv-backend/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type SESService struct {
	client *ses.Client
	sender string
}

func NewSESService(ctx context.Context, cfg config.AWSConfig) (*SESService, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     cfg.AccessKeyID,
				SecretAccessKey: cfg.SecretAccessKey,
			}, nil
		})))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// create SES client, overriding endpoint if provided (for LocalStack)
	client := ses.NewFromConfig(awsCfg, func(o *ses.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		}
	})

	// hardcoded sender for localstack (the sender doesn't matter, we change this later for actual SES)
	return &SESService{
		client: client,
		sender: "test@example.com",
	}, nil
}

func (s *SESService) SendEmail(ctx context.Context, to string, subject string, body string) error {
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
		Source: aws.String(s.sender),
	}

	_, err := s.client.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}

func (s *SESService) Client() *ses.Client {
	return s.client
}

func (s *SESService) Sender() string {
	return s.sender
}
