package encoding

import (
	"encoding/hex"
)

// ToHex encodes binary data to a hexadecimal string.
func ToHex(data []byte) string {
	return hex.EncodeToString(data)
}

// FromHex decodes a hexadecimal string to binary data.
func FromHex(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}

	return hex.DecodeString(s)
}
