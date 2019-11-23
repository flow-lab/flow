package base64

import (
	"encoding/base64"
	"fmt"
)

// Encode encodes src using the encoding enc
func Encode(src string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(src)))
}

// Decode decodes src using the encoding enc.
// New line characters (\r and \n) are ignored.
func Decode(src string) ([]byte, error) {
	dst, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		return nil, fmt.Errorf("call to base64.StdEncoding.Decode failed: %s", err)
	}
	return dst, nil
}
