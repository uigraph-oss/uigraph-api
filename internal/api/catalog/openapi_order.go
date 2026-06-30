package catalog

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseOpenAPIDocumentOrder extracts path and HTTP method order as declared in the
// spec file. yaml.v3 preserves key order for both YAML and JSON documents.
func parseOpenAPIDocumentOrder(spec string) (paths []string, methodsByPath map[string][]string, err error) {
	var root yaml.Node
	if err = yaml.Unmarshal([]byte(spec), &root); err != nil {
		return nil, nil, err
	}

	docNode := documentContent(&root)
	if docNode == nil {
		return []string{}, map[string][]string{}, nil
	}

	pathsNode := mappingValue(docNode, "paths")
	if pathsNode == nil || pathsNode.Kind != yaml.MappingNode {
		return []string{}, map[string][]string{}, nil
	}

	methodsByPath = make(map[string][]string)
	for i := 0; i+1 < len(pathsNode.Content); i += 2 {
		path := pathsNode.Content[i].Value
		paths = append(paths, path)

		pathItemNode := pathsNode.Content[i+1]
		if pathItemNode.Kind != yaml.MappingNode {
			continue
		}

		var methods []string
		for j := 0; j+1 < len(pathItemNode.Content); j += 2 {
			key := strings.ToLower(pathItemNode.Content[j].Value)
			if isHTTPMethodKey(key) {
				methods = append(methods, key)
			}
		}
		methodsByPath[path] = methods
	}

	return paths, methodsByPath, nil
}

func documentContent(root *yaml.Node) *yaml.Node {
	if root == nil || len(root.Content) == 0 {
		return nil
	}
	return root.Content[0]
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func isHTTPMethodKey(s string) bool {
	switch s {
	case "get", "post", "put", "delete", "patch", "head", "options", "trace":
		return true
	default:
		return false
	}
}

func sortedPathKeys(paths map[string]interface{}) []string {
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func methodsForPathItem(
	path string,
	item map[string]interface{},
	methodsByPath map[string][]string,
) []string {
	if ordered, ok := methodsByPath[path]; ok && len(ordered) > 0 {
		out := make([]string, 0, len(ordered))
		for _, method := range ordered {
			if _, exists := item[method]; exists {
				out = append(out, method)
			}
		}
		return out
	}

	out := make([]string, 0, len(httpMethods))
	for _, method := range httpMethods {
		if _, exists := item[method]; exists {
			out = append(out, method)
		}
	}
	return out
}
