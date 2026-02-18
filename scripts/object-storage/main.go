package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/USSTM/cv-backend/internal/aws"
	"github.com/USSTM/cv-backend/internal/config"
)

var (
	uploadPtr  = flag.String("upload", "", "Path to file to upload")
	getPtr     = flag.String("get", "", "Key of file to retrieve")
	linkPtr    = flag.String("link", "", "Key of file to generate presigned URL for")
	listPtr    = flag.Bool("list", false, "List all objects in the bucket")
	bucketsPtr = flag.Bool("buckets", false, "List all buckets")
)

func main() {
	flag.Parse()

	cfg := config.Load()

	s3Service, err := aws.NewS3Service(cfg.AWS)
	if err != nil {
		log.Fatalf("Failed to initialize S3 service: %v", err)
	}

	ctx := context.Background()

	// create bucket if it doesn't exist (for localstack)
	if err := s3Service.CreateBucket(ctx); err != nil {
		log.Fatalf("Warning: failed to ensure bucket exists: %v", err)
	}

	if *uploadPtr != "" {
		filePath := *uploadPtr
		file, err := os.Open(filePath)
		if err != nil {
			log.Fatalf("Failed to open file: %v", err)
		}
		defer file.Close()

		key := filepath.Base(filePath)
		contentType := "application/octet-stream"

		fmt.Printf("Uploading %s to %s/%s...\n", filePath, cfg.AWS.Bucket, key)
		if err := s3Service.PutObject(ctx, key, file, contentType); err != nil {
			log.Fatalf("Failed to upload file: %v", err)
		}

		fmt.Println("Upload successful!")
		return
	}

	if *getPtr != "" {
		key := *getPtr
		fmt.Printf("Retrieving %s from %s...\n", key, cfg.AWS.Bucket)

		body, err := s3Service.GetObject(ctx, key)
		if err != nil {
			log.Fatalf("Failed to get file: %v", err)
		}
		defer body.Close()

		outFile, err := os.Create(key)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, body); err != nil {
			log.Fatalf("Failed to save file: %v", err)
		}
		fmt.Printf("File saved to %s", key)
		return
	}

	if *linkPtr != "" {
		key := *linkPtr
		url, err := s3Service.GeneratePresignedURL(ctx, "GET", key, 15*time.Minute)
		if err != nil {
			log.Fatalf("Failed to generate presigned URL: %v", err)
		}
		fmt.Printf("Presigned URL for %s (expires in 15m):\n%s\n", key, url)
		return
	}

	if *listPtr {
		fmt.Printf("Listing objects in bucket %s...", cfg.AWS.Bucket)
		objects, err := s3Service.ListObjects(ctx)
		if err != nil {
			log.Fatalf("Failed to list objects: %v", err)
		}

		if len(objects) == 0 {
			fmt.Println("No objects found.")
		} else {
			fmt.Printf("%-30s %-10s %s\n", "Key", "Size", "LastModified")
			fmt.Println("------------------------------------------------------------")
			for _, obj := range objects {
				fmt.Printf("%-30s %-10d %s\n", *obj.Key, obj.Size, obj.LastModified.Format(time.RFC3339))
			}
		}
		return
	}

	if *bucketsPtr {
		fmt.Println("Listing all buckets...")
		buckets, err := s3Service.ListBuckets(ctx)
		if err != nil {
			log.Fatalf("Failed to list buckets: %v", err)
		}

		if len(buckets) == 0 {
			fmt.Println("No buckets found.")
		} else {
			fmt.Println("Buckets:")
			for _, b := range buckets {
				fmt.Printf("- %s (created: %s)\n", *b.Name, b.CreationDate.Format(time.RFC3339))
			}
		}
		return
	}
}
