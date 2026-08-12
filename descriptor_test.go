package ulog

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

type typedPoint struct {
	X int16
	Y float32
}

type typedPointSample struct {
	Timestamp uint64
	X         int16
	Y         float32
}

type typedSample struct {
	Timestamp  uint64
	HTTPStatus uint16
	Points     [2]typedPoint
	Q          [2]float32
	Ignored    string  `ulog:"-"`
	Speed      float32 `ulog:"velocity"`
}

type typedTextSample struct {
	Timestamp uint64
	Name      string `ulog:"name,char[8]"`
}

type typedLabel struct {
	Name string `ulog:"name,char[8]"`
}

type typedNestedTextSample struct {
	Timestamp uint64
	Labels    [2]typedLabel
}

type zeroWidthTextSample struct {
	Name string `ulog:"name,char[0]"`
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

func TestTypedAdaptersMapFixedWidthCharacterArraysToStrings(t *testing.T) {
	format, err := FormatFor[typedTextSample]()
	if err != nil {
		t.Fatalf("FormatFor() error = %v", err)
	}
	wantFormat := Format{
		Name: "typed_text_sample",
		Fields: []Field{
			{Name: "timestamp", Type: TypeUint64},
			{Name: "name", Type: TypeChar, ArrayLength: 8},
		},
	}
	if !reflect.DeepEqual(*format, wantFormat) {
		t.Errorf("FormatFor() = %#v, want %#v", *format, wantFormat)
	}

	var destination bytes.Buffer
	writer, err := NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := Register[typedTextSample](writer)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := stream.Write(typedTextSample{Timestamp: 42, Name: "zōda"}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reader, err := NewReader(bytes.NewReader(destination.Bytes()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if !reader.Next() {
		t.Fatalf("Next() = false, error = %v", reader.Err())
	}
	value, err := reader.Record().Value("name")
	if err != nil {
		t.Fatalf("Value(name) error = %v", err)
	}
	if got, want := value, any("zōda"); got != want {
		t.Errorf("Value(name) = %#v, want %#v", got, want)
	}
	decoded, err := Decode[typedTextSample](reader.Record())
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got, want := decoded, (typedTextSample{Timestamp: 42, Name: "zōda"}); got != want {
		t.Errorf("Decode() = %#v, want %#v", got, want)
	}
}

func TestTypedAdaptersMapNestedCharacterArrayStrings(t *testing.T) {
	formats, err := FormatsFor[typedNestedTextSample]()
	if err != nil {
		t.Fatalf("FormatsFor() error = %v", err)
	}
	wantFormats := []Format{
		{
			Name: "typed_label",
			Fields: []Field{
				{Name: "name", Type: TypeChar, ArrayLength: 8},
			},
		},
		{
			Name: "typed_nested_text_sample",
			Fields: []Field{
				{Name: "timestamp", Type: TypeUint64},
				{Name: "labels", Type: "typed_label", ArrayLength: 2},
			},
		},
	}
	if !reflect.DeepEqual(formats, wantFormats) {
		t.Errorf("FormatsFor() = %#v, want %#v", formats, wantFormats)
	}

	var destination bytes.Buffer
	writer, err := NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := Register[typedNestedTextSample](writer)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	want := typedNestedTextSample{
		Timestamp: 42,
		Labels: [2]typedLabel{
			{Name: "zōda"},
			{Name: "hi\x00x"},
		},
	}
	if err := stream.Write(want); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reader, err := NewReader(bytes.NewReader(destination.Bytes()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if !reader.Next() {
		t.Fatalf("Next() = false, error = %v", reader.Err())
	}
	values, err := reader.Record().Values()
	if err != nil {
		t.Fatalf("Values() error = %v", err)
	}
	wantValues := []FieldValue{
		{Name: "timestamp", Type: TypeUint64, Value: uint64(42)},
		{Name: "labels[0].name", Type: TypeChar, ArrayLength: 8, Value: "zōda"},
		{Name: "labels[1].name", Type: TypeChar, ArrayLength: 8, Value: "hi\x00x"},
	}
	if !reflect.DeepEqual(values, wantValues) {
		t.Errorf("Values() = %#v, want %#v", values, wantValues)
	}
	got, err := Decode[typedNestedTextSample](reader.Record())
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Decode() = %#v, want %#v", got, want)
	}
}

func TestTypedStringWriteRejectsValuesWiderThanCharacterArray(t *testing.T) {
	var destination bytes.Buffer
	writer, err := NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := Register[typedTextSample](writer)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := stream.Write(typedTextSample{Name: "123456789"}); err == nil {
		t.Fatal("Write() succeeded with a string wider than char[8]")
	}
	if err := stream.Write(typedTextSample{Timestamp: 42, Name: "zoda"}); err != nil {
		t.Fatalf("Write() after correcting string = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reader, err := NewReader(bytes.NewReader(destination.Bytes()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if !reader.Next() {
		t.Fatalf("Next() = false after retry, error = %v", reader.Err())
	}
	if got, err := reader.Record().Value("name"); err != nil || got != "zoda" {
		t.Errorf("Value(name) = %#v, %v, want %q, nil", got, err, "zoda")
	}
}

func TestTypedStringDecodeRejectsDifferentCharacterArrayWidth(t *testing.T) {
	formats := mustParseFormats(t, "external:uint64_t timestamp;char[7] name;")
	data := binary.LittleEndian.AppendUint64(nil, 42)
	data = append(data, 'z', 'o', 'd', 'a', 0, 0, 0)
	record := mustRecord(t, formats, "external", data)

	if _, err := Decode[typedTextSample](record); err == nil {
		t.Fatal("Decode() succeeded with a different character array width")
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

func TestFormatForRequiresFixedWidthTagForStrings(t *testing.T) {
	type missingWidth struct {
		Name string
	}

	if _, err := FormatFor[missingWidth](); err == nil {
		t.Fatal("FormatFor() succeeded for a string without a char[N] tag")
	}
	if _, err := FormatFor[zeroWidthTextSample](); err == nil {
		t.Fatal("FormatFor() succeeded for char[0]")
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
