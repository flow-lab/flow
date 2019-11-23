package base64

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEncode(t *testing.T) {
	str := "test"

	bytes := Encode(str)

	assert.Equal(t, "dGVzdA==", string(bytes))
}

func TestDecode(t *testing.T) {
	str := "dGVzdA=="

	bytes, err := Decode(str)

	assert.Nil(t, err)
	assert.Equal(t, "test", string(bytes))
}
