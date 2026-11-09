package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"go/types"
	"log"
	"os"
	"strings"
	"text/template"
	"unicode"

	"golang.org/x/tools/go/packages"
)

//go:embed template.go.tmpl
var templateContent string

//go:embed template_test.go.tmpl
var testTemplateContent string

// Field kinds of a field whose type contains a union, directly or nested.
const (
	kindUnion      = "union"
	kindUnionList  = "unionList"
	kindStructPtr  = "structPtr"
	kindStructList = "structList"
)

// Kinds of the Value field of a union member. A member whose Value contains
// no union has an empty ValueKind and is decoded by encoding/json.
const (
	valueStruct = "struct"
	valueUnion  = "union"
)

// BearingField holds metadata about a struct field whose type contains a
// union. ToFunc and FromFunc name the generated functions that convert the
// field value from and to its JSON form.
type BearingField struct {
	FieldName string
	JSONKey   string
	Kind      string
	TypeName  string
	ToFunc    string
	FromFunc  string
	Members   []MemberInfo
}

// IsStructPtr reports whether the field is a pointer to a struct containing a union.
func (f BearingField) IsStructPtr() bool { return f.Kind == kindStructPtr }

// IsUnionList reports whether the field is a slice of a union.
func (f BearingField) IsUnionList() bool { return f.Kind == kindUnionList }

// UnionTypeInfo holds metadata about a union interface type. One pair of
// convertTo/convertFrom functions is generated per union type.
type UnionTypeInfo struct {
	TypeName string
	Members  []MemberInfo
}

// StructTypeInfo holds metadata about a struct type that contains a union.
// One pair of decode/encode functions is generated per such struct type.
type StructTypeInfo struct {
	TypeName string
	Fields   []BearingField
	MapPaths []string
}

// ListInfo describes a slice element type that needs list converters.
type ListInfo struct {
	TypeName string
	IsUnion  bool
}

// MemberInfo holds metadata about a concrete union member
type MemberInfo struct {
	TypeName       string      // e.g., "AgentRuntimeArtifactMemberContainerConfiguration"
	JSONKey        string      // e.g., "containerConfiguration"
	JSONTag        string      // e.g., "`json:\"containerConfiguration\"`"
	ValueType      string      // e.g., "github.com/.../types.ContainerConfiguration"
	ValueTypeShort string      // e.g., "types.ContainerConfiguration"
	ValueFields    []FieldInfo // Fields of the Value struct (if struct)
	IsDirectValue  bool        // True if Value is not a struct (e.g., []string)
	ValueKind      string
	ValueTypeName  string
	MapPaths       []string
	TestInputJSON  string // Generated test input JSON (e.g., `{"containerConfiguration": {...}}`)
	TestExpected   string // Generated expected Go value (e.g., `&types.XXX{Value: ...}`)
}

// ValueIsStruct reports whether the member value is a struct containing a union.
func (m MemberInfo) ValueIsStruct() bool { return m.ValueKind == valueStruct }

// ValueIsUnion reports whether the member value is itself a union.
func (m MemberInfo) ValueIsUnion() bool { return m.ValueKind == valueUnion }

// FieldInfo holds metadata about a struct field
type FieldInfo struct {
	Name      string // Field name
	Type      string // Field type as string
	IsPointer bool   // True if it's a pointer type
}

func main() {
	// Load the bedrockagentcorecontrol package and its types subpackage.
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax | packages.NeedName,
	}
	pkgs, err := packages.Load(cfg,
		"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol",
		"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types",
	)
	if err != nil {
		log.Fatalf("failed to load packages: %v", err)
	}
	for _, pkg := range pkgs {
		for _, err := range pkg.Errors {
			log.Printf("Package error in %s: %v", pkg.PkgPath, err)
		}
	}
	if packages.PrintErrors(pkgs) > 0 {
		log.Fatalf("package errors encountered")
	}

	var basePkg *packages.Package
	var typesPkg *packages.Package
	for _, p := range pkgs {
		log.Printf("Loaded package: %s", p.PkgPath)
		switch p.PkgPath {
		case "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol":
			basePkg = p
		case "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types":
			typesPkg = p
		}
	}
	if basePkg == nil || typesPkg == nil {
		log.Fatalf("required packages not found (basePkg=%v, typesPkg=%v)", basePkg != nil, typesPkg != nil)
	}

	// Locate the target struct (e.g. CreateAgentRuntimeInput) in the base package.
	obj := basePkg.Types.Scope().Lookup("CreateAgentRuntimeInput")
	if obj == nil {
		log.Fatalf("CreateAgentRuntimeInput not found in package")
	}
	struc, ok := obj.Type().Underlying().(*types.Struct)
	if !ok {
		log.Fatalf("CreateAgentRuntimeInput is not a struct")
	}

	c := newCollector(typesPkg.Types)
	var unionFields []BearingField
	for i := 0; i < struc.NumFields(); i++ {
		field := struc.Field(i)
		bf, ok, err := c.bearingField(field, field.Name())
		if err != nil {
			log.Fatalf("unsupported union shape: %v", err)
		}
		if !ok {
			continue
		}
		if bf.Kind != kindUnion && bf.Kind != kindUnionList {
			log.Fatalf("unsupported union shape: CreateAgentRuntimeInput.%s holds a union inside a struct, which codegen does not support at the top level", field.Name())
		}
		bf.Members = c.unions[c.unionIdx[bf.TypeName]].Members
		unionFields = append(unionFields, bf)
	}

	mapPathsOf := func(name string) []string {
		obj := basePkg.Types.Scope().Lookup(name)
		if obj == nil {
			log.Fatalf("%s not found in package", name)
		}
		return mapPaths(obj.Type(), "$", map[types.Type]bool{})
	}
	data := map[string]any{
		"UnionFields":    unionFields,
		"UnionTypes":     c.unions,
		"StructTypes":    c.structs,
		"Lists":          c.lists,
		"NeedsStrings":   len(c.structs) > 0,
		"CreateMapPaths": mapPathsOf("CreateAgentRuntimeInput"),
		"GetMapPaths":    mapPathsOf("GetAgentRuntimeOutput"),
		"UpdateMapPaths": mapPathsOf("UpdateAgentRuntimeInput"),
	}

	// Generate code
	if err := generateCode(data); err != nil {
		log.Fatalf("failed to generate code: %v", err)
	}

	// Generate tests
	if err := generateTests(data); err != nil {
		log.Fatalf("failed to generate tests: %v", err)
	}

	log.Printf("Successfully generated aws.gen.go and aws.gen_test.go")
}

func generateCode(data map[string]any) error {
	// Parse template
	tmpl, err := template.New("codegen").Option("missingkey=error").Funcs(template.FuncMap{
		"toFieldName": func(s string) string {
			return toFieldName(toPascalCase(s))
		},
	}).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Format generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("Error: failed to format generated code: %v", err)
		return fmt.Errorf("failed to format generated code: %w", err)
	}

	// Write to file
	outputPath := "aws.gen.go"
	if err := os.WriteFile(outputPath, formatted, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func generateTests(data map[string]any) error {
	// Parse template
	tmpl, err := template.New("codegen_test").Option("missingkey=error").Funcs(template.FuncMap{
		"toFieldName": func(s string) string {
			return toFieldName(toPascalCase(s))
		},
		"toLowerCamel": toLowerCamelCase,
	}).Parse(testTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse test template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute test template: %w", err)
	}

	// Format generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("Error: failed to format generated test code: %v", err)
		log.Printf("Generated code (first 2000 chars):\n%s", string(buf.Bytes()[:min(2000, len(buf.Bytes()))]))
		return fmt.Errorf("failed to format generated test code: %w", err)
	}

	// Write to file
	outputPath := "aws.gen_test.go"
	if err := os.WriteFile(outputPath, formatted, 0600); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	return nil
}

func extractMemberInfo(typeName string, strct *types.Struct, samples map[string]unionSample) MemberInfo {
	jsonKey := inferJSONKey(typeName)
	member := MemberInfo{
		TypeName: typeName,
		JSONKey:  jsonKey,
		JSONTag:  fmt.Sprintf("`json:\"%s\"`", jsonKey),
	}

	// Find the "Value" field
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		if field.Name() == "Value" {
			member.ValueType = field.Type().String()
			member.ValueTypeShort = shortTypeName(field.Type().String())

			// Check if Value is a struct
			if valueStruct, ok := field.Type().Underlying().(*types.Struct); ok {
				member.IsDirectValue = false
				// Extract value fields (skip noSmithyDocumentSerde)
				for j := 0; j < valueStruct.NumFields(); j++ {
					vField := valueStruct.Field(j)
					if vField.Name() == "noSmithyDocumentSerde" {
						continue
					}
					fieldInfo := FieldInfo{
						Name: vField.Name(),
						Type: vField.Type().String(),
					}
					// Check if it's a pointer type
					if _, ok := vField.Type().(*types.Pointer); ok {
						fieldInfo.IsPointer = true
					}
					member.ValueFields = append(member.ValueFields, fieldInfo)
				}
				// Generate test values for struct
				member.TestInputJSON = generateStructTestInputJSON(jsonKey, valueStruct, samples)
				member.TestExpected = generateStructTestExpected(typeName, member.ValueTypeShort, valueStruct, samples)
			} else {
				// Direct value (e.g., []string)
				member.IsDirectValue = true
				// Generate test values for direct value
				member.TestInputJSON = generateDirectTestInputJSON(jsonKey, field.Type(), samples)
				member.TestExpected = generateDirectTestExpected(typeName, field.Type(), samples)
			}
			break
		}
	}

	return member
}

// shortTypeName converts full package path to short form
// e.g., "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types.Foo" -> "types.Foo"
func shortTypeName(fullType string) string {
	return strings.ReplaceAll(fullType, "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types.", "types.")
}

// inferJSONKey derives the JSON key from the Member type name
// e.g., "AgentRuntimeArtifactMemberContainerConfiguration" -> "containerConfiguration"
// e.g., "RequestHeaderConfigurationMemberRequestHeaderAllowlist" -> "allowList"
func inferJSONKey(typeName string) string {
	// Remove "Member" prefix pattern
	// Pattern: <Interface>Member<Variant>
	parts := strings.Split(typeName, "Member")
	if len(parts) != 2 {
		return toLowerCamelCase(typeName)
	}

	interfacePrefix := parts[0]
	variantName := parts[1]

	// Remove "Configuration" suffix from interface prefix for comparison
	// e.g., "RequestHeaderConfiguration" -> "RequestHeader"
	interfacePrefixBase := strings.TrimSuffix(interfacePrefix, "Configuration")

	// Special case: if variant name starts with the interface prefix base, remove duplication
	// e.g., "RequestHeader" + "RequestHeaderAllowlist" -> "Allowlist"
	variantName = strings.TrimPrefix(variantName, interfacePrefixBase)

	// Convert to field name (e.g., "Allowlist" -> "AllowList")
	fieldName := toFieldName(variantName)

	return toLowerCamelCase(fieldName)
}

// toFieldName converts compound words to proper field names
// e.g., "Allowlist" -> "AllowList"
func toFieldName(s string) string {
	// Common compound words that should be split
	replacements := map[string]string{
		"Allowlist": "AllowList",
		"Blocklist": "BlockList",
		// Add more as needed
	}

	if replacement, ok := replacements[s]; ok {
		return replacement
	}
	return s
}

// toLowerCamelCase converts PascalCase to lowerCamelCase
func toLowerCamelCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// toPascalCase converts lowerCamelCase to PascalCase
func toPascalCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// generateDirectTestInputJSON generates test input JSON for direct value types (e.g., []string)
func generateDirectTestInputJSON(jsonKey string, t types.Type, samples map[string]unionSample) string {
	value := generateTestValueJSON(t, samples)
	return fmt.Sprintf(`map[string]any{"%s": %s}`, jsonKey, value)
}

// generateDirectTestExpected generates expected Go value for direct value types
func generateDirectTestExpected(typeName string, t types.Type, samples map[string]unionSample) string {
	value := generateTestValueGo(t, samples)
	return fmt.Sprintf(`&types.%s{Value: %s}`, typeName, value)
}

// generateStructTestInputJSON generates test input JSON for struct types
func generateStructTestInputJSON(jsonKey string, strct *types.Struct, samples map[string]unionSample) string {
	var fields []string
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		if field.Name() == "noSmithyDocumentSerde" {
			continue
		}
		fieldNameLower := toLowerCamelCase(field.Name())
		value := generateTestValueJSON(field.Type(), samples)
		fields = append(fields, fmt.Sprintf(`"%s": %s`, fieldNameLower, value))
	}
	return fmt.Sprintf(`map[string]any{"%s": map[string]any{%s}}`, jsonKey, strings.Join(fields, ", "))
}

// generateStructTestExpected generates expected Go value for struct types
func generateStructTestExpected(typeName, valueTypeShort string, strct *types.Struct, samples map[string]unionSample) string {
	var fields []string
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		if field.Name() == "noSmithyDocumentSerde" {
			continue
		}
		value := generateTestValueGo(field.Type(), samples)
		fields = append(fields, fmt.Sprintf("%s: %s", field.Name(), value))
	}
	return fmt.Sprintf(`&types.%s{Value: %s{%s}}`, typeName, valueTypeShort, strings.Join(fields, ", "))
}

// generateTestValueJSON generates a test value in JSON-compatible Go syntax
func generateTestValueJSON(t types.Type, samples map[string]unionSample) string {
	// Handle Named types first
	if named, ok := t.(*types.Named); ok {
		// Check if it's an interface (union type)
		if _, ok := named.Underlying().(*types.Interface); ok {
			if sample, ok := samples[named.Obj().Name()]; ok {
				return sample.JSON
			}
			return "nil"
		}
		// Check if it's a struct type
		if _, ok := named.Underlying().(*types.Struct); ok {
			return "map[string]any{}"
		}
		// Check if it's a basic type alias (enum-like string types)
		if basic, ok := named.Underlying().(*types.Basic); ok {
			switch basic.Kind() {
			case types.String:
				// String-based enum - use test string value
				return `"test_value"`
			case types.Int, types.Int32, types.Int64:
				// Int-based enum - use numeric value
				return "123"
			}
		}
	}

	switch u := t.Underlying().(type) {
	case *types.Pointer:
		// For pointers, generate the underlying value (will be wrapped in aws.String etc in Go)
		return generateTestValueJSON(u.Elem(), samples)
	case *types.Slice:
		elem := generateTestValueJSON(u.Elem(), samples)
		return fmt.Sprintf(`[]any{%s, %s}`, elem, elem)
	case *types.Basic:
		switch u.Kind() {
		case types.String:
			return `"test_value"`
		case types.Int, types.Int32, types.Int64:
			return "123"
		case types.Bool:
			return "true"
		}
	case *types.Interface:
		// Skip interface/union types in test generation (too complex)
		return "nil"
	case *types.Map:
		// Match generateTestValueGo's fallback so Input/Expected stay consistent
		return "nil"
	}
	return "nil"
}

// generateTestValueGo generates a test value in Go syntax
func generateTestValueGo(t types.Type, samples map[string]unionSample) string {
	// Handle Named types (enums, structs, etc.) before checking Underlying
	if named, ok := t.(*types.Named); ok {
		// Check if it's an interface (union type)
		if _, ok := named.Underlying().(*types.Interface); ok {
			if sample, ok := samples[named.Obj().Name()]; ok {
				return sample.Go
			}
			return "nil"
		}
		// Check if it's a struct type
		if _, ok := named.Underlying().(*types.Struct); ok {
			// For complex struct types, use empty struct literal
			return typeStringShort(t) + "{}"
		}
		// Check if it's a basic type alias (enum-like string types)
		if basic, ok := named.Underlying().(*types.Basic); ok {
			switch basic.Kind() {
			case types.String:
				// String-based enum - use test value for consistency with JSON
				return typeStringShort(t) + `("test_value")`
			case types.Int, types.Int32, types.Int64:
				// Int-based enum - use test value
				return typeStringShort(t) + "(123)"
			}
		}
	}

	switch u := t.Underlying().(type) {
	case *types.Pointer:
		elem := generateTestValueGo(u.Elem(), samples)
		// Use aws.String, aws.Int32, etc. for AWS SDK types
		if basic, ok := u.Elem().Underlying().(*types.Basic); ok {
			switch basic.Kind() {
			case types.String:
				return `aws.String("test_value")`
			case types.Int32:
				return `aws.Int32(123)`
			case types.Int64:
				return `aws.Int64(123)`
			case types.Bool:
				return `aws.Bool(true)`
			}
		}
		return "&(" + elem + ")"
	case *types.Slice:
		elemType := typeStringShort(u.Elem())
		elem := generateTestValueGo(u.Elem(), samples)
		return fmt.Sprintf(`[]%s{%s, %s}`, elemType, elem, elem)
	case *types.Basic:
		switch u.Kind() {
		case types.String:
			return `"test_value"`
		case types.Int, types.Int32, types.Int64:
			return "123"
		case types.Bool:
			return "true"
		}
	case *types.Interface:
		// Skip interface/union types in test generation (too complex)
		return "nil"
	}
	return "nil"
}

// typeStringShort converts type to short string representation
func typeStringShort(t types.Type) string {
	s := t.String()
	return shortTypeName(s)
}

// unionSample is a sample value of a union type: a JSON-compatible Go literal
// and the Go value it decodes to. It is used as the test value of fields of
// that union type.
type unionSample struct {
	JSON string
	Go   string
}

// collector walks the types reachable from CreateAgentRuntimeInput and
// records each union type and each struct type containing a union once, in
// discovery order.
type collector struct {
	typesPkg   *types.Package
	unions     []UnionTypeInfo
	unionIdx   map[string]int
	structs    []StructTypeInfo
	structIdx  map[string]int
	lists      []ListInfo
	listSeen   map[string]bool
	samples    map[string]unionSample
	bearsMemo  map[types.Type]bool
	inProgress map[types.Type]bool
}

func newCollector(pkg *types.Package) *collector {
	return &collector{
		typesPkg:   pkg,
		unionIdx:   map[string]int{},
		structIdx:  map[string]int{},
		listSeen:   map[string]bool{},
		samples:    map[string]unionSample{},
		bearsMemo:  map[types.Type]bool{},
		inProgress: map[types.Type]bool{},
	}
}

// bears reports whether t contains a union interface at any depth. It fails
// on recursive types because a memoized answer computed inside a cycle could
// be wrong.
func (c *collector) bears(t types.Type) (bool, error) {
	if v, ok := c.bearsMemo[t]; ok {
		return v, nil
	}
	if c.inProgress[t] {
		return false, fmt.Errorf("recursive type %s is not supported by codegen", t)
	}
	c.inProgress[t] = true
	defer delete(c.inProgress, t)

	var result bool
	var err error
	switch u := t.(type) {
	case *types.Pointer:
		result, err = c.bears(u.Elem())
	case *types.Slice:
		result, err = c.bears(u.Elem())
	case *types.Map:
		result, err = c.bears(u.Elem())
	default:
		switch under := t.Underlying().(type) {
		case *types.Interface:
			result = true
		case *types.Struct:
			for i := 0; i < under.NumFields() && !result; i++ {
				f := under.Field(i)
				if !f.Exported() {
					continue
				}
				if result, err = c.bears(f.Type()); err != nil {
					return false, err
				}
			}
		}
	}
	if err != nil {
		return false, err
	}
	c.bearsMemo[t] = result
	return result, nil
}

// bearingField classifies a struct field whose type contains a union and
// registers the types it needs. It returns false for fields without unions
// and an error for shapes the generated code cannot convert.
func (c *collector) bearingField(field *types.Var, path string) (BearingField, bool, error) {
	t := field.Type()
	bears, err := c.bears(t)
	if err != nil || !bears {
		return BearingField{}, false, err
	}
	bf := BearingField{FieldName: field.Name(), JSONKey: toLowerCamelCase(field.Name())}
	switch u := t.(type) {
	case *types.Named:
		if iface, ok := u.Underlying().(*types.Interface); ok {
			name, err := c.union(u, iface, path)
			if err != nil {
				return BearingField{}, false, err
			}
			bf.Kind, bf.TypeName = kindUnion, name
			bf.ToFunc, bf.FromFunc = "convertTo"+name, "convertFrom"+name
			return bf, true, nil
		}
	case *types.Pointer:
		if named, ok := u.Elem().(*types.Named); ok {
			if strct, ok := named.Underlying().(*types.Struct); ok {
				name, err := c.structType(named, strct, path)
				if err != nil {
					return BearingField{}, false, err
				}
				bf.Kind, bf.TypeName = kindStructPtr, name
				bf.ToFunc, bf.FromFunc = "decode"+name, "encode"+name
				return bf, true, nil
			}
		}
	case *types.Slice:
		if named, ok := u.Elem().(*types.Named); ok {
			if iface, ok := named.Underlying().(*types.Interface); ok {
				name, err := c.union(named, iface, path+"[]")
				if err != nil {
					return BearingField{}, false, err
				}
				bf.Kind, bf.TypeName = kindUnionList, name
				bf.ToFunc, bf.FromFunc = "convertTo"+name+"List", "convertFrom"+name+"List"
				c.addList(name, true)
				return bf, true, nil
			}
			if strct, ok := named.Underlying().(*types.Struct); ok {
				name, err := c.structType(named, strct, path+"[]")
				if err != nil {
					return BearingField{}, false, err
				}
				bf.Kind, bf.TypeName = kindStructList, name
				bf.ToFunc, bf.FromFunc = "decode"+name+"List", "encode"+name+"List"
				c.addList(name, false)
				return bf, true, nil
			}
		}
	}
	return BearingField{}, false, fmt.Errorf("%s has type %s, which codegen cannot convert", path, t)
}

func (c *collector) addList(typeName string, isUnion bool) {
	if c.listSeen[typeName] {
		return
	}
	c.listSeen[typeName] = true
	c.lists = append(c.lists, ListInfo{TypeName: typeName, IsUnion: isUnion})
}

// union registers a union type with its members and returns its name.
func (c *collector) union(named *types.Named, iface *types.Interface, path string) (string, error) {
	typeName := named.Obj().Name()
	if _, ok := c.unionIdx[typeName]; ok {
		return typeName, nil
	}
	idx := len(c.unions)
	c.unionIdx[typeName] = idx
	c.unions = append(c.unions, UnionTypeInfo{TypeName: typeName})

	var members []MemberInfo
	for _, name := range c.typesPkg.Scope().Names() {
		tn, ok := c.typesPkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		t := tn.Type()
		pt := types.NewPointer(t)
		if !types.AssignableTo(pt, named) && !types.Implements(pt, iface) {
			continue
		}
		if tn.Name() == "UnknownUnionMember" || types.Identical(t, named) {
			continue
		}
		strct, ok := t.Underlying().(*types.Struct)
		if !ok {
			return "", fmt.Errorf("member %s of union %s at %s is not a struct", tn.Name(), typeName, path)
		}
		valueKind, valueTypeName, err := c.memberValue(tn.Name(), strct, path+"<"+tn.Name()+">")
		if err != nil {
			return "", err
		}
		member := extractMemberInfo(tn.Name(), strct, c.samples)
		member.ValueKind, member.ValueTypeName = valueKind, valueTypeName
		if valueKind == "" && !member.IsDirectValue {
			for i := 0; i < strct.NumFields(); i++ {
				if strct.Field(i).Name() == "Value" {
					member.MapPaths = mapPaths(strct.Field(i).Type(), "$", map[types.Type]bool{})
				}
			}
		}
		members = append(members, member)
	}
	if len(members) == 0 {
		return "", fmt.Errorf("union %s at %s has no members", typeName, path)
	}
	c.unions[idx].Members = members
	c.samples[typeName] = unionSample{JSON: members[0].TestInputJSON, Go: members[0].TestExpected}
	return typeName, nil
}

// memberValue classifies the Value field of a union member and registers the
// types it needs. It returns an empty kind for a Value without unions.
func (c *collector) memberValue(memberName string, strct *types.Struct, path string) (string, string, error) {
	for i := 0; i < strct.NumFields(); i++ {
		f := strct.Field(i)
		if f.Name() != "Value" {
			continue
		}
		vt := f.Type()
		bears, err := c.bears(vt)
		if err != nil || !bears {
			return "", "", err
		}
		if named, ok := vt.(*types.Named); ok {
			switch under := named.Underlying().(type) {
			case *types.Interface:
				name, err := c.union(named, under, path)
				return valueUnion, name, err
			case *types.Struct:
				name, err := c.structType(named, under, path)
				return valueStruct, name, err
			}
		}
		return "", "", fmt.Errorf("%s.Value has type %s, which codegen cannot convert", path, vt)
	}
	return "", "", fmt.Errorf("union member %s has no Value field", memberName)
}

// structType registers a struct type containing a union and returns its name.
func (c *collector) structType(named *types.Named, strct *types.Struct, path string) (string, error) {
	typeName := named.Obj().Name()
	if _, ok := c.structIdx[typeName]; ok {
		return typeName, nil
	}
	idx := len(c.structs)
	c.structIdx[typeName] = idx
	c.structs = append(c.structs, StructTypeInfo{TypeName: typeName})

	var fields []BearingField
	for i := 0; i < strct.NumFields(); i++ {
		f := strct.Field(i)
		if !f.Exported() {
			continue
		}
		bf, ok, err := c.bearingField(f, path+"."+f.Name())
		if err != nil {
			return "", err
		}
		if ok {
			fields = append(fields, bf)
		}
	}
	c.structs[idx].Fields = fields
	c.structs[idx].MapPaths = mapPaths(named, "$", map[types.Type]bool{})
	return typeName, nil
}

// mapPaths returns the JSON path patterns of the keys of every map reachable
// from t without crossing a union. Map keys are data such as tag names, so
// marshalJSON and unmarshalJSON must not change their case.
func mapPaths(t types.Type, path string, seen map[types.Type]bool) []string {
	switch u := t.(type) {
	case *types.Pointer:
		return mapPaths(u.Elem(), path, seen)
	case *types.Slice:
		return mapPaths(u.Elem(), path+".*", seen)
	case *types.Map:
		return append([]string{path + ".*"}, mapPaths(u.Elem(), path+".*", seen)...)
	}
	strct, ok := t.Underlying().(*types.Struct)
	if !ok || seen[t] {
		return nil
	}
	seen[t] = true
	defer delete(seen, t)
	var paths []string
	for i := 0; i < strct.NumFields(); i++ {
		f := strct.Field(i)
		if !f.Exported() {
			continue
		}
		paths = append(paths, mapPaths(f.Type(), path+"."+toLowerCamelCase(f.Name()), seen)...)
	}
	return paths
}
