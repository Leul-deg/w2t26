// Package crypto provides field-level AES-256-GCM encryption for sensitive data.
// The encryption key is loaded once at startup from a file path in config.
package crypto

import (
	"fmt"
	"os"
)

const keyLength = 32 // AES-256 requires a 32-byte key

// LoadKey reads a 32-byte AES-256 key from the given file path.
// It fails fast if the file cannot be read or if the key is not exactly 32 bytes.
// The key file should contain exactly 32 raw bytes (not hex or base64 encoded).
func LoadKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to read key file %q: %w", path, err)
	}
	if len(data) != keyLength {
		return nil, fmt.Errorf("crypto: key file must be exactly %d bytes, got %d", keyLength, len(data))
	}
	// Return a copy to prevent mutation of the file contents buffer.
	key := make([]byte, keyLength)
	copy(key, data)
	return key, nil
}
