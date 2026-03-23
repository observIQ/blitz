package hec

import "github.com/google/uuid"

// generateChannelID generates a random UUID v4 string for use as a HEC channel ID.
func generateChannelID() string {
	return uuid.New().String()
}
