package ulog

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// KeyValue is a decoded information or parameter entry. [KeyValue.Value] has the
// Go scalar or slice type selected by [KeyValue.Type]; character arrays are
// strings. [KeyValue.ArrayLength] is zero for a scalar.
type KeyValue struct {
	// Name is the case-sensitive information key or parameter name.
	Name string
	// Type is the primitive ULog type of Value.
	Type Type
	// ArrayLength is the fixed element count, or zero for a scalar.
	ArrayLength int
	// Value is a Go scalar, primitive slice, or string matching Type and ArrayLength.
	Value any
}

// MultiInformationValue is one independently typed value from a
// [MultiInformationGroup]. IsArray distinguishes scalar values from arrays,
// including PX4's empty char[0] separator values.
type MultiInformationValue struct {
	KeyValue
	// IsArray reports whether the wire declaration included an array length.
	IsArray bool
}

// MultiInformationGroup contains one ordered group of multi-information values
// with the same name. The first value starts the group; subsequent values were
// marked as continuations on the wire. Each value retains its own declared type
// and array length.
type MultiInformationGroup struct {
	// Name is the case-sensitive information key shared by Values.
	Name string
	// Values contains the independently typed entries in wire order.
	Values []MultiInformationValue
}

// DefaultParameterTypes identifies the independent configuration scopes to
// which a [DefaultParameter] applies. A value may apply to both scopes.
type DefaultParameterTypes uint8

const (
	// DefaultParameterSystemWide marks a system-wide default value.
	DefaultParameterSystemWide DefaultParameterTypes = 1 << 0
	// DefaultParameterCurrentConfiguration marks a default for the current configuration.
	DefaultParameterCurrentConfiguration DefaultParameterTypes = 1 << 1
)

// DefaultParameter is a parameter's default value for the scopes in
// [DefaultParameter.Types]. If a log has no default for a given parameter and
// scope, ULog defines its parameter value as the default.
type DefaultParameter struct {
	KeyValue
	// Types contains one or more applicable default scopes.
	Types DefaultParameterTypes
}

// LogLevel is the severity of a ULog text message.
type LogLevel uint8

const (
	// LogLevelEmergency indicates that the system is unusable.
	LogLevelEmergency LogLevel = '0'
	// LogLevelAlert indicates that action must be taken immediately.
	LogLevelAlert LogLevel = '1'
	// LogLevelCritical indicates a critical condition.
	LogLevelCritical LogLevel = '2'
	// LogLevelError indicates an error condition.
	LogLevelError LogLevel = '3'
	// LogLevelWarning indicates a warning condition.
	LogLevelWarning LogLevel = '4'
	// LogLevelNotice indicates a normal but significant condition.
	LogLevelNotice LogLevel = '5'
	// LogLevelInfo indicates an informational message.
	LogLevelInfo LogLevel = '6'
	// LogLevelDebug indicates a debug message.
	LogLevelDebug LogLevel = '7'
)

// LogEntry is one printf-style ULog text message. [LogEntry.Timestamp] is in
// microseconds. [LogEntry.Tag] identifies an application-defined source only
// when [LogEntry.Tagged] is true.
type LogEntry struct {
	// Level is the message severity.
	Level LogLevel
	// Timestamp is the message timestamp in microseconds.
	Timestamp uint64
	// Message is the logged text.
	Message string
	// Tag is an application-defined source identifier when Tagged is true.
	Tag uint16
	// Tagged reports whether Tag was present on the wire.
	Tagged bool
}

// Dropout describes a period during which logging messages were lost, often
// because the logging device could not keep up.
type Dropout struct {
	// Duration is the period for which logging messages were lost.
	Duration time.Duration
}

func keyValueField(key string) (Field, int, error) {
	field, err := parseKey(key)
	if err != nil {
		return Field{}, 0, err
	}
	size, primitive := primitiveSize(field.Type)
	if !primitive {
		return Field{}, 0, fmt.Errorf("key %q uses non-primitive type %q", field.Name, field.Type)
	}
	count := field.ArrayLength
	if count == 0 {
		count = 1
	}
	return field, size * count, nil
}

func decodeKeyValue(key string, data []byte) (KeyValue, error) {
	field, wantSize, err := keyValueField(key)
	if err != nil {
		return KeyValue{}, err
	}
	if len(data) != wantSize {
		return KeyValue{}, fmt.Errorf("key %q has %d value bytes, want %d", field.Name, len(data), wantSize)
	}

	entry := KeyValue{Name: field.Name, Type: field.Type, ArrayLength: field.ArrayLength}
	if field.Type == TypeChar && field.ArrayLength > 0 {
		entry.Value = string(bytes.TrimRight(data, "\x00"))
		return entry, nil
	}
	if field.ArrayLength == 0 {
		entry.Value, err = decodePrimitive(field.Type, data)
		return entry, err
	}

	size, _ := primitiveSize(field.Type)
	values := make([]any, field.ArrayLength)
	for i := range field.ArrayLength {
		value, err := decodePrimitive(field.Type, data[i*size:(i+1)*size])
		if err != nil {
			return KeyValue{}, err
		}
		values[i] = value
	}
	entry.Value, err = typedPrimitiveSlice(field.Type, values)
	return entry, err
}

func decodeMultiInformationValue(key string, data []byte) (MultiInformationValue, error) {
	const emptyCharacterArray = "char[0] "
	if strings.HasPrefix(key, emptyCharacterArray) && len(data) == 0 {
		name := strings.TrimPrefix(key, emptyCharacterArray)
		if !formatNamePattern.MatchString(name) {
			return MultiInformationValue{}, fmt.Errorf("invalid key name %q", name)
		}
		return MultiInformationValue{
			KeyValue: KeyValue{Name: name, Type: TypeChar, Value: ""},
			IsArray:  true,
		}, nil
	}
	entry, err := decodeKeyValue(key, data)
	if err != nil {
		return MultiInformationValue{}, err
	}
	typeDeclaration, _, _ := strings.Cut(key, " ")
	return MultiInformationValue{
		KeyValue: entry,
		IsArray:  strings.Contains(typeDeclaration, "["),
	}, nil
}

func typedPrimitiveSlice(typeID Type, values []any) (any, error) {
	switch typeID {
	case TypeInt8:
		return collectPrimitiveValues[int8](values)
	case TypeUint8, TypeChar:
		return collectPrimitiveValues[uint8](values)
	case TypeInt16:
		return collectPrimitiveValues[int16](values)
	case TypeUint16:
		return collectPrimitiveValues[uint16](values)
	case TypeInt32:
		return collectPrimitiveValues[int32](values)
	case TypeUint32:
		return collectPrimitiveValues[uint32](values)
	case TypeInt64:
		return collectPrimitiveValues[int64](values)
	case TypeUint64:
		return collectPrimitiveValues[uint64](values)
	case TypeFloat32:
		return collectPrimitiveValues[float32](values)
	case TypeFloat64:
		return collectPrimitiveValues[float64](values)
	case TypeBool:
		return collectPrimitiveValues[bool](values)
	default:
		return nil, fmt.Errorf("unsupported primitive array type %q", typeID)
	}
}

func collectPrimitiveValues[T any](values []any) ([]T, error) {
	result := make([]T, len(values))
	for i, value := range values {
		typed, ok := value.(T)
		if !ok {
			return nil, fmt.Errorf("primitive value %d has type %T", i, value)
		}
		result[i] = typed
	}
	return result, nil
}

func encodeKeyValue(name string, value any) (string, []byte, error) {
	if !formatNamePattern.MatchString(name) {
		return "", nil, fmt.Errorf("invalid ULog key name %q", name)
	}
	if text, ok := value.(string); ok {
		if text == "" {
			return "", nil, fmt.Errorf("ULog string value %q must not be empty", name)
		}
		return fmt.Sprintf("char[%d] %s", len(text), name), []byte(text), nil
	}

	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return "", nil, fmt.Errorf("ULog key %q has a nil value", name)
	}
	typeID, ok := primitiveTypeFor(reflected.Type())
	if !ok {
		return "", nil, fmt.Errorf("ULog key %q has unsupported type %s", name, reflected.Type())
	}
	encoded, err := binary.Append(nil, binary.LittleEndian, reflected.Interface())
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("%s %s", typeID, name), encoded, nil
}

func cloneKeyValues(values []KeyValue) []KeyValue {
	cloned := make([]KeyValue, len(values))
	for i, value := range values {
		cloned[i] = value
		cloned[i].Value = clonePrimitiveSlice(value.Value)
	}
	return cloned
}

func cloneMultiInformation(groups []MultiInformationGroup) []MultiInformationGroup {
	cloned := make([]MultiInformationGroup, len(groups))
	for i, group := range groups {
		cloned[i] = group
		cloned[i].Values = append([]MultiInformationValue(nil), group.Values...)
		for j := range cloned[i].Values {
			cloned[i].Values[j].Value = clonePrimitiveSlice(cloned[i].Values[j].Value)
		}
	}
	return cloned
}

func cloneDefaultParameters(values []DefaultParameter) []DefaultParameter {
	cloned := append([]DefaultParameter(nil), values...)
	for i := range cloned {
		cloned[i].Value = clonePrimitiveSlice(cloned[i].Value)
	}
	return cloned
}

func clonePrimitiveSlice(value any) any {
	switch value := value.(type) {
	case []int8:
		return append([]int8(nil), value...)
	case []uint8:
		return append([]uint8(nil), value...)
	case []int16:
		return append([]int16(nil), value...)
	case []uint16:
		return append([]uint16(nil), value...)
	case []int32:
		return append([]int32(nil), value...)
	case []uint32:
		return append([]uint32(nil), value...)
	case []int64:
		return append([]int64(nil), value...)
	case []uint64:
		return append([]uint64(nil), value...)
	case []float32:
		return append([]float32(nil), value...)
	case []float64:
		return append([]float64(nil), value...)
	case []bool:
		return append([]bool(nil), value...)
	default:
		return value
	}
}
