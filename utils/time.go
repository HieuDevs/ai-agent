package utils

import (
	"strconv"
	"time"
)

// GetCurrentTimestamp returns the current Unix timestamp in seconds
func GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// GetCurrentTimestampString returns the current Unix timestamp as a string
func GetCurrentTimestampString() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}
