package ulog

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

type typedPoint struct {
	X int16
	Y float32
}

type typedSample struct {
	Timestamp  uint64
	HTTPStatus uint16
	Points     [2]typedPoint
	Q          [2]float32
	Ignored    string  `ulog:"-"`
	Speed      float32 `ulog:"velocity"`
}

func TestFormatsForDerivesULogSchemasFromGoTypes(t *testing.T) {
	formats, err := FormatsFor[typedSample]()
	if err != nil {
		t.Fatalf("FormatsFor() error = %v", err)
	}

	want := []Format{
		{
			Name: "typed_point",
			Fields: []Field{
				{Name: "x", Type: TypeInt16},
				{Name: "y", Type: TypeFloat32},
			},
		},
		{
			Name: "typed_sample",
			Fields: []Field{
				{Name: "timestamp", Type: TypeUint64},
				{Name: "http_status", Type: TypeUint16},
				{Name: "points", Type: "typed_point", ArrayLength: 2},
				{Name: "q", Type: TypeFloat32, ArrayLength: 2},
				{Name: "velocity", Type: TypeFloat32},
			},
		},
	}
	if !reflect.DeepEqual(formats, want) {
		t.Errorf("FormatsFor() = %#v, want %#v", formats, want)
	}

	root, err := FormatFor[typedSample]()
	if err != nil {
		t.Fatalf("FormatFor() error = %v", err)
	}
	if !reflect.DeepEqual(*root, want[1]) {
		t.Errorf("FormatFor() = %#v, want %#v", *root, want[1])
	}
}

func TestDecodeUsesRecordSchemaRatherThanGoGeneratedSchema(t *testing.T) {
	formats := mustParseFormats(t,
		"typed_point:int16_t x;float y;",
		"external:uint64_t timestamp;uint16_t http_status;typed_point[2] points;float[2] q;float velocity;uint32_t future_field;",
	)

	data := binary.LittleEndian.AppendUint64(nil, 123456)
	data = binary.LittleEndian.AppendUint16(data, 204)
	data = append(data, 0xf9, 0xff)
	data = binary.LittleEndian.AppendUint32(data, math.Float32bits(1.25))
	data = binary.LittleEndian.AppendUint16(data, 9)
	data = binary.LittleEndian.AppendUint32(data, math.Float32bits(-2.5))
	data = binary.LittleEndian.AppendUint32(data, math.Float32bits(0.5))
	data = binary.LittleEndian.AppendUint32(data, math.Float32bits(-0.75))
	data = binary.LittleEndian.AppendUint32(data, math.Float32bits(4.5))
	data = binary.LittleEndian.AppendUint32(data, 99)
	record := mustRecord(t, formats, "external", data)

	got, err := Decode[typedSample](record)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	want := typedSample{
		Timestamp:  123456,
		HTTPStatus: 204,
		Points: [2]typedPoint{
			{X: -7, Y: 1.25},
			{X: 9, Y: -2.5},
		},
		Q:     [2]float32{0.5, -0.75},
		Speed: 4.5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Decode() = %#v, want %#v", got, want)
	}
}

func TestDecodeRejectsIncompatibleFields(t *testing.T) {
	formats := mustParseFormats(t,
		"typed_point:int16_t x;float y;",
		"typed_sample:uint64_t timestamp;uint32_t http_status;typed_point[2] points;float[2] q;float velocity;",
	)
	record := mustRecord(t, formats, "typed_sample", make([]byte, 42))

	if _, err := Decode[typedSample](record); err == nil {
		t.Fatal("Decode() succeeded with an incompatible field type")
	}
}

func TestFormatForRejectsUnsupportedGoFields(t *testing.T) {
	type invalid struct {
		Samples []float32
	}

	if _, err := FormatFor[invalid](); err == nil {
		t.Fatal("FormatFor() succeeded for a slice field")
	}
}

func mustParseFormats(t *testing.T, definitions ...string) map[string]Format {
	t.Helper()
	formats := make(map[string]Format, len(definitions))
	for _, definition := range definitions {
		format, err := ParseFormat(definition)
		if err != nil {
			t.Fatalf("ParseFormat(%q): %v", definition, err)
		}
		formats[format.Name] = *format
	}
	return formats
}

func mustRecord(t *testing.T, formats map[string]Format, name string, data []byte) Record {
	t.Helper()
	format, ok := formats[name]
	if !ok {
		t.Fatalf("missing format %q", name)
	}
	layout, err := resolveLayout(format, formats)
	if err != nil {
		t.Fatalf("resolveLayout(%q): %v", name, err)
	}
	return Record{format: format, layout: layout, data: data}
}
