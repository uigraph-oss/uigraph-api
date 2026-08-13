package httputil

import (
	"net/url"
	"strconv"
	"strings"
)

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

func Exact(q url.Values, name string) *string {
	v := strings.TrimSpace(q.Get(name))
	if v == "" {
		return nil
	}
	return &v
}
