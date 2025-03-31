package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"text/template"
)

// TypeInfo holds metadata about the type we're generating
type TypeInfo struct {
	TypeName  string // The name of the generated type (e.g., "PaymentMethod")
	ValueType string // The underlying type (e.g., "string" or "int")
	Package   string // The package name for the generated code
}

// EnumValue represents a single enum value with its constant name and display name
type EnumValue struct {
	ConstName   string      // The constant name in PascalCase
	Value       interface{} // The internal value (string or int)
	DisplayName string      // The human-readable display name
}

// TemplateData holds all the data needed for code generation
type TemplateData struct {
	Type   TypeInfo
	Values []EnumValue
}

// MapInfo stores information about a discovered map in the source file
type MapInfo struct {
	Name       string            // The variable name of the map
	KeyType    string            // The type of the map keys
	ValueType  string            // The type of the map values
	Definition *ast.CompositeLit // The AST node containing the map definition
}

// commentInfo holds information extracted from the generate comment
type commentInfo struct {
	sourcePackage string // The package name of the source file
	outputPackage string // The desired package name for generated code
}

// Template for generating enum types
const codeTemplate = `// Code generated by enum-generator; DO NOT EDIT.

package {{ .Type.Package }}

import (
	"fmt"
)

// {{ .Type.TypeName }} represents a strongly typed enum-like value.
// The internal value is kept private to maintain type safety and prevent
// invalid values from being created outside this package.
type {{ .Type.TypeName }} struct {
	value {{ .Type.ValueType }}
}

// Pre-defined constants for {{ .Type.TypeName }}.
// The constant names are derived from the display names for better readability.
var (
	{{- range .Values }}
	{{ $.Type.TypeName }}{{ .ConstName }} = {{ $.Type.TypeName }}{value: {{ printf "%#v" .Value }}}
	{{- end }}
)

// displayNames maps internal values to their human-readable representations.
var displayNames = map[{{ .Type.ValueType }}]string{
	{{- range .Values }}
	{{ printf "%#v" .Value }}: {{ printf "%#v" .DisplayName }},
	{{- end }}
}

// String returns the human-readable representation of the value.
func (e {{ .Type.TypeName }}) String() string {
	if name, ok := displayNames[e.value]; ok {
		return name
	}
	return "Unknown"
}

// Parse{{ .Type.TypeName }} converts a {{ .Type.ValueType }} into a {{ .Type.TypeName }}.
// It returns an error if the input doesn't match any known value.
func Parse{{ .Type.TypeName }}(v {{ .Type.ValueType }}) ({{ .Type.TypeName }}, error) {
	if _, ok := displayNames[v]; ok {
		return {{ .Type.TypeName }}{value: v}, nil
	}
	return {{ .Type.TypeName }}{}, fmt.Errorf("invalid {{ .Type.TypeName }}: %v", v)
}

// IsValid returns true if this represents a known value.
func (e {{ .Type.TypeName }}) IsValid() bool {
	_, ok := displayNames[e.value]
	return ok
}
`

// findMapsInFile discovers all map variables in a Go source file
func findMapsInFile(filename string) ([]MapInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing source file: %v", err)
	}

	var maps []MapInfo
	ast.Inspect(node, func(n ast.Node) bool {
		if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.VAR {
			for _, spec := range decl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for i, ident := range valueSpec.Names {
						if mapType, ok := valueSpec.Type.(*ast.MapType); ok {
							keyType := extractTypeString(mapType.Key)
							valueType := extractTypeString(mapType.Value)

							// Only process maps with string values and string/int keys
							if valueType == "string" && (keyType == "string" || keyType == "int") {
								if lit, ok := valueSpec.Values[i].(*ast.CompositeLit); ok {
									maps = append(maps, MapInfo{
										Name:       ident.Name,
										KeyType:    keyType,
										ValueType:  valueType,
										Definition: lit,
									})
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	return maps, nil
}

// extractTypeString converts an ast.Expr representing a type into a string
func extractTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	default:
		return "unknown"
	}
}

var mapSuffixes = []string{"MapToString", "Map", "ToDisplay", "ToString"}

// deriveTypeName extracts a type name from a map variable name
func deriveTypeName(varName string, suffixes []string) string {
	// Remove common suffixes
	name := varName
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	// Convert to PascalCase
	return toPascalCase(name)
}

// extractMapData converts a map's AST definition into a map[interface{}]string
func extractMapData(mapInfo MapInfo) (map[interface{}]string, error) {
	result := make(map[interface{}]string)

	for _, elt := range mapInfo.Definition.Elts {
		if kvExpr, ok := elt.(*ast.KeyValueExpr); ok {
			key := kvExpr.Key.(*ast.BasicLit)
			value := kvExpr.Value.(*ast.BasicLit)

			// Process the key based on its type
			var keyValue interface{}
			switch mapInfo.KeyType {
			case "string":
				keyValue = strings.Trim(key.Value, "\"")
			case "int":
				keyStr := strings.TrimSuffix(strings.TrimSpace(key.Value), "i")
				fmt.Sscanf(keyStr, "%d", &keyValue)
			default:
				return nil, fmt.Errorf("unsupported key type: %s", mapInfo.KeyType)
			}

			valueStr := strings.Trim(value.Value, "\"")
			result[keyValue] = valueStr
		}
	}

	return result, nil
}

// generateEnum creates the enum code from a map and its metadata
func generateEnum(sourceMap map[any]string, mapVarName, packageName string) ([]byte, error) {
	typeName := deriveTypeName(mapVarName, mapSuffixes)

	// Determine value type from the first key's type
	var valueType string
	for k := range sourceMap {
		switch k.(type) {
		case string:
			valueType = "string"
		case int:
			valueType = "int"
		default:
			return nil, fmt.Errorf("unsupported key type: %T", k)
		}
		break
	}

	typeInfo := TypeInfo{
		TypeName:  typeName,
		ValueType: valueType,
		Package:   packageName,
	}

	var values []EnumValue
	for value, display := range sourceMap {
		values = append(values, EnumValue{
			ConstName:   toPascalCase(display),
			Value:       value,
			DisplayName: display,
		})
	}

	tmpl, err := template.New("enum").Parse(codeTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, TemplateData{
		Type:   typeInfo,
		Values: values,
	}); err != nil {
		return nil, fmt.Errorf("executing template: %v", err)
	}

	return format.Source(buf.Bytes())
}

// deriveOutputFileName creates an appropriate output filename for a given map
func deriveOutputFileName(mapName string) string {
	// Remove common suffixes
	name := strings.TrimSuffix(mapName, "MapToString")
	name = strings.TrimSuffix(name, "ToString")
	name = strings.TrimSuffix(name, "Map")

	return strings.ToLower(name) + ".go"
}
