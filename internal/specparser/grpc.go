package specparser

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
)

// ParsedGrpcMethod represents a single RPC method parsed from a .proto file.
type ParsedGrpcMethod struct {
	MethodID        string
	PackageName     string
	ServiceName     string
	MethodName      string
	RequestType     string
	ResponseType    string
	StreamingType   string
	Description     string
	ProtoSnippet    string
	RequestExample  string
	ResponseExample string
	Tags            []string
}

// ---------- Public API ----------

// ParseGrpcSchemas parses multiple .proto files and returns all RPC methods.
func ParseGrpcSchemas(protoContents [][]byte) ([]ParsedGrpcMethod, error) {
	if len(protoContents) == 0 {
		return nil, fmt.Errorf("no proto contents provided")
	}

	// Virtual filesystem for protos (enables multi-file parse consistency).
	vfs := map[string][]byte{}
	names := make([]string, 0, len(protoContents))
	for i, b := range protoContents {
		name := fmt.Sprintf("schema_%d.proto", i)
		vfs[name] = b
		names = append(names, name)
	}

	// Parse each file into AST.
	files := make([]fileInfo, 0, len(names))
	for _, name := range names {
		h := reporter.NewHandler(nil)
		fn, err := parser.Parse(name, strings.NewReader(string(vfs[name])), h)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", name, err)
		}
		pkg := extractProtoPackage(fn)
		files = append(files, fileInfo{
			name:    name,
			content: vfs[name],
			node:    fn,
			pkg:     pkg,
		})
	}

	// Merge message/type index across all files.
	idx := buildMergedTypeIndex(files)

	// Extract all methods.
	methods := extractAllMethods(files, idx)

	// Deterministic order.
	sort.SliceStable(methods, func(i, j int) bool {
		if methods[i].PackageName != methods[j].PackageName {
			return methods[i].PackageName < methods[j].PackageName
		}
		if methods[i].ServiceName != methods[j].ServiceName {
			return methods[i].ServiceName < methods[j].ServiceName
		}
		return methods[i].MethodName < methods[j].MethodName
	})

	return methods, nil
}

// ParseGrpc parses a single .proto file.
func ParseGrpc(specContent []byte) ([]ParsedGrpcMethod, error) {
	return ParseGrpcSchemas([][]byte{specContent})
}

// ---------- Extraction ----------

func extractAllMethods(files []fileInfo, idx typeIndex) []ParsedGrpcMethod {
	var out []ParsedGrpcMethod

	for _, f := range files {
		for _, decl := range f.node.Decls {
			svc, ok := decl.(*ast.ServiceNode)
			if !ok {
				continue
			}

			svcName := string(svc.Name.AsIdentifier())
			for _, sdecl := range svc.Decls {
				rpc, ok := sdecl.(*ast.RPCNode)
				if !ok {
					continue
				}

				methodName := string(rpc.Name.AsIdentifier())

				reqRaw, reqStream := rpcTypeRefString(rpc.Input)
				resRaw, resStream := rpcTypeRefString(rpc.Output)

				reqType := resolveTypeName(idx, f.pkg, reqRaw)
				resType := resolveTypeName(idx, f.pkg, resRaw)

				streaming := classifyStreaming(reqStream, resStream)

				methodID := methodIdentifier(f.pkg, svcName, methodName)

				snippet := snippetForRPC(rpc)

				reqExample := exampleJSONForType(idx, f.pkg, reqType, 3)
				resExample := exampleJSONForType(idx, f.pkg, resType, 3)

				out = append(out, ParsedGrpcMethod{
					MethodID:        methodID,
					PackageName:     f.pkg,
					ServiceName:     svcName,
					MethodName:      methodName,
					RequestType:     reqType,
					ResponseType:    resType,
					StreamingType:   streaming,
					Description:     "",
					ProtoSnippet:    snippet,
					RequestExample:  reqExample,
					ResponseExample: resExample,
					Tags:            []string{"gRPC", svcName},
				})
			}
		}
	}

	return out
}

func extractProtoPackage(file *ast.FileNode) string {
	for _, decl := range file.Decls {
		if p, ok := decl.(*ast.PackageNode); ok {
			return string(p.Name.AsIdentifier())
		}
	}
	return ""
}

func methodIdentifier(pkg, svc, method string) string {
	if strings.TrimSpace(pkg) == "" {
		return fmt.Sprintf("%s/%s", svc, method)
	}
	return fmt.Sprintf("%s.%s/%s", pkg, svc, method)
}

func rpcTypeRefString(t *ast.RPCTypeNode) (string, bool) {
	if t == nil {
		return "", false
	}
	isStream := t.Stream != nil
	if t.MessageType == nil {
		return "", isStream
	}
	// Convert Identifier to string
	return string(t.MessageType.AsIdentifier()), isStream
}

func classifyStreaming(reqStream, resStream bool) string {
	switch {
	case reqStream && resStream:
		return "BIDIRECTIONAL_STREAMING"
	case reqStream:
		return "CLIENT_STREAMING"
	case resStream:
		return "SERVER_STREAMING"
	default:
		return "UNARY"
	}
}

// snippetForRPC renders a best-effort `rpc Name(Request) returns (Response);` declaration.
func snippetForRPC(rpc *ast.RPCNode) string {
	if rpc == nil {
		return ""
	}
	req, reqStream := rpcTypeRefString(rpc.Input)
	res, resStream := rpcTypeRefString(rpc.Output)

	rs := ""
	if reqStream {
		rs = "stream "
	}
	ss := ""
	if resStream {
		ss = "stream "
	}

	return fmt.Sprintf("rpc %s(%s%s) returns (%s%s);",
		rpc.Name.AsIdentifier(), rs, strings.TrimPrefix(req, "."), ss, strings.TrimPrefix(res, "."))
}

// ---------- Type index + examples ----------

type fileInfo struct {
	name    string
	content []byte
	node    *ast.FileNode
	pkg     string
}

type msgField struct {
	Name     string
	TypeName string
	IsList   bool
}

type msgDef struct {
	FullName string
	Fields   []msgField
}

type typeIndex map[string]msgDef

func buildMergedTypeIndex(files []fileInfo) typeIndex {
	idx := typeIndex{}
	for _, f := range files {
		indexFileMessages(idx, f.node, f.pkg)
	}
	return idx
}

func indexFileMessages(idx typeIndex, file *ast.FileNode, pkg string) {
	for _, decl := range file.Decls {
		m, ok := decl.(*ast.MessageNode)
		if !ok {
			continue
		}
		indexMessageRecursive(idx, pkg, "", m)
	}
}

func indexMessageRecursive(idx typeIndex, pkg string, prefix string, m *ast.MessageNode) {
	if m == nil {
		return
	}

	name := string(m.Name.AsIdentifier())
	full := name
	if prefix != "" {
		full = prefix + "." + name
	}

	def := msgDef{
		FullName: qualify(pkg, full),
		Fields:   extractMessageFields(m),
	}

	// Store common lookup keys.
	storeType(idx, name, def)
	storeType(idx, full, def)
	if pkg != "" {
		storeType(idx, pkg+"."+name, def)
		storeType(idx, pkg+"."+full, def)
	}

	// Nested messages
	for _, decl := range m.Decls {
		nested, ok := decl.(*ast.MessageNode)
		if !ok {
			continue
		}
		indexMessageRecursive(idx, pkg, full, nested)
	}
}

func extractMessageFields(m *ast.MessageNode) []msgField {
	var fields []msgField

	for _, decl := range m.Decls {
		f, ok := decl.(*ast.FieldNode)
		if !ok {
			continue
		}

		fieldName := string(f.Name.AsIdentifier())
		fieldType := fieldTypeName(f)
		if fieldName == "" || fieldType == "" {
			continue
		}

		isRepeated := false
		if f.Label.IsPresent() {
			isRepeated = strings.EqualFold(f.Label.Val, "repeated")
		}

		fields = append(fields, msgField{
			Name:     fieldName,
			TypeName: fieldType,
			IsList:   isRepeated,
		})
	}

	return fields
}

func fieldTypeName(f *ast.FieldNode) string {
	if f == nil || f.FieldType() == nil {
		return ""
	}

	switch t := f.FieldType().(type) {
	case *ast.IdentNode:
		return string(t.AsIdentifier())
	case *ast.CompoundIdentNode:
		// For qualified names like .pkg.Type - build from components
		parts := t.Components
		if len(parts) == 0 {
			return ""
		}
		var names []string
		for _, part := range parts {
			names = append(names, string(part.AsIdentifier()))
		}
		return strings.Join(names, ".")
	default:
		// map<k,v> etc. — keep it simple for examples
		return ""
	}
}

func qualify(pkg, t string) string {
	t = strings.TrimPrefix(strings.TrimSpace(t), ".")
	if pkg == "" || strings.Contains(t, ".") {
		return t
	}
	return pkg + "." + t
}

func storeType(idx typeIndex, key string, def msgDef) {
	key = strings.TrimSpace(strings.TrimPrefix(key, "."))
	if key == "" {
		return
	}
	if _, exists := idx[key]; !exists {
		idx[key] = def
	}
}

func resolveTypeName(idx typeIndex, pkg string, raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "."))
	if raw == "" {
		return raw
	}
	if isScalarProtoType(raw) {
		return raw
	}
	if _, ok := idx[raw]; ok {
		return raw
	}
	if pkg != "" {
		if _, ok := idx[pkg+"."+raw]; ok {
			return pkg + "." + raw
		}
	}
	return raw
}

func exampleJSONForType(idx typeIndex, pkg string, typeName string, depth int) string {
	v, ok := exampleValueForType(idx, pkg, typeName, depth)
	if !ok {
		return "{}"
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func exampleValueForType(idx typeIndex, pkg string, typeName string, depth int) (any, bool) {
	typeName = strings.TrimSpace(strings.TrimPrefix(typeName, "."))
	if typeName == "" {
		return map[string]any{}, false
	}

	if isScalarProtoType(typeName) {
		return exampleScalar(typeName), true
	}

	if depth <= 0 {
		return map[string]any{}, true
	}

	// Try resolution keys.
	keys := []string{typeName}
	if pkg != "" && !strings.Contains(typeName, ".") {
		keys = append(keys, pkg+"."+typeName)
	}

	var def msgDef
	found := false
	for _, k := range keys {
		if d, ok := idx[k]; ok {
			def = d
			found = true
			break
		}
	}
	if !found {
		return map[string]any{}, false
	}

	obj := map[string]any{}
	for _, f := range def.Fields {
		ft := strings.TrimSpace(strings.TrimPrefix(f.TypeName, "."))
		val, _ := exampleValueForType(idx, pkg, ft, depth-1)

		if f.IsList {
			obj[f.Name] = []any{val}
		} else {
			obj[f.Name] = val
		}
	}
	return obj, true
}

func isScalarProtoType(t string) bool {
	switch t {
	case "string", "bytes",
		"int32", "int64", "uint32", "uint64",
		"sint32", "sint64",
		"fixed32", "fixed64",
		"sfixed32", "sfixed64",
		"float", "double",
		"bool":
		return true
	case "google.protobuf.Timestamp", "Timestamp":
		return true
	}
	return false
}

func exampleScalar(t string) any {
	switch t {
	case "string":
		return "example"
	case "bytes":
		return "ZXhhbXBsZQ=="
	case "int32", "int64", "uint32", "uint64", "sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64":
		return 123
	case "float", "double":
		return 123.45
	case "bool":
		return true
	case "google.protobuf.Timestamp", "Timestamp":
		return "2025-01-01T00:00:00Z"
	default:
		return "example"
	}
}
