package ulog

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/sunfish-robotics/ulog/pkg/wire"
)

func TestReadBuildsTypedDatasets(t *testing.T) {
	var source bytes.Buffer
	writer, err := NewWriter(&source)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := Register[typedPoint](writer, WithMultiID(3))
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	for _, sample := range []typedPoint{{X: -2, Y: 1.25}, {X: 7, Y: -3.5}} {
		if err := stream.Write(sample); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	file, err := Read(bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	dataset, err := file.Dataset("typed_point", 3)
	if err != nil {
		t.Fatalf("Dataset() error = %v", err)
	}
	if got, want := dataset.Len(), 2; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}

	x, ok := dataset.Column("x")
	if !ok {
		t.Fatal("Column(x) not found")
	}
	if got, want := x.Values(), []int16{-2, 7}; !reflect.DeepEqual(got, want) {
		t.Errorf("x.Values() = %#v, want %#v", got, want)
	}
	y, ok := dataset.Column("y")
	if !ok {
		t.Fatal("Column(y) not found")
	}
	if got, want := y.Values(), []float32{1.25, -3.5}; !reflect.DeepEqual(got, want) {
		t.Errorf("y.Values() = %#v, want %#v", got, want)
	}
	if got := y.Type(); got != TypeFloat32 {
		t.Errorf("y.Type() = %q, want %q", got, TypeFloat32)
	}
	if got, valid := y.Value(1); !valid || got != float32(-3.5) {
		t.Errorf("y.Value(1) = %#v, %t, want -3.5, true", got, valid)
	}
}

func TestReadMarksMissingTrailingValuesNull(t *testing.T) {
	fixture := newULogFixture(t, 0)
	fixture.message(t, wire.MessageTypeFormat, wire.FormatMessage{
		Format: "versioned:uint64_t timestamp;float old_value;uint32_t new_value;",
	})
	fixture.message(t, wire.MessageTypeSubscription, wire.SubscriptionMessage{MessageID: 1, MessageName: "versioned"})

	complete := binary.LittleEndian.AppendUint64(nil, 1)
	complete = binary.LittleEndian.AppendUint32(complete, math.Float32bits(1.5))
	complete = binary.LittleEndian.AppendUint32(complete, 10)
	fixture.message(t, wire.MessageTypeData, wire.DataMessage{MessageID: 1, Data: complete})

	older := binary.LittleEndian.AppendUint64(nil, 2)
	older = binary.LittleEndian.AppendUint32(older, math.Float32bits(2.5))
	fixture.message(t, wire.MessageTypeData, wire.DataMessage{MessageID: 1, Data: older})

	file, err := Read(bytes.NewReader(fixture.bytes()))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	dataset, err := file.Dataset("versioned", 0)
	if err != nil {
		t.Fatalf("Dataset() error = %v", err)
	}
	column, ok := dataset.Column("new_value")
	if !ok {
		t.Fatal("Column(new_value) not found")
	}
	if got, want := column.Values(), []uint32{10, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("Values() = %#v, want %#v", got, want)
	}
	if _, valid := column.Value(0); !valid {
		t.Error("Value(0) is null, want valid")
	}
	if value, valid := column.Value(1); valid || value != nil {
		t.Errorf("Value(1) = %#v, %t, want nil, false", value, valid)
	}
}

func TestFileDatasetsPreserveFirstSeenOrder(t *testing.T) {
	var source bytes.Buffer
	writer, err := NewWriter(&source)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	first, err := Register[typedPoint](writer, WithMultiID(2))
	if err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	second, err := Register[typedPoint](writer, WithMultiID(1))
	if err != nil {
		t.Fatalf("Register(second) error = %v", err)
	}
	if err := second.Write(typedPoint{}); err != nil {
		t.Fatalf("second.Write() error = %v", err)
	}
	if err := first.Write(typedPoint{}); err != nil {
		t.Fatalf("first.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	file, err := Read(bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	datasets := file.Datasets()
	if got, want := []uint8{datasets[0].MultiID(), datasets[1].MultiID()}, []uint8{1, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("dataset order = %v, want %v", got, want)
	}
}
