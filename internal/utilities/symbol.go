package utilities

import (
	"fmt"
	"reflect"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// Symbol Kind String Mapping
// This is a map of LSP SymbolKind values to their human-readable string representation
// Used by both document_symbols.go and find-references.go

// GetSymbolKindString converts a SymbolKind to a descriptive format string with brackets
func GetSymbolKindString(kind protocol.SymbolKind) string {
	switch kind {
	case 1: // File
		return "[File]"
	case 2: // Module
		return "[Module]"
	case 3: // Namespace
		return "[Namespace]"
	case 4: // Package
		return "[Package]"
	case 5: // Class
		return "[Class]"
	case 6: // Method
		return "[Method]"
	case 7: // Property
		return "[Property]"
	case 8: // Field
		return "[Field]"
	case 9: // Constructor
		return "[Constructor]"
	case 10: // Enum
		return "[Enum]"
	case 11: // Interface
		return "[Interface]"
	case 12: // Function
		return "[Function]"
	case 13: // Variable
		return "[Variable]"
	case 14: // Constant
		return "[Constant]"
	case 15: // String
		return "[String]"
	case 16: // Number
		return "[Number]"
	case 17: // Boolean
		return "[Boolean]"
	case 18: // Array
		return "[Array]"
	case 19: // Object
		return "[Object]"
	case 20: // Key
		return "[Key]"
	case 21: // Null
		return "[Null]"
	case 22: // EnumMember
		return "[EnumMember]"
	case 23: // Struct
		return "[Struct]"
	case 24: // Event
		return "[Event]"
	case 25: // Operator
		return "[Operator]"
	case 26: // TypeParameter
		return "[TypeParameter]"
	default:
		return "[Unknown]"
	}
}

// FormatSymbolWithKind formats a symbol with its kind in a consistent way across the codebase
func FormatSymbolWithKind(kind, name string) string {
	if kind == "" {
		return name
	}
	return fmt.Sprintf("%s %s", kind, name)
}

// ExtractSymbolKind attempts to get the SymbolKind from a DocumentSymbolResult using reflection
// Returns the formatted kind string with brackets (e.g. [Function])
func ExtractSymbolKind(sym protocol.DocumentSymbolResult) string {
	// Default to Symbol
	kindStr := "[Symbol]"

	// Try to extract kind through reflection since we have different struct types
	// with different ways to access Kind
	symValue := reflect.ValueOf(sym).Elem()

	// Try direct Kind field
	if kindField := symValue.FieldByName("Kind"); kindField.IsValid() {
		kind := protocol.SymbolKind(kindField.Uint())
		return GetSymbolKindString(kind)
	}

	// Try BaseSymbolInformation.Kind
	if baseField := symValue.FieldByName("BaseSymbolInformation"); baseField.IsValid() {
		if kindField := baseField.FieldByName("Kind"); kindField.IsValid() {
			kind := protocol.SymbolKind(kindField.Uint())
			return GetSymbolKindString(kind)
		}
	}

	return kindStr
}
