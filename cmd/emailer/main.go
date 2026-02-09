package main

import (
	"context"
	"flag"
	"log"

	"encoding/json"
	"fmt"
	"io"
	"net/http"

	emailSvc "github.com/USSTM/cv-backend/internal/aws"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/queue"
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

var (
	enqueuePtr = flag.Bool("enqueue", false, "Enqueue the email task instead of sending directly")
	viewPtr    = flag.Bool("view", false, "View the emails")
	testPtr    = flag.Bool("test", false, "Test sending an email")
)

func main() {
	flag.Parse()

	cfg := config.Load()

	to := "test@example.com"
	subject := "Test Email from LocalStack"
	body := "Sup ladies and gentlemen"

	// this is for make email-enqueue (enqueueing the email to redis/asynq to then be processed by the worker)
	if *enqueuePtr {
		log.Println("Initializing Redis queue...")
		q, err := queue.NewQueue(&cfg.Redis)
		if err != nil {
			log.Fatalf("Failed to connect to queue: %v", err)
		}
		defer q.Close()

		log.Printf("Enqueuing email to %s...", to)
		payload := queue.EmailDeliveryPayload{
			To:      to,
			Subject: subject,
			Body:    body,
		}

		// enqueue email
		info, err := q.Enqueue(queue.TypeEmailDelivery, payload)
		if err != nil {
			log.Fatalf("Failed to enqueue task: %v", err)
		}
		log.Printf("Task enqueued successfully! ID: %s", info.ID)
		return
	}

	// this is for make email-view (viewing the emails)
	if *viewPtr {
		viewEmails()
		return
	}

	// this is for make email-test (testing to send an email directly)
	if *testPtr {
		log.Println("Initializing email service...")
		svc, err := emailSvc.NewEmailService(cfg.AWS)
		if err != nil {
			log.Fatalf("Failed to create email service: %v", err)
		}

		log.Printf("Verifying sender identity %s...", svc.Sender())
		_, err = svc.VerifyEmailIdentity(context.Background())
		if err != nil {
			log.Fatalf("Failed to verify email identity: %v", err)
		}

		log.Printf("Sending email to %s...", to)
		err = svc.SendEmail(context.Background(), to, subject, body)
		if err != nil {
			log.Fatalf("Failed to send email: %v", err)
		}

		log.Println("Email sent successfully!")

		viewEmails()
	}
}

func viewEmails() {
	log.Println("\n--- LocalStack SES Inbox ---")

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
