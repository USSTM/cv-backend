package image

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/disintegration/imaging"
)

const (
	MaxFileSize   = 10 * 1024 * 1024 // 10MB
	MaxDimension  = 4096
	ThumbnailSize = 300
)

type ProcessedImage struct {
	Original    []byte
	Thumbnail   []byte
	ContentType string
	Width       int
	Height      int
}


// generates a 300x300 center-cropped thumbnail from jpeg/png.
func ValidateAndProcess(file io.Reader, header *multipart.FileHeader) (*ProcessedImage, error) {
	if header.Size > MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum %d bytes", header.Size, MaxFileSize)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" && contentType != "image/png" {
		return nil, fmt.Errorf("invalid file type %q: only jpeg and png are allowed", contentType)
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w > MaxDimension || h > MaxDimension {
		return nil, fmt.Errorf("image dimensions %dx%d exceed maximum %d", w, h, MaxDimension)
	}

	thumb := imaging.Fill(img, ThumbnailSize, ThumbnailSize, imaging.Center, imaging.Lanczos)

	var thumbBuf bytes.Buffer
	switch format {
	case "jpeg":
		err = imaging.Encode(&thumbBuf, thumb, imaging.JPEG, imaging.JPEGQuality(85))
	case "png":
		err = imaging.Encode(&thumbBuf, thumb, imaging.PNG)
	default:
		err = imaging.Encode(&thumbBuf, thumb, imaging.JPEG, imaging.JPEGQuality(85))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return &ProcessedImage{
		Original:    data,
		Thumbnail:   thumbBuf.Bytes(),
		ContentType: contentType,
		Width:       w,
		Height:      h,
	}, nil
}

// returns true if the dimensions are within 0.01 of a 1:1 (square) ratio.
func IsSquare(width, height int) bool {
	if width == height {
		return true
	}
	larger, smaller := width, height
	if height > width {
		larger, smaller = height, width
	}
	return float64(larger-smaller)/float64(larger) <= 0.01
}
