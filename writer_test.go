package ulog

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"testing"
)

func TestTypedWriterProducesDynamicallyReadableRecords(t *testing.T) {
	var destination bytes.Buffer
	writer, err := NewWriter(&destination, WithStartTimestamp(987654))
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := Register[typedSample](writer, WithMultiID(2))
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	want := typedSample{
		Timestamp:  123,
		HTTPStatus: 201,
		Points: [2]typedPoint{
			{X: -4, Y: 1.5},
			{X: 8, Y: -2.25},
		},
		Q:     [2]float32{0.25, -0.5},
		Speed: 4.75,
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
	if got := reader.Header().Timestamp; got != 987654 {
		t.Errorf("Header().Timestamp = %d, want 987654", got)
	}
	if !reader.Next() {
		t.Fatalf("Next() = false, error = %v", reader.Err())
	}
	if got := reader.Record().MultiID(); got != 2 {
		t.Errorf("MultiID() = %d, want 2", got)
	}
	got, err := Decode[typedSample](reader.Record())
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decoded value = %#v, want %#v", got, want)
	}
}

func TestWriterSupportsDynamicFormats(t *testing.T) {
	var destination bytes.Buffer
	writer, err := NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}

	inner, err := ParseFormat("coordinates:int16_t x;int16_t y;")
	if err != nil {
		t.Fatalf("ParseFormat(inner) error = %v", err)
	}
	outer, err := ParseFormat("dynamic_sample:uint64_t timestamp;coordinates point;float value;")
	if err != nil {
		t.Fatalf("ParseFormat(outer) error = %v", err)
	}
	if err := writer.Define(*inner); err != nil {
		t.Fatalf("Define() error = %v", err)
	}
	stream, err := writer.RegisterFormat(*outer, WithMultiID(4))
	if err != nil {
		t.Fatalf("RegisterFormat() error = %v", err)
	}

	payload := binary.LittleEndian.AppendUint64(nil, 77)
	payload = append(payload, 0xfd, 0xff)
	payload = binary.LittleEndian.AppendUint16(payload, 9)
	payload = binary.LittleEndian.AppendUint32(payload, math.Float32bits(6.25))
	if err := stream.Write(payload); err != nil {
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
	want := []FieldValue{
		{Name: "timestamp", Type: TypeUint64, Value: uint64(77)},
		{Name: "point.x", Type: TypeInt16, Value: int16(-3)},
		{Name: "point.y", Type: TypeInt16, Value: int16(9)},
		{Name: "value", Type: TypeFloat32, Value: float32(6.25)},
	}
	if !reflect.DeepEqual(values, want) {
		t.Errorf("Values() = %#v, want %#v", values, want)
	}
}

func TestWriterRejectsRegistrationAfterDataStarts(t *testing.T) {
	var destination bytes.Buffer
	writer, err := NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := Register[typedPointSample](writer)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := stream.Write(typedPointSample{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := Register[typedSample](writer); err == nil {
		t.Fatal("Register() succeeded after data started")
	}
}

func TestRawStreamRejectsIncorrectPayloadSize(t *testing.T) {
	var destination bytes.Buffer
	writer, err := NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	format, err := ParseFormat("sample:uint64_t timestamp;uint32_t value;")
	if err != nil {
		t.Fatalf("ParseFormat() error = %v", err)
	}
	stream, err := writer.RegisterFormat(*format)
	if err != nil {
		t.Fatalf("RegisterFormat() error = %v", err)
	}
	if err := stream.Write([]byte{1, 2}); err == nil {
		t.Fatal("Write() succeeded with an incorrect payload size")
	}
}

func TestRawStreamAllowsOmittedTopLevelTrailingPadding(t *testing.T) {
	var destination bytes.Buffer
	writer, err := NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := writer.RegisterFormat(Format{
		Name: "padded",
		Fields: []Field{
			{Name: "timestamp", Type: TypeUint64},
			{Name: "value", Type: TypeUint8},
			{Name: "_padding0", Type: TypeUint8, ArrayLength: 3},
		},
	})
	if err != nil {
		t.Fatalf("RegisterFormat() error = %v", err)
	}
	payload := binary.LittleEndian.AppendUint64(nil, 100)
	payload = append(payload, 7)
	if err := stream.Write(append(bytes.Clone(payload), 0)); err == nil {
		t.Fatal("Write() accepted partially omitted trailing padding")
	}
	if err := stream.Write(payload); err != nil {
		t.Fatalf("Write() omitted trailing padding error = %v", err)
	}
}

func TestWriterRequiresTimestampInSubscribedFormats(t *testing.T) {
	formats := []Format{
		{Name: "missing", Fields: []Field{{Name: "value", Type: TypeUint8}}},
		{Name: "wrong_type", Fields: []Field{{Name: "timestamp", Type: TypeUint32}}},
		{Name: "array", Fields: []Field{{Name: "timestamp", Type: TypeUint64, ArrayLength: 2}}},
	}
	for _, format := range formats {
		t.Run(format.Name, func(t *testing.T) {
			var destination bytes.Buffer
			writer, err := NewWriter(&destination)
			if err != nil {
				t.Fatalf("NewWriter() error = %v", err)
			}
			if _, err := writer.RegisterFormat(format); err == nil {
				t.Fatal("RegisterFormat() succeeded without scalar uint64_t timestamp")
			}
		})
	}
}
