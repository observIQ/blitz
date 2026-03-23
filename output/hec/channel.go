package hec

import (
	"crypto/rand"
	"fmt"
)

// generateChannelID generates a random UUID v4 string for use as a HEC channel ID.
func generateChannelID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])

	// Set version (4) and variant (RFC 4122)
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
