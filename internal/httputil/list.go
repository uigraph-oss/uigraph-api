package httputil

import "strconv"

const (
	defaultListLimit = 24
	maxListLimit     = 100
)

func ListLimit(raw string) int {
	if raw == "" {
		return defaultListLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultListLimit
	}
	if n > maxListLimit {
		return maxListLimit
	}
	return n
}

func ListOffset(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
