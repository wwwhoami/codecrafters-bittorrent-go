package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenRandStr generates a random string of the specified length.
// The resulting string is base64 encoded.
func GenRandStr(length int) (string, error) {
	buffer := make([]byte, length)

	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(buffer)[:length], nil
}

// GetStringFromMap returns a string value from a map.
func GetStringFromMap(m map[string]any, key string) (string, error) {
	if value, ok := m[key].(string); ok {
		return value, nil
	}
	return "", fmt.Errorf("invalid %s", key)
}

// GetStringOrBytesFromMap returns a string or bytes value from a map.
func GetStringOrBytesFromMap(m map[string]any, key string) (string, error) {
	if value, ok := m[key].(string); ok {
		return value, nil
	} else if value, ok := m[key].([]byte); ok {
		return string(value), nil
	}
	return "", fmt.Errorf("invalid %s", key)
}

// GetIntFromMap returns an int value from a map.
func GetIntFromMap(m map[string]any, key string) (int, error) {
	if value, ok := m[key].(int); ok {
		return value, nil
	}
	return 0, fmt.Errorf("invalid %s", key)
}
