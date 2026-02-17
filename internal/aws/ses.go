package aws

import (
	"context"
	"fmt"

	"github.com/USSTM/cv-backend/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type EmailService struct {
	client *ses.Client
	sender string
}

func NewEmailService(cfg config.AWSConfig) (*EmailService, error) {
	awsCfg, err := LoadAWSConfig(cfg)
	if err != nil {
		return nil, err
	}

	// create SES client, overriding endpoint if provided (for LocalStack)
	client := ses.NewFromConfig(awsCfg, func(o *ses.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		}
	})

	// hardcoded sender for localstack (the sender doesn't matter, we change this later for actual SES)
	return &EmailService{
		client: client,
		sender: cfg.Sender,
	}, nil
}

func (s *EmailService) SendEmail(ctx context.Context, to string, subject string, body string) error {
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

func (s *EmailService) VerifyEmailIdentity(ctx context.Context) (*ses.VerifyEmailIdentityOutput, error) {
	output, err := s.client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(s.sender),
	})

	return output, err
}

// for printing/debugging
func (s *EmailService) Sender() string {
	return s.sender
}
