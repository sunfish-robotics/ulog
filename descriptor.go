package ulog

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

type typedSchema struct {
	root    reflect.Type
	formats []Format
	leaves  []typedLeaf
}

type typedLeaf struct {
	path   string
	typeID Type
	steps  []valueStep
}

type valueStep struct {
	field int
	array int
}

type schemaBuilder struct {
	visiting map[reflect.Type]bool
	built    map[reflect.Type]Format
	byName   map[string]Format
	ordered  []Format
}

// FormatsFor derives the ULog format graph for T. Nested definitions precede
// the root definition in the returned slice. T must be a named struct type.
func FormatsFor[T any]() ([]Format, error) {
	schema, err := typedSchemaFor(reflect.TypeFor[T]())
	if err != nil {
		return nil, err
	}
	return cloneFormats(schema.formats), nil
}

// FormatFor derives the root ULog format for T. Use [FormatsFor] when nested
// format definitions are also required.
func FormatFor[T any]() (*Format, error) {
	formats, err := FormatsFor[T]()
	if err != nil {
		return nil, err
	}
	root := cloneFormat(formats[len(formats)-1])
	return &root, nil
}

// Decode decodes record into T after checking flattened field names, order, and
// primitive wire types. Additional trailing record fields are ignored.
func Decode[T any](record Record) (T, error) {
	var result T
	schema, err := typedSchemaFor(reflect.TypeFor[T]())
	if err != nil {
		return result, err
	}
	values, err := record.Values()
	if err != nil {
		return result, err
	}
	if len(values) < len(schema.leaves) {
		return result, fmt.Errorf("record %q has %d fields, but %s requires %d", record.Name(), len(values), schema.root, len(schema.leaves))
	}

	destination := reflect.ValueOf(&result).Elem()
	for i, leaf := range schema.leaves {
		value := values[i]
		if value.Name != leaf.path {
			return result, fmt.Errorf("record field %d is %q, but %s requires %q", i, value.Name, schema.root, leaf.path)
		}
		if value.Type != leaf.typeID {
			return result, fmt.Errorf("record field %q has type %q, but %s requires %q", value.Name, value.Type, schema.root, leaf.typeID)
		}
		if err := setTypedValue(destination, leaf.steps, value.Value); err != nil {
			return result, fmt.Errorf("decode field %q: %w", leaf.path, err)
		}
	}
	return result, nil
}

func typedSchemaFor(root reflect.Type) (*typedSchema, error) {
	if root == nil || root.Kind() != reflect.Struct || root.Name() == "" {
		return nil, errors.New("ULog reflection requires a named struct type")
	}

	builder := schemaBuilder{
		visiting: make(map[reflect.Type]bool),
		built:    make(map[reflect.Type]Format),
		byName:   make(map[string]Format),
	}
	if _, err := builder.build(root); err != nil {
		return nil, err
	}

	leaves, err := typedLeaves(root, "", nil)
	if err != nil {
		return nil, err
	}
	return &typedSchema{root: root, formats: builder.ordered, leaves: leaves}, nil
}

func (b *schemaBuilder) build(typ reflect.Type) (Format, error) {
	if format, ok := b.built[typ]; ok {
		return format, nil
	}
	if b.visiting[typ] {
		return Format{}, fmt.Errorf("recursive Go type %s cannot be represented by ULog", typ)
	}
	b.visiting[typ] = true
	defer delete(b.visiting, typ)

	format := Format{Name: lowerSnake(typ.Name())}
	if !formatNamePattern.MatchString(format.Name) {
		return Format{}, fmt.Errorf("go type %s produces invalid ULog format name %q", typ, format.Name)
	}
	seenFields := make(map[string]struct{})
	for i := range typ.NumField() {
		field, err := reflectField(typ.Field(i))
		if err != nil {
			return Format{}, fmt.Errorf("%s.%s: %w", typ, typ.Field(i).Name, err)
		}
		if field.skipped {
			continue
		}
		if _, duplicate := seenFields[field.name]; duplicate {
			return Format{}, fmt.Errorf("%s has duplicate ULog field name %q", typ, field.name)
		}
		seenFields[field.name] = struct{}{}

		if field.nested != nil {
			nested, err := b.build(field.nested)
			if err != nil {
				return Format{}, err
			}
			field.typeID = Type(nested.Name)
		}
		format.Fields = append(format.Fields, Field{
			Name:        field.name,
			Type:        field.typeID,
			ArrayLength: field.arrayLength,
		})
	}
	if len(format.Fields) == 0 {
		return Format{}, fmt.Errorf("go type %s has no ULog fields", typ)
	}

	if existing, ok := b.byName[format.Name]; ok && existing.String() != format.String() {
		return Format{}, fmt.Errorf("go types produce incompatible ULog format definitions named %q", format.Name)
	}
	b.built[typ] = format
	b.byName[format.Name] = format
	b.ordered = append(b.ordered, cloneFormat(format))
	return format, nil
}

type reflectedField struct {
	name        string
	typeID      Type
	arrayLength int
	nested      reflect.Type
	skipped     bool
}

func reflectField(field reflect.StructField) (reflectedField, error) {
	tag := field.Tag.Get("ulog")
	if tag == "-" {
		return reflectedField{skipped: true}, nil
	}
	if field.PkgPath != "" {
		return reflectedField{}, errors.New("unexported fields must use `ulog:\"-\"`")
	}
	if strings.Contains(tag, ",") {
		return reflectedField{}, fmt.Errorf("unsupported ulog tag %q", tag)
	}

	name := tag
	if name == "" {
		name = lowerSnake(field.Name)
	}
	if !fieldNamePattern.MatchString(name) {
		return reflectedField{}, fmt.Errorf("invalid ULog field name %q", name)
	}

	typ := field.Type
	arrayLength := 0
	if typ.Kind() == reflect.Array {
		arrayLength = typ.Len()
		if arrayLength == 0 {
			return reflectedField{}, errors.New("zero-length arrays are not supported")
		}
		typ = typ.Elem()
		if typ.Kind() == reflect.Array {
			return reflectedField{}, errors.New("multidimensional arrays are not supported")
		}
	}

	if typeID, ok := primitiveTypeFor(typ); ok {
		return reflectedField{name: name, typeID: typeID, arrayLength: arrayLength}, nil
	}
	if typ.Kind() == reflect.Struct && typ.Name() != "" {
		return reflectedField{name: name, arrayLength: arrayLength, nested: typ}, nil
	}
	return reflectedField{}, fmt.Errorf("unsupported Go type %s", field.Type)
}

func primitiveTypeFor(typ reflect.Type) (Type, bool) {
	switch typ.Kind() {
	case reflect.Int8:
		return TypeInt8, true
	case reflect.Uint8:
		return TypeUint8, true
	case reflect.Int16:
		return TypeInt16, true
	case reflect.Uint16:
		return TypeUint16, true
	case reflect.Int32:
		return TypeInt32, true
	case reflect.Uint32:
		return TypeUint32, true
	case reflect.Int64:
		return TypeInt64, true
	case reflect.Uint64:
		return TypeUint64, true
	case reflect.Float32:
		return TypeFloat32, true
	case reflect.Float64:
		return TypeFloat64, true
	case reflect.Bool:
		return TypeBool, true
	default:
		return "", false
	}
}

func typedLeaves(typ reflect.Type, prefix string, steps []valueStep) ([]typedLeaf, error) {
	var leaves []typedLeaf
	for i := range typ.NumField() {
		field, err := reflectField(typ.Field(i))
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", typ, typ.Field(i).Name, err)
		}
		if field.skipped {
			continue
		}

		count := field.arrayLength
		if count == 0 {
			count = 1
		}
		for arrayIndex := range count {
			path := field.name
			step := valueStep{field: i, array: -1}
			if field.arrayLength > 0 {
				path += fmt.Sprintf("[%d]", arrayIndex)
				step.array = arrayIndex
			}
			if prefix != "" {
				path = prefix + "." + path
			}
			fieldSteps := append(append([]valueStep(nil), steps...), step)
			if field.nested != nil {
				nested, err := typedLeaves(field.nested, path, fieldSteps)
				if err != nil {
					return nil, err
				}
				leaves = append(leaves, nested...)
				continue
			}
			leaves = append(leaves, typedLeaf{path: path, typeID: field.typeID, steps: fieldSteps})
		}
	}
	return leaves, nil
}

func setTypedValue(destination reflect.Value, steps []valueStep, value any) error {
	for _, step := range steps {
		destination = destination.Field(step.field)
		if step.array >= 0 {
			destination = destination.Index(step.array)
		}
	}

	source := reflect.ValueOf(value)
	switch destination.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		destination.SetInt(source.Int())
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		destination.SetUint(source.Uint())
	case reflect.Float32, reflect.Float64:
		destination.SetFloat(source.Float())
	case reflect.Bool:
		destination.SetBool(source.Bool())
	default:
		return fmt.Errorf("unsupported destination type %s", destination.Type())
	}
	return nil
}

func lowerSnake(name string) string {
	runes := []rune(name)
	var result strings.Builder
	for i, current := range runes {
		if unicode.IsUpper(current) && i > 0 {
			previous := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextIsLower) {
				result.WriteByte('_')
			}
		}
		result.WriteRune(unicode.ToLower(current))
	}
	return result.String()
}

func cloneFormats(formats []Format) []Format {
	cloned := make([]Format, len(formats))
	for i, format := range formats {
		cloned[i] = cloneFormat(format)
	}
	return cloned
}
