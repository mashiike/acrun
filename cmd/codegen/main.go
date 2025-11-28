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

// UnionFieldInfo holds metadata about a union type field
type UnionFieldInfo struct {
	FieldName     string       // e.g., "AgentRuntimeArtifact"
	InterfaceType string       // e.g., "types.AgentRuntimeArtifact"
	Members       []MemberInfo // List of concrete member types
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
	TestInputJSON  string      // Generated test input JSON (e.g., `{"containerConfiguration": {...}}`)
	TestExpected   string      // Generated expected Go value (e.g., `&types.XXX{Value: ...}`)
}

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

	// Collect union field information
	var unionFields []UnionFieldInfo

	// Iterate over the struct's fields and identify interface types.
	for i := 0; i < struc.NumFields(); i++ {
		field := struc.Field(i)
		fieldType := field.Type()
		iface, ok := fieldType.Underlying().(*types.Interface)
		if !ok {
			// Not an interface field; skip.
			continue
		}

		unionInfo := UnionFieldInfo{
			FieldName:     field.Name(),
			InterfaceType: fieldType.String(),
			Members:       []MemberInfo{},
		}

		// Collect the concrete types in the types package that implement this interface.
		for _, name := range typesPkg.Types.Scope().Names() {
			obj := typesPkg.Types.Scope().Lookup(name)
			tn, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}
			// Check whether the named type or its pointer implements the interface.
			t := tn.Type()
			// Try pointer type first; many union member wrappers are pointer receivers.
			pt := types.NewPointer(t)
			if types.AssignableTo(pt, fieldType) || types.Implements(pt, iface) {
				// Skip UnknownUnionMember
				if tn.Name() == "UnknownUnionMember" {
					continue
				}
				// Skip the interface itself
				if types.Identical(t, fieldType) {
					continue
				}

				// Extract Member type metadata
				if strct, ok := t.Underlying().(*types.Struct); ok {
					memberInfo := extractMemberInfo(tn.Name(), strct)
					unionInfo.Members = append(unionInfo.Members, memberInfo)
				}
			}
		}

		unionFields = append(unionFields, unionInfo)
	}

	// Generate code
	if err := generateCode(unionFields); err != nil {
		log.Fatalf("failed to generate code: %v", err)
	}

	// Generate tests
	if err := generateTests(unionFields); err != nil {
		log.Fatalf("failed to generate tests: %v", err)
	}

	log.Printf("Successfully generated aws.gen.go and aws.gen_test.go")
}

func generateCode(unionFields []UnionFieldInfo) error {
	// Parse template
	tmpl, err := template.New("codegen").Funcs(template.FuncMap{
		"toFieldName": func(s string) string {
			return toFieldName(toPascalCase(s))
		},
		"toLowerCamel": toLowerCamelCase,
	}).Parse(templateContent)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	data := map[string]interface{}{
		"UnionFields": unionFields,
	}
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

func generateTests(unionFields []UnionFieldInfo) error {
	// Parse template
	tmpl, err := template.New("codegen_test").Funcs(template.FuncMap{
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
	data := map[string]interface{}{
		"UnionFields": unionFields,
	}
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

func extractMemberInfo(typeName string, strct *types.Struct) MemberInfo {
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
				member.TestInputJSON = generateStructTestInputJSON(jsonKey, valueStruct)
				member.TestExpected = generateStructTestExpected(typeName, member.ValueTypeShort, valueStruct)
			} else {
				// Direct value (e.g., []string)
				member.IsDirectValue = true
				// Generate test values for direct value
				member.TestInputJSON = generateDirectTestInputJSON(jsonKey, field.Type())
				member.TestExpected = generateDirectTestExpected(typeName, field.Type())
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
func generateDirectTestInputJSON(jsonKey string, t types.Type) string {
	value := generateTestValueJSON(t)
	return fmt.Sprintf(`map[string]any{"%s": %s}`, jsonKey, value)
}

// generateDirectTestExpected generates expected Go value for direct value types
func generateDirectTestExpected(typeName string, t types.Type) string {
	value := generateTestValueGo(t)
	return fmt.Sprintf(`&types.%s{Value: %s}`, typeName, value)
}

// generateStructTestInputJSON generates test input JSON for struct types
func generateStructTestInputJSON(jsonKey string, strct *types.Struct) string {
	var fields []string
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		if field.Name() == "noSmithyDocumentSerde" {
			continue
		}
		fieldNameLower := toLowerCamelCase(field.Name())
		value := generateTestValueJSON(field.Type())
		fields = append(fields, fmt.Sprintf(`"%s": %s`, fieldNameLower, value))
	}
	return fmt.Sprintf(`map[string]any{"%s": map[string]any{%s}}`, jsonKey, strings.Join(fields, ", "))
}

// generateStructTestExpected generates expected Go value for struct types
func generateStructTestExpected(typeName, valueTypeShort string, strct *types.Struct) string {
	var fields []string
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		if field.Name() == "noSmithyDocumentSerde" {
			continue
		}
		value := generateTestValueGo(field.Type())
		fields = append(fields, fmt.Sprintf("%s: %s", field.Name(), value))
	}
	return fmt.Sprintf(`&types.%s{Value: %s{%s}}`, typeName, valueTypeShort, strings.Join(fields, ", "))
}

// generateTestValueJSON generates a test value in JSON-compatible Go syntax
func generateTestValueJSON(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		// For pointers, generate the underlying value (will be wrapped in aws.String etc in Go)
		return generateTestValueJSON(u.Elem())
	case *types.Slice:
		elem := generateTestValueJSON(u.Elem())
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
	}
	return `"unknown"`
}

// generateTestValueGo generates a test value in Go syntax
func generateTestValueGo(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		elem := generateTestValueGo(u.Elem())
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
		elem := generateTestValueGo(u.Elem())
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
	case *types.Named:
		// Check if it's an interface (union type)
		if _, ok := t.Underlying().(*types.Interface); ok {
			return "nil"
		}
		// Handle enum types - use zero value
		return typeStringShort(t) + "(0)"
	}
	return "nil"
}

// typeStringShort converts type to short string representation
func typeStringShort(t types.Type) string {
	s := t.String()
	return shortTypeName(s)
}
