package dataset_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/sunfish-robotics/ulog"
	"github.com/sunfish-robotics/ulog/pkg/dataset"
	"github.com/sunfish-robotics/ulog/pkg/wire"
)

type csvSample struct {
	Timestamp uint64
	Signed    int8
	Unsigned  uint32
	Single    float32
	Double    float64
	Armed     bool
}

func TestDatasetWriteCSVWritesColumnsInWireOrder(t *testing.T) {
	instance := readCSVDataset(t)

	var destination bytes.Buffer
	if err := instance.WriteCSV(&destination); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}

	want := "timestamp,signed,unsigned,single,double,armed\n" +
		"42,-7,9,1.2345678,-3.5000000000000004,true\n"
	if got := destination.String(); got != want {
		t.Errorf("WriteCSV() = %q, want %q", got, want)
	}
}

func TestDatasetWriteCSVWritesNullsAsEmptyFields(t *testing.T) {
	fixture := newFixture(t, 0)
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

	file, err := dataset.Read(bytes.NewReader(fixture.bytes()))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	instance, err := file.Dataset("versioned", 0)
	if err != nil {
		t.Fatalf("Dataset() error = %v", err)
	}

	var destination bytes.Buffer
	if err := instance.WriteCSV(&destination); err != nil {
		t.Fatalf("WriteCSV() error = %v", err)
	}

	want := "timestamp,old_value,new_value\n1,1.5,10\n2,2.5,\n"
	if got := destination.String(); got != want {
		t.Errorf("WriteCSV() = %q, want %q", got, want)
	}
}

func TestDatasetWriteCSVReturnsDestinationErrors(t *testing.T) {
	instance := readCSVDataset(t)
	want := errors.New("CSV destination failed")

	err := instance.WriteCSV(errorWriter{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("WriteCSV() error = %v, want %v", err, want)
	}
}

func readCSVDataset(t *testing.T) *dataset.Dataset {
	t.Helper()
	var source bytes.Buffer
	writer, err := ulog.NewWriter(&source)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := ulog.Register[csvSample](writer, ulog.WithFormatName("csv_sample"))
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := stream.Write(csvSample{
		Timestamp: 42,
		Signed:    -7,
		Unsigned:  9,
		Single:    1.2345678,
		Double:    -3.5000000000000004,
		Armed:     true,
	}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	file, err := dataset.Read(bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	instance, err := file.Dataset("csv_sample", 0)
	if err != nil {
		t.Fatalf("Dataset() error = %v", err)
	}
	return instance
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
