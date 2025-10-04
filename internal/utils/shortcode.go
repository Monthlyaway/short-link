package utils

import (
	"strings"
)

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// GenerateShortCode generates a short code from a snowflake ID using Base62 encoding
func GenerateShortCode() (string, error) {
	id, err := GenerateID()
	if err != nil {
		return "", err
	}
	return EncodeBase62(id), nil
}

// EncodeBase62 converts a decimal number to Base62 encoding
func EncodeBase62(num int64) string {
	if num == 0 {
		return string(base62Chars[0])
	}

	var result strings.Builder
	base := int64(len(base62Chars))

	for num > 0 {
		remainder := num % base
		result.WriteByte(base62Chars[remainder])
		num = num / base
	}

	// Reverse the string
	encoded := result.String()
	return reverseString(encoded)
}

// DecodeBase62 converts a Base62 string back to decimal number
func DecodeBase62(encoded string) int64 {
	var num int64
	base := int64(len(base62Chars))

	for i := 0; i < len(encoded); i++ {
		char := encoded[i]
		var value int64

		switch {
		case char >= '0' && char <= '9':
			value = int64(char - '0')
		case char >= 'a' && char <= 'z':
			value = int64(char-'a') + 10
		case char >= 'A' && char <= 'Z':
			value = int64(char-'A') + 36
		default:
			return 0
		}

		num = num*base + value
	}

	return num
}

// reverseString reverses a string
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
