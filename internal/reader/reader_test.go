package reader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadStdin(t *testing.T) {
	t.Run("should read from stdin pipe", func(t *testing.T) {
		r, w, err := os.Pipe()
		assert.NoError(t, err)

		origStdin := os.Stdin
		os.Stdin = r
		defer func() { os.Stdin = origStdin }()

		_, err = w.WriteString("hello\nworld\n")
		assert.NoError(t, err)
		_ = w.Close()

		result := ReadStdin()
		assert.Equal(t, "helloworld", result)
	})

	t.Run("should return empty string for empty stdin", func(t *testing.T) {
		r, w, err := os.Pipe()
		assert.NoError(t, err)

		origStdin := os.Stdin
		os.Stdin = r
		defer func() { os.Stdin = origStdin }()

		_ = w.Close()

		result := ReadStdin()
		assert.Equal(t, "", result)
	})
}
