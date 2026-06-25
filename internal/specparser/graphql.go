// Package specparser parses non-OpenAPI API specifications (GraphQL SDL,
// gRPC protobuf) into a flat list of operations/methods, mirroring the shape
// produced for OpenAPI specs elsewhere in the catalog package.
//
// Adapted from project-manager/service/api-parser.
package specparser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// ParsedGraphQLOperation represents a parsed GraphQL operation derived from a
// root field (Query, Mutation, or Subscription).
type ParsedGraphQLOperation struct {
	OperationID     string   // Root field name (e.g., v1GetProject)
	Name            string   // Same as OperationID
	Kind            string   // Query, Mutation, Subscription
	Signature       string   // e.g., v1GetProject(orgId: String): Project
	Description     string   // Field description + deprecation note
	Variables       string   // JSON representation of variables payload (UI hint)
	RequestExample  string   // Example request document (valid GraphQL)
	ResponseExample string   // Example response JSON mirroring selection set
	Tags            []string // e.g., Query, deprecated, @auth, returns:Project
}

// ParseGraphQLSchemas parses multiple GraphQL SDL schema files and returns operations from the root types.
func ParseGraphQLSchemas(schemaContents [][]byte) ([]ParsedGraphQLOperation, error) {
	if len(schemaContents) == 0 {
		return nil, fmt.Errorf("no schema contents provided")
	}

	sources := make([]*ast.Source, 0, len(schemaContents))
	for i, content := range schemaContents {
		sources = append(sources, &ast.Source{
			Name:  fmt.Sprintf("schema_%d.graphql", i),
			Input: string(content),
		})
	}

	schema, err := gqlparser.LoadSchema(sources...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL schemas: %w", err)
	}

	var ops []ParsedGraphQLOperation
	if schema.Query != nil {
		ops = append(ops, extractOperationsFromType(schema, schema.Query, "Query")...)
	}
	if schema.Mutation != nil {
		ops = append(ops, extractOperationsFromType(schema, schema.Mutation, "Mutation")...)
	}
	if schema.Subscription != nil {
		ops = append(ops, extractOperationsFromType(schema, schema.Subscription, "Subscription")...)
	}

	// Stable ordering for determinism in UI and tests.
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Kind == ops[j].Kind {
			return ops[i].Name < ops[j].Name
		}
		return ops[i].Kind < ops[j].Kind
	})

	return ops, nil
}

// ParseGraphQL parses a single GraphQL SDL schema file.
func ParseGraphQL(specContent []byte) ([]ParsedGraphQLOperation, error) {
	return ParseGraphQLSchemas([][]byte{specContent})
}

func extractOperationsFromType(schema *ast.Schema, typeDef *ast.Definition, kind string) []ParsedGraphQLOperation {
	if schema == nil || typeDef == nil {
		return nil
	}

	ops := make([]ParsedGraphQLOperation, 0, len(typeDef.Fields))
	for _, field := range typeDef.Fields {
		if field == nil {
			continue
		}

		// Skip GraphQL introspection fields
		if strings.HasPrefix(field.Name, "__") {
			continue
		}

		ops = append(ops, parseFieldAsOperation(schema, field, kind))
	}
	return ops
}

func parseFieldAsOperation(schema *ast.Schema, field *ast.FieldDefinition, kind string) ParsedGraphQLOperation {
	signature := buildFieldSignature(field)

	desc := strings.TrimSpace(field.Description)
	tags := buildTags(field, kind)

	// Deprecation notes
	if dep := field.Directives.ForName("deprecated"); dep != nil {
		reason := "No reason provided"
		if a := dep.Arguments.ForName("reason"); a != nil && a.Value != nil && strings.TrimSpace(a.Value.Raw) != "" {
			reason = strings.Trim(a.Value.Raw, `"`)
		}
		if desc != "" {
			desc += "\n\n"
		}
		desc += fmt.Sprintf("DEPRECATED: %s", reason)
	}

	// Build a deterministic selection tree from schema
	visited := map[string]bool{}
	sel := buildSelectionTree(schema, field.Type, selectionOptions{
		MaxDepth:        10,    // Reasonable depth - prevents massive queries while showing full structure
		MaxScalarFields: 10000, // No limit - show all scalar fields
		MaxObjectFields: 10000, // No limit - show all nested objects
	}, visited)

	selectionSet := renderSelectionSet(sel, 2)

	varsJSON := buildVariablesJSON(schema, field.Arguments, 2)

	req := renderRequestExample(kind, field.Name, field.Arguments, selectionSet)

	resp := renderResponseExample(schema, field.Name, field.Type, sel, 2)

	return ParsedGraphQLOperation{
		OperationID:     field.Name,
		Name:            field.Name,
		Kind:            kind,
		Signature:       signature,
		Description:     desc,
		Variables:       varsJSON,
		RequestExample:  req,
		ResponseExample: resp,
		Tags:            tags,
	}
}

func buildTags(field *ast.FieldDefinition, kind string) []string {
	tags := []string{kind}

	if field.Directives.ForName("deprecated") != nil {
		tags = append(tags, "deprecated")
	}

	for _, d := range field.Directives {
		if d == nil || d.Name == "" || d.Name == "deprecated" {
			continue
		}
		tags = append(tags, "@"+d.Name)
	}

	ret := getBaseTypeName(field.Type)
	if ret != "" {
		tags = append(tags, "returns:"+ret)
	}
	return tags
}

func buildFieldSignature(field *ast.FieldDefinition) string {
	var b strings.Builder
	b.WriteString(field.Name)

	if len(field.Arguments) > 0 {
		b.WriteString("(")
		for i, a := range field.Arguments {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(a.Name)
			b.WriteString(": ")
			b.WriteString(formatType(a.Type))
		}
		b.WriteString(")")
	}

	b.WriteString(": ")
	b.WriteString(formatType(field.Type))
	return b.String()
}

func formatType(t *ast.Type) string {
	if t == nil {
		return ""
	}
	if t.Elem != nil {
		inner := "[" + formatType(t.Elem) + "]"
		if t.NonNull {
			return inner + "!"
		}
		return inner
	}
	if t.NonNull {
		return t.NamedType + "!"
	}
	return t.NamedType
}

func getBaseTypeName(t *ast.Type) string {
	if t == nil {
		return ""
	}
	if t.Elem != nil {
		return getBaseTypeName(t.Elem)
	}
	return t.NamedType
}

/*
Selection tree (production-ready approach)

We build a typed selection tree and render both:
- request selection set
- response JSON

This avoids reparsing GraphQL strings and prevents the "{ }" missing-shape bug.
*/

type selectionOptions struct {
	MaxDepth        int
	MaxScalarFields int
	MaxObjectFields int
}

// selNode represents a selection set node for request/response generation.
type selNode struct {
	FieldName  string
	Children   []*selNode
	Fragments  map[string][]*selNode // for unions/interfaces: typeName -> child selections
	IsLeaf     bool                  // scalar/enum leaf
	ReturnType *ast.Type             // helpful for response generation
}

// buildSelectionTree builds a selection tree from schema + return type.
// It selects a limited number of fields to keep examples readable.
func buildSelectionTree(schema *ast.Schema, returnType *ast.Type, opts selectionOptions, visited map[string]bool) []*selNode {
	if schema == nil || returnType == nil {
		return nil
	}
	base := getBaseTypeName(returnType)
	if base == "" {
		return nil
	}

	// Stop cycles by base type name
	if visited[base] {
		return nil
	}
	visited[base] = true
	defer func() { visited[base] = false }()

	def := schema.Types[base]
	if def == nil {
		return nil
	}

	// Scalars/enums have no sub-selection
	if def.Kind == ast.Scalar || def.Kind == ast.Enum {
		return nil
	}

	if opts.MaxDepth <= 0 {
		return nil
	}

	switch def.Kind {
	case ast.Object:
		return buildObjectSelection(schema, def, opts, visited)

	case ast.Interface:
		// We can select interface fields directly, but also could add fragments.
		// For simplicity: select interface fields + 0-1 nested object.
		return buildObjectSelection(schema, def, opts, visited)

	case ast.Union:
		// Union requires fragments. We generate fragments for up to 2 member types.
		nodes := []*selNode{
			{
				Fragments: make(map[string][]*selNode),
			},
		}
		memberTypes := def.Types
		if len(memberTypes) > 2 {
			memberTypes = memberTypes[:2]
		}
		for _, mt := range memberTypes {
			mtDef := schema.Types[mt]
			if mtDef == nil {
				continue
			}
			child := buildObjectSelection(schema, mtDef, selectionOptions{
				MaxDepth:        opts.MaxDepth - 1,
				MaxScalarFields: opts.MaxScalarFields,
				MaxObjectFields: opts.MaxObjectFields,
			}, visited)
			nodes[0].Fragments[mt] = child
		}
		return nodes
	default:
		return nil
	}
}

func buildObjectSelection(schema *ast.Schema, def *ast.Definition, opts selectionOptions, visited map[string]bool) []*selNode {
	if def == nil {
		return nil
	}

	// 1) scalar/enum fields
	var scalars []*selNode
	for _, f := range def.Fields {
		if f == nil {
			continue
		}
		bt := getBaseTypeName(f.Type)
		ft := schema.Types[bt]
		if ft == nil {
			continue
		}
		if ft.Kind == ast.Scalar || ft.Kind == ast.Enum {
			scalars = append(scalars, &selNode{
				FieldName:  f.Name,
				IsLeaf:     true,
				ReturnType: f.Type,
			})
			if len(scalars) >= opts.MaxScalarFields {
				break
			}
		}
	}

	// 2) a small number of nested object fields
	var objects []*selNode
	if opts.MaxDepth > 1 && opts.MaxObjectFields > 0 {
		for _, f := range def.Fields {
			if f == nil {
				continue
			}
			bt := getBaseTypeName(f.Type)
			ft := schema.Types[bt]
			if ft == nil {
				continue
			}
			if ft.Kind == ast.Object || ft.Kind == ast.Interface || ft.Kind == ast.Union {
				child := buildSelectionTree(schema, f.Type, selectionOptions{
					MaxDepth:        opts.MaxDepth - 1,
					MaxScalarFields: opts.MaxScalarFields,
					MaxObjectFields: opts.MaxObjectFields,
				}, visited)

				objects = append(objects, &selNode{
					FieldName:  f.Name,
					Children:   child,
					IsLeaf:     len(child) == 0, // union fragment node might be 1 element with fragments
					ReturnType: f.Type,
				})
				if len(objects) >= opts.MaxObjectFields {
					break
				}
			}
		}
	}

	return append(scalars, objects...)
}

// renderSelectionSet renders the selection set for the request.
// indentSpaces is the indentation before field names (e.g., 2 for "  field").
func renderSelectionSet(nodes []*selNode, indentSpaces int) string {
	if len(nodes) == 0 {
		return ""
	}

	var b strings.Builder
	indent := strings.Repeat(" ", indentSpaces)

	for i, n := range nodes {
		if n == nil {
			continue
		}
		if i > 0 {
			b.WriteString("\n")
		}

		// Union fragment holder node
		if n.FieldName == "" && len(n.Fragments) > 0 {
			// Sort fragment keys for stable output
			keys := make([]string, 0, len(n.Fragments))
			for k := range n.Fragments {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for ki, k := range keys {
				if ki > 0 {
					b.WriteString("\n")
				}
				b.WriteString(indent)
				b.WriteString("... on ")
				b.WriteString(k)
				b.WriteString(" {\n")
				child := renderSelectionSet(n.Fragments[k], indentSpaces+2)
				if strings.TrimSpace(child) != "" {
					b.WriteString(child)
					b.WriteString("\n")
				}
				b.WriteString(indent)
				b.WriteString("}")
			}
			continue
		}

		// Normal field
		b.WriteString(indent)
		b.WriteString(n.FieldName)

		// Leaf: scalar/enum
		if n.IsLeaf || len(n.Children) == 0 {
			continue
		}

		child := renderSelectionSet(n.Children, indentSpaces+2)
		if strings.TrimSpace(child) == "" {
			continue
		}

		b.WriteString(" {\n")
		b.WriteString(child)
		b.WriteString("\n")
		b.WriteString(indent)
		b.WriteString("}")
	}

	return b.String()
}

/*
Variables (UI hint)

We emit a JSON object for GraphQL variables payload, using schema to
fill placeholder values for scalars/enums and shallow input objects.
*/

func buildVariablesJSON(schema *ast.Schema, args ast.ArgumentDefinitionList, indentSpaces int) string {
	if len(args) == 0 {
		return "{}"
	}
	m := make(map[string]any, len(args))
	for _, a := range args {
		m[a.Name] = exampleJSONValueForType(schema, a.Type, 2) // depth limit for inputs
	}

	// Pretty JSON with deterministic key order via MarshalIndent on a map isn't guaranteed.
	// We'll build manually with sorted keys for stable output.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	indent := strings.Repeat(" ", indentSpaces)
	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		v := m[k]
		raw, _ := json.Marshal(v)
		b.WriteString(indent)
		b.WriteString(fmt.Sprintf("%q: %s", k, string(raw)))
		if i < len(keys)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String()
}

func exampleJSONValueForType(schema *ast.Schema, t *ast.Type, depth int) any {
	if t == nil {
		return nil
	}
	if t.Elem != nil {
		// list
		return []any{exampleJSONValueForType(schema, t.Elem, depth)}
	}

	name := t.NamedType
	switch name {
	case "String", "ID":
		return "example"
	case "Int":
		return 123
	case "Float":
		return 123.45
	case "Boolean":
		return true
	}

	if schema == nil {
		return map[string]any{}
	}
	def := schema.Types[name]
	if def == nil {
		return map[string]any{}
	}

	// Enum: choose first value if available
	if def.Kind == ast.Enum && len(def.EnumValues) > 0 {
		return def.EnumValues[0].Name
	}

	// Input object: shallow expansion
	if def.Kind == ast.InputObject && depth > 0 {
		obj := map[string]any{}
		for _, f := range def.Fields {
			if f == nil {
				continue
			}
			obj[f.Name] = exampleJSONValueForType(schema, f.Type, depth-1)
		}
		return obj
	}

	// Default for complex types
	return map[string]any{}
}

/*
Request rendering

Important: scalar returns must NOT include a selection set.
*/

func renderRequestExample(kind, fieldName string, args ast.ArgumentDefinitionList, selectionSet string) string {
	opKind := strings.ToLower(kind)
	opName := toPascal(kind) + toPascal(fieldName)

	varDefs := renderVarDefs(args)
	argCalls := renderArgCalls(args)

	var b strings.Builder
	b.WriteString(opKind)
	b.WriteString(" ")
	b.WriteString(opName)
	if varDefs != "" {
		b.WriteString(varDefs)
	}
	b.WriteString(" {\n  ")
	b.WriteString(fieldName)
	if argCalls != "" {
		b.WriteString(argCalls)
	}

	trimSel := strings.TrimSpace(selectionSet)
	if trimSel != "" {
		b.WriteString(" {\n")
		// selectionSet already includes indentation starting at 2 spaces
		b.WriteString(selectionSet)
		b.WriteString("\n  }")
	}

	b.WriteString("\n}")
	return b.String()
}

func renderVarDefs(args ast.ArgumentDefinitionList) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for _, a := range args {
		parts = append(parts, fmt.Sprintf("$%s: %s", a.Name, formatType(a.Type)))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func renderArgCalls(args ast.ArgumentDefinitionList) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for _, a := range args {
		parts = append(parts, fmt.Sprintf("%s: $%s", a.Name, a.Name))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

/*
Response rendering

We mirror the selection tree, emitting placeholder values.
If no selection is possible (scalar/enum), we emit scalar placeholder directly.
*/

func renderResponseExample(schema *ast.Schema, rootFieldName string, returnType *ast.Type, sel []*selNode, indentSpaces int) string {
	val := exampleResponseValue(schema, returnType, sel, 3)

	// Stable JSON output
	outer := map[string]any{
		"data": map[string]any{
			rootFieldName: val,
		},
	}
	raw, _ := json.MarshalIndent(outer, "", strings.Repeat(" ", indentSpaces))
	return string(raw)
}

func exampleResponseValue(schema *ast.Schema, t *ast.Type, sel []*selNode, depth int) any {
	if t == nil {
		return nil
	}
	if t.Elem != nil {
		// list response
		return []any{exampleResponseValue(schema, t.Elem, sel, depth)}
	}

	base := t.NamedType
	// Scalars
	switch base {
	case "String", "ID":
		return "example"
	case "Int":
		return 123
	case "Float":
		return 123.45
	case "Boolean":
		return true
	}

	if schema == nil {
		return map[string]any{}
	}
	def := schema.Types[base]
	if def == nil {
		return map[string]any{}
	}

	if def.Kind == ast.Enum {
		if len(def.EnumValues) > 0 {
			return def.EnumValues[0].Name
		}
		return "ENUM_VALUE"
	}

	// Objects/interfaces/unions: use selection
	if depth <= 0 {
		return map[string]any{}
	}

	// If selection is empty, return empty object (shape unknown)
	if len(sel) == 0 {
		return map[string]any{}
	}

	// Union selection node is represented as a single node with fragments.
	// We'll render the first fragment shape as example.
	if len(sel) == 1 && sel[0] != nil && sel[0].FieldName == "" && len(sel[0].Fragments) > 0 {
		keys := make([]string, 0, len(sel[0].Fragments))
		for k := range sel[0].Fragments {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			return map[string]any{}
		}
		frag := sel[0].Fragments[keys[0]]
		return objectFromSelection(schema, base, frag, depth-1)
	}

	return objectFromSelection(schema, base, sel, depth-1)
}

func objectFromSelection(schema *ast.Schema, baseType string, sel []*selNode, depth int) map[string]any {
	obj := map[string]any{}

	def := schema.Types[baseType]
	if def == nil {
		return obj
	}

	// Index fields for O(1) lookups
	fieldIndex := map[string]*ast.FieldDefinition{}
	for _, f := range def.Fields {
		if f != nil {
			fieldIndex[f.Name] = f
		}
	}

	for _, n := range sel {
		if n == nil || n.FieldName == "" {
			continue
		}
		fd := fieldIndex[n.FieldName]
		if fd == nil {
			// If selection includes a field not present on this type (possible with interfaces),
			// we skip gracefully.
			continue
		}
		obj[n.FieldName] = exampleResponseValue(schema, fd.Type, n.Children, depth)
	}
	return obj
}

/*
Utilities
*/

// toPascal makes a best-effort PascalCase identifier from GraphQL names.
func toPascal(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Split by non-alphanumerics and camel humps (simple)
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	parts := re.Split(s, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	return b.String()
}
