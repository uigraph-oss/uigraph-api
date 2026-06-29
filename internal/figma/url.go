package figma

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	validURLRe = regexp.MustCompile(`^https://www\.figma\.com/[^/]+/[a-zA-Z0-9]+`)
	fileKeyRe  = regexp.MustCompile(`figma\.com/[^/]+/([a-zA-Z0-9]+)`)
)

func ParseURL(figmaURL string) (fileKey, nodeID string, err error) {
	if !validURLRe.MatchString(figmaURL) {
		return "", "", fmt.Errorf("figma: invalid Figma URL")
	}
	m := fileKeyRe.FindStringSubmatch(figmaURL)
	if m == nil {
		return "", "", fmt.Errorf("figma: URL should have a file key")
	}
	fileKey = m[1]

	var query string
	if i := strings.IndexByte(figmaURL, '?'); i >= 0 {
		query = figmaURL[i+1:]
	}
	params, _ := url.ParseQuery(query)
	rawNodeID := params.Get("node-id")
	if rawNodeID == "" {
		return "", "", fmt.Errorf("figma: URL should have a node id")
	}
	nodeID = strings.ReplaceAll(rawNodeID, "-", ":")
	return fileKey, nodeID, nil
}
