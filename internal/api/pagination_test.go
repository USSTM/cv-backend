package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePagination(t *testing.T) {
	t.Run("nil nil to 50 0", func(t *testing.T) {
		l, o := parsePagination(nil, nil)
		assert.Equal(t, int64(50), l)
		assert.Equal(t, int64(0), o)
	})

	t.Run("valid values success", func(t *testing.T) {
		limit, offset := 10, 5
		l, o := parsePagination(&limit, &offset)
		assert.Equal(t, int64(10), l)
		assert.Equal(t, int64(5), o)
	})

	t.Run("limit capped to 100", func(t *testing.T) {
		limit := 200
		l, _ := parsePagination(&limit, nil)
		assert.Equal(t, int64(100), l)
	})

	t.Run("limit minimum set to 1", func(t *testing.T) {
		limit := 0
		l, _ := parsePagination(&limit, nil)
		assert.Equal(t, int64(1), l)
	})

	t.Run("negative limit set to 1", func(t *testing.T) {
		limit := -5
		l, _ := parsePagination(&limit, nil)
		assert.Equal(t, int64(1), l)
	})

	t.Run("negative offset set to 0", func(t *testing.T) {
		offset := -10
		_, o := parsePagination(nil, &offset)
		assert.Equal(t, int64(0), o)
	})
}
