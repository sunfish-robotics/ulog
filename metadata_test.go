package ulog

import (
	"bytes"
	"reflect"
	"testing"
	"time"
)

func TestWriterAndFilePreserveAnalysisMetadata(t *testing.T) {
	var source bytes.Buffer
	writer, err := NewWriter(&source)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	if err := writer.WriteInformation("system_name", "sunfish"); err != nil {
		t.Fatalf("WriteInformation(string) error = %v", err)
	}
	if err := writer.WriteInformation("build_id", uint32(0x10203040)); err != nil {
		t.Fatalf("WriteInformation(uint32) error = %v", err)
	}
	if err := writer.WriteParameter("gain", float32(1.25)); err != nil {
		t.Fatalf("WriteParameter() error = %v", err)
	}
	stream, err := Register[typedPoint](writer)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := stream.Write(typedPoint{X: 3, Y: 2.5}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.WriteLog(LogLevelInfo, 1234, "ready"); err != nil {
		t.Fatalf("WriteLog() error = %v", err)
	}
	if err := writer.WriteDropout(25 * time.Millisecond); err != nil {
		t.Fatalf("WriteDropout() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	file, err := Read(bytes.NewReader(source.Bytes()))
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	wantInformation := []KeyValue{
		{Name: "system_name", Type: TypeChar, ArrayLength: 7, Value: "sunfish"},
		{Name: "build_id", Type: TypeUint32, Value: uint32(0x10203040)},
	}
	if got := file.Information(); !reflect.DeepEqual(got, wantInformation) {
		t.Errorf("Information() = %#v, want %#v", got, wantInformation)
	}
	wantParameters := []KeyValue{{Name: "gain", Type: TypeFloat32, Value: float32(1.25)}}
	if got := file.Parameters(); !reflect.DeepEqual(got, wantParameters) {
		t.Errorf("Parameters() = %#v, want %#v", got, wantParameters)
	}
	wantLogs := []LogEntry{{Level: LogLevelInfo, Timestamp: 1234, Message: "ready"}}
	if got := file.Logs(); !reflect.DeepEqual(got, wantLogs) {
		t.Errorf("Logs() = %#v, want %#v", got, wantLogs)
	}
	wantDropouts := []Dropout{{Duration: 25 * time.Millisecond}}
	if got := file.Dropouts(); !reflect.DeepEqual(got, wantDropouts) {
		t.Errorf("Dropouts() = %#v, want %#v", got, wantDropouts)
	}
}

func TestWriterRejectsInitialMetadataAfterDataStarts(t *testing.T) {
	var source bytes.Buffer
	writer, err := NewWriter(&source)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	stream, err := Register[typedPoint](writer)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := stream.Write(typedPoint{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := writer.WriteInformation("late", uint32(1)); err == nil {
		t.Fatal("WriteInformation() succeeded after data started")
	}
	if err := writer.WriteParameter("late", int32(1)); err == nil {
		t.Fatal("WriteParameter() succeeded after data started")
	}
}

func TestWriterValidatesMetadataMessages(t *testing.T) {
	var destination bytes.Buffer
	writer, err := NewWriter(&destination)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "nil parameter", call: func() error { return writer.WriteParameter("gain", nil) }},
		{name: "unsupported parameter type", call: func() error { return writer.WriteParameter("gain", uint32(1)) }},
		{name: "invalid log level", call: func() error { return writer.WriteLog(LogLevel('8'), 0, "bad") }},
		{name: "fractional millisecond dropout", call: func() error { return writer.WriteDropout(1500 * time.Microsecond) }},
		{name: "oversized dropout", call: func() error { return writer.WriteDropout(65536 * time.Millisecond) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("call succeeded, want validation error")
			}
		})
	}
}
