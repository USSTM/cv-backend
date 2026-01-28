package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
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
}

func NewTestLocalStack(t *testing.T) *TestLocalStack {
	ctx := context.Background()

	container, err := localstack.Run(ctx,
		"localstack/localstack:3.0",
		testcontainers.WithReuseByName("cv-backend-test-localstack"),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Env: map[string]string{
					"SERVICES": "ses",
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

	ls := &TestLocalStack{
		Container: container,
		Config:    cfg,
		SES:       sesClient,
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
