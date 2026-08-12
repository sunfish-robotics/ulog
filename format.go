package ulog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Type identifies a primitive ULog type or the name of another [Format].
type Type string

// ULog message sizes are uint16 and a data message reserves two bytes for its
// subscription ID.
const maxDataPayloadSize = 1<<16 - 1 - 2

const (
	// TypeInt8 identifies a signed 8-bit integer.
	TypeInt8 Type = "int8_t"
	// TypeUint8 identifies an unsigned 8-bit integer.
	TypeUint8 Type = "uint8_t"
	// TypeInt16 identifies a signed 16-bit integer.
	TypeInt16 Type = "int16_t"
	// TypeUint16 identifies an unsigned 16-bit integer.
	TypeUint16 Type = "uint16_t"
	// TypeInt32 identifies a signed 32-bit integer.
	TypeInt32 Type = "int32_t"
	// TypeUint32 identifies an unsigned 32-bit integer.
	TypeUint32 Type = "uint32_t"
	// TypeInt64 identifies a signed 64-bit integer.
	TypeInt64 Type = "int64_t"
	// TypeUint64 identifies an unsigned 64-bit integer.
	TypeUint64 Type = "uint64_t"
	// TypeFloat32 identifies a 32-bit IEEE-754 floating-point value.
	TypeFloat32 Type = "float"
	// TypeFloat64 identifies a 64-bit IEEE-754 floating-point value.
	TypeFloat64 Type = "double"
	// TypeBool identifies a one-byte Boolean value.
	TypeBool Type = "bool"
	// TypeChar identifies a one-byte character.
	TypeChar Type = "char"
)

// IsPrimitive reports whether t is one of ULog's built-in scalar types.
func (t Type) IsPrimitive() bool {
	_, ok := primitiveSize(t)
	return ok
}

// Field describes one member of a [Format]. A non-primitive [Field.Type] names
// another format. [Field.ArrayLength] is zero for a scalar.
type Field struct {
	// Name is the case-sensitive field name used on the wire.
	Name string
	// Type is a ULog primitive type or the name of another [Format].
	Type Type
	// ArrayLength is the fixed element count, or zero for a scalar.
	ArrayLength int
}

// Format is the self-described wire schema for one kind of data record. Fields
// remain in wire order.
type Format struct {
	// Name is the case-sensitive name used by subscriptions and nested fields.
	Name string
	// Fields contains at least one field in wire order.
	Fields []Field
}

var (
	formatNamePattern = regexp.MustCompile(`^[A-Za-z0-9_/-]+$`)
	fieldNamePattern  = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	typePattern       = regexp.MustCompile(`^([A-Za-z0-9_/-]+)(?:\[([1-9][0-9]*)\])?$`)
)

// ParseFormat parses one "name:type field;" definition. It validates the grammar,
// names, duplicate fields, and primitive array sizes, but does not resolve nested
// [Format] names, detect cycles, or require the timestamp field needed by a
// subscription.
func ParseFormat(definition string) (*Format, error) {
	name, fieldsText, ok := strings.Cut(definition, ":")
	if !ok {
		return nil, fmt.Errorf("format definition has no name separator: %q", definition)
	}
	if !formatNamePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid format name %q", name)
	}
	if Type(name).IsPrimitive() {
		return nil, fmt.Errorf("format name %q collides with a primitive type", name)
	}
	if fieldsText == "" {
		return nil, fmt.Errorf("format %q has no fields", name)
	}

	format := &Format{Name: name}
	seen := make(map[string]struct{})
	for declaration := range strings.SplitSeq(fieldsText, ";") {
		declaration = strings.TrimSpace(declaration)
		if declaration == "" {
			continue
		}

		field, err := parseField(declaration)
		if err != nil {
			return nil, fmt.Errorf("format %q: %w", name, err)
		}
		if _, duplicate := seen[field.Name]; duplicate {
			return nil, fmt.Errorf("format %q has duplicate field %q", name, field.Name)
		}
		seen[field.Name] = struct{}{}
		format.Fields = append(format.Fields, field)
	}
	if len(format.Fields) == 0 {
		return nil, fmt.Errorf("format %q has no fields", name)
	}

	return format, nil
}

func parseField(declaration string) (Field, error) {
	return parseDeclaration(declaration, fieldNamePattern, "field")
}

func parseKey(declaration string) (Field, error) {
	return parseDeclaration(declaration, formatNamePattern, "key")
}

func parseDeclaration(declaration string, namePattern *regexp.Regexp, nameKind string) (Field, error) {
	parts := strings.Fields(declaration)
	if len(parts) != 2 {
		return Field{}, fmt.Errorf("invalid %s declaration %q", nameKind, declaration)
	}
	if !namePattern.MatchString(parts[1]) {
		return Field{}, fmt.Errorf("invalid %s name %q", nameKind, parts[1])
	}

	matches := typePattern.FindStringSubmatch(parts[0])
	if matches == nil {
		return Field{}, fmt.Errorf("invalid field type %q", parts[0])
	}

	field := Field{Name: parts[1], Type: Type(matches[1])}
	if matches[2] != "" {
		length, err := strconv.Atoi(matches[2])
		if err != nil {
			return Field{}, fmt.Errorf("invalid array length %q: %w", matches[2], err)
		}
		if length > maxDataPayloadSize {
			return Field{}, fmt.Errorf("array length %d exceeds maximum data payload %d", length, maxDataPayloadSize)
		}
		if size, primitive := primitiveSize(field.Type); primitive && length > maxDataPayloadSize/size {
			return Field{}, fmt.Errorf("array %q requires %d bytes, maximum data payload is %d", field.Name, length*size, maxDataPayloadSize)
		}
		field.ArrayLength = length
	}

	return field, nil
}

// String returns f in canonical "name:type field;" form.
func (f Format) String() string {
	var definition strings.Builder
	definition.WriteString(f.Name)
	definition.WriteByte(':')
	for _, field := range f.Fields {
		definition.WriteString(string(field.Type))
		if field.ArrayLength > 0 {
			definition.WriteByte('[')
			definition.WriteString(strconv.Itoa(field.ArrayLength))
			definition.WriteByte(']')
		}
		definition.WriteByte(' ')
		definition.WriteString(field.Name)
		definition.WriteByte(';')
	}
	return definition.String()
}

func validateSubscriptionFormat(format Format) error {
	for _, field := range format.Fields {
		if field.Name != "timestamp" {
			continue
		}
		if field.Type != TypeUint64 || field.ArrayLength != 0 {
			return fmt.Errorf("subscribed format %q requires scalar uint64_t timestamp", format.Name)
		}
		return nil
	}
	return fmt.Errorf("subscribed format %q requires scalar uint64_t timestamp", format.Name)
}

func primitiveSize(t Type) (int, bool) {
	switch t {
	case TypeInt8, TypeUint8, TypeBool, TypeChar:
		return 1, true
	case TypeInt16, TypeUint16:
		return 2, true
	case TypeInt32, TypeUint32, TypeFloat32:
		return 4, true
	case TypeInt64, TypeUint64, TypeFloat64:
		return 8, true
	default:
		return 0, false
	}
}
