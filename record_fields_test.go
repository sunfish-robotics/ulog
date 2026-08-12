package ulog

import (
	"bytes"
	"reflect"
	"testing"
)

func TestRecordFieldsReturnsFlattenedScalarSchema(t *testing.T) {
	type position struct {
		X float32
		Y float32
	}
	type sample struct {
		Timestamp uint64
		Position  position
		Values    [2]int16
		Padding   [2]uint8 `ulog:"_padding"`
	}

	var source bytes.Buffer
	writer, err := NewWriter(&source)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := Register[sample](writer)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := stream.Write(sample{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reader, err := NewReader(bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if !reader.Next() {
		t.Fatalf("Next() = false, error = %v", reader.Err())
	}
	want := []ScalarField{
		{Name: "timestamp", Type: TypeUint64},
		{Name: "position.x", Type: TypeFloat32},
		{Name: "position.y", Type: TypeFloat32},
		{Name: "values[0]", Type: TypeInt16},
		{Name: "values[1]", Type: TypeInt16},
	}
	if got := reader.Record().Fields(); !reflect.DeepEqual(got, want) {
		t.Errorf("Fields() = %#v, want %#v", got, want)
	}
}
