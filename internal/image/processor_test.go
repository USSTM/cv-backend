package image_test

import (
	"bytes"
	stdimage "image"
	"image/color"
	imgdraw "image/draw"
	"image/jpeg"
	"mime/multipart"
	"testing"

	cvimage "github.com/USSTM/cv-backend/internal/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mock jpeg image
func createTestJPEG(t *testing.T, w, h int) ([]byte, *multipart.FileHeader) {
	t.Helper()
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, w, h))
	imgdraw.Draw(img, img.Bounds(), &stdimage.Uniform{color.RGBA{R: 255, A: 255}}, stdimage.Point{}, imgdraw.Src)
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes(), &multipart.FileHeader{Size: int64(buf.Len())}
}

func TestValidateAndProcess_AcceptsValidSquareJPEG(t *testing.T) {
	data, header := createTestJPEG(t, 500, 500)
	result, err := cvimage.ValidateAndProcess(bytes.NewReader(data), header)
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", result.ContentType)
	assert.Equal(t, 500, result.Width)
	assert.Equal(t, 500, result.Height)
	assert.NotEmpty(t, result.Original)
	assert.NotEmpty(t, result.Thumbnail)
}

func TestValidateAndProcess_AcceptsNonSquareImage(t *testing.T) {
	data, header := createTestJPEG(t, 800, 400)
	result, err := cvimage.ValidateAndProcess(bytes.NewReader(data), header)
	require.NoError(t, err)
	assert.Equal(t, 800, result.Width)
	assert.Equal(t, 400, result.Height)
}

func TestValidateAndProcess_RejectsOversizedFile(t *testing.T) {
	data, _ := createTestJPEG(t, 100, 100)
	header := &multipart.FileHeader{Size: 11 * 1024 * 1024} // size too big
	_, err := cvimage.ValidateAndProcess(bytes.NewReader(data), header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file size")
}

func TestValidateAndProcess_RejectsTooLargeDimensions(t *testing.T) {
	// 4097px image exceeds limit
	data, header := createTestJPEG(t, 4097, 4097)
	_, err := cvimage.ValidateAndProcess(bytes.NewReader(data), header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dimensions")
}

func TestIsSquare(t *testing.T) {
	assert.True(t, cvimage.IsSquare(500, 500))
	assert.True(t, cvimage.IsSquare(100, 101)) // within 1%
	assert.False(t, cvimage.IsSquare(800, 400))
	assert.False(t, cvimage.IsSquare(500, 600))
}

func TestThumbnailSize(t *testing.T) {
	data, header := createTestJPEG(t, 800, 400)
	result, err := cvimage.ValidateAndProcess(bytes.NewReader(data), header)
	require.NoError(t, err)

	img, _, err := stdimage.Decode(bytes.NewReader(result.Thumbnail))
	require.NoError(t, err)
	assert.Equal(t, 300, img.Bounds().Dx())
	assert.Equal(t, 300, img.Bounds().Dy())
}
