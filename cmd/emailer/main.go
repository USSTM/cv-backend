package main

import (
	"context"
	"log"

	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/email"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
)

type LocalStackEmail struct {
	ID          string    `json:"Id"`
	Timestamp   string    `json:"Timestamp"`
	Subject     string    `json:"Subject"`
	Body        EmailBody `json:"Body"`
	Destination Dest      `json:"Destination"`
}
type EmailBody struct {
	Text string `json:"text_part"`
	HTML string `json:"html_part"`
}
type Dest struct {
	ToAddresses []string `json:"ToAddresses"`
}
type LocalStackResponse struct {
	Messages []LocalStackEmail `json:"messages"`
}

func main() {
	// load configuration
	cfg := config.Load()

	// initialize email service
	ctx := context.Background()
	svc, err := email.NewSESService(ctx, cfg.AWS)
	if err != nil {
		log.Fatalf("Failed to create email service: %v", err)
	}

	// verify sender identity (required for SES)
	log.Printf("Verifying sender identity %s...", svc.Sender())
	_, err = svc.Client().VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(svc.Sender()),
	})
	if err != nil {
		log.Printf("Warning: Failed to verify identity: %v", err)
	}

	// send a test email
	to := "test@example.com"
	subject := "Test Email from LocalStack"
	body := "Sup ladies and gentlemen"

	log.Printf("Sending email to %s...", to)
	err = svc.SendEmail(ctx, to, subject, body)
	if err != nil {
		log.Fatalf("Failed to send email: %v", err)
	}

	log.Println("Email sent successfully!")

	// retrieve messages from LocalStack API (for verification)
	log.Println("\n--- LocalStack SES Inbox ---")
	// fetch from http://localhost:4566/_aws/ses
	resp, err := http.Get("http://localhost:4566/_aws/ses")
	if err != nil {
		log.Printf("Failed to fetch LocalStack messages: %v", err)
		return
	}
	defer resp.Body.Close()

	bodyData, _ := io.ReadAll(resp.Body)
	var lsResp LocalStackResponse
	if err := json.Unmarshal(bodyData, &lsResp); err != nil {
		log.Printf("Failed to parse LocalStack response: %v\nRaw body: %s", err, string(bodyData))
		return
	}

	if len(lsResp.Messages) == 0 {
		fmt.Println("No messages found in LocalStack.")
		return
	}

	fmt.Printf("\nFound %d message(s):\n", len(lsResp.Messages))
	for i, msg := range lsResp.Messages {
		fmt.Printf("\n[%d] Time: %s\n", i+1, msg.Timestamp)
		fmt.Printf("To: %v\n", msg.Destination.ToAddresses)
		fmt.Printf("Subject: %s\n", msg.Subject)
		fmt.Printf("Body: %s\n", msg.Body.Text)
		fmt.Println("---------------------------------------------------")
	}
}
