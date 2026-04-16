// Package gleam provides Gleam language formatting utilities.
package gleam

import (
	"strings"
	"unicode"
)

// ModuleName converts a proto file path (e.g. "aegis/scan/v1/scan.proto")
// to a Gleam module name (e.g. "scan_v1" or with prefix "my_app/proto/scan_v1").
func ModuleName(protoPath, prefix string) string {
	// Strip directory and .proto extension.
	name := protoPath
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".proto")
	name = ToSnakeCase(name)

	if prefix != "" {
		return prefix + "/" + name
	}
	return name
}

// ToSnakeCase converts a PascalCase or camelCase string to snake_case.
func ToSnakeCase(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rune(s[i-1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					b.WriteByte('_')
				} else if unicode.IsUpper(prev) && i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
					// Handle "XMLParser" -> "xml_parser"
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ToPascalCase converts a snake_case string to PascalCase.
func ToPascalCase(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

// EnumVariantName converts a proto enum value name to a Gleam constructor.
// E.g. "SCAN_MODE_URL" with prefix "SCAN_MODE" -> "ScanModeUrl".
func EnumVariantName(enumName, valueName string) string {
	// Proto convention: VALUE_NAME = ENUM_PREFIX + "_" + VARIANT.
	// We want to produce PascalCase from the full name.
	lower := strings.ToLower(valueName)
	return ToPascalCase(lower)
}

// EnumToStringValue returns the lowercase string form of a variant.
// E.g. "SEVERITY_CRITICAL" with prefix "SEVERITY" -> "critical".
func EnumToStringValue(enumName, valueName string) string {
	prefix := strings.ToUpper(ToSnakeCase(enumName)) + "_"
	trimmed := strings.TrimPrefix(valueName, prefix)
	return strings.ToLower(trimmed)
}

// FieldName converts a proto field name to Gleam record field name (already snake_case in proto3).
func FieldName(protoName string) string {
	return protoName
}

// TypeName converts a proto message/enum name to a Gleam type name (PascalCase).
func TypeName(protoName string) string {
	return protoName
}

// OneofVariantName converts a oneof field name to a Gleam sum type constructor.
// E.g. "start_command" -> "StartCommand".
func OneofVariantName(fieldName string) string {
	return ToPascalCase(fieldName)
}
