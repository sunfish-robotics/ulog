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

// Field describes one member of a [Format]. ArrayLength is zero for a scalar.
type Field struct {
	Name        string
	Type        Type
	ArrayLength int
}

// Format describes the schema carried by a ULog format-definition message.
type Format struct {
	Name   string
	Fields []Field
}

var (
	formatNamePattern = regexp.MustCompile(`^[A-Za-z0-9_/-]+$`)
	fieldNamePattern  = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	typePattern       = regexp.MustCompile(`^([A-Za-z0-9_/-]+)(?:\[([1-9][0-9]*)\])?$`)
)

// ParseFormat parses the payload of a ULog format-definition message.
func ParseFormat(definition string) (*Format, error) {
	name, fieldsText, ok := strings.Cut(definition, ":")
	if !ok {
		return nil, fmt.Errorf("format definition has no name separator: %q", definition)
	}
	if !formatNamePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid format name %q", name)
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
	parts := strings.Fields(declaration)
	if len(parts) != 2 {
		return Field{}, fmt.Errorf("invalid field declaration %q", declaration)
	}
	if !fieldNamePattern.MatchString(parts[1]) {
		return Field{}, fmt.Errorf("invalid field name %q", parts[1])
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

// String returns the canonical ULog format definition.
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
