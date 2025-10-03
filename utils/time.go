package utils

import "time"

// GetCurrentTimestamp returns the current Unix timestamp in seconds
func GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}
