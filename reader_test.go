package ulog

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/sunfish-robotics/ulog/pkg/wire"
)

func TestReaderResolvesDynamicRecords(t *testing.T) {
	data := newULogFixture(t, 42)
	data.message(t, wire.MessageTypeFormat, wire.FormatMessage{Format: "sample:int16_t x;float y;"})
	data.message(t, wire.MessageTypeFormat, wire.FormatMessage{Format: "telemetry:uint64_t timestamp;float[2] q;sample[2] samples;uint8_t _padding0;"})
	data.message(t, wire.MessageTypeSubscription, wire.SubscriptionMessage{
		MultiID:     3,
		MessageID:   17,
		MessageName: "telemetry",
	})

	payload := make([]byte, 0, 31)
	payload = binary.LittleEndian.AppendUint64(payload, 0x0102030405060708)
	payload = binary.LittleEndian.AppendUint32(payload, math.Float32bits(1.5))
	payload = binary.LittleEndian.AppendUint32(payload, math.Float32bits(-2.25))
	payload = append(payload, 0x85, 0xff)
	payload = binary.LittleEndian.AppendUint32(payload, math.Float32bits(3.5))
	payload = binary.LittleEndian.AppendUint16(payload, 456)
	payload = binary.LittleEndian.AppendUint32(payload, math.Float32bits(-4.75))
	payload = append(payload, 0)
	data.message(t, wire.MessageTypeData, wire.DataMessage{MessageID: 17, Data: payload})

	reader, err := NewReader(bytes.NewReader(data.bytes()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if got := reader.Header().Timestamp; got != 42 {
		t.Errorf("Header().Timestamp = %d, want 42", got)
	}
	if !reader.Next() {
		t.Fatalf("Next() = false, error = %v", reader.Err())
	}

	record := reader.Record()
	if got, want := record.Name(), "telemetry"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if got, want := record.MultiID(), uint8(3); got != want {
		t.Errorf("MultiID() = %d, want %d", got, want)
	}
	if got, want := record.MessageID(), uint16(17); got != want {
		t.Errorf("MessageID() = %d, want %d", got, want)
	}

	wantValues := []FieldValue{
		{Name: "timestamp", Type: TypeUint64, Value: uint64(0x0102030405060708)},
		{Name: "q[0]", Type: TypeFloat32, Value: float32(1.5)},
		{Name: "q[1]", Type: TypeFloat32, Value: float32(-2.25)},
		{Name: "samples[0].x", Type: TypeInt16, Value: int16(-123)},
		{Name: "samples[0].y", Type: TypeFloat32, Value: float32(3.5)},
		{Name: "samples[1].x", Type: TypeInt16, Value: int16(456)},
		{Name: "samples[1].y", Type: TypeFloat32, Value: float32(-4.75)},
	}
	values, err := record.Values()
	if err != nil {
		t.Fatalf("Values() error = %v", err)
	}
	if !reflect.DeepEqual(values, wantValues) {
		t.Errorf("Values() = %#v, want %#v", values, wantValues)
	}

	value, err := record.Value("samples[1].x")
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if value != int16(456) {
		t.Errorf("Value() = %#v, want int16(456)", value)
	}

	copied := record.Bytes()
	copied[0] = 0
	if got := record.Bytes()[0]; got == 0 {
		t.Error("Bytes() aliases the record payload")
	}

	if reader.Next() {
		t.Fatal("second Next() = true, want false")
	}
	if err := reader.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
}

func TestReaderAcceptsRecordsWithMissingTrailingFields(t *testing.T) {
	data := newULogFixture(t, 0)
	data.message(t, wire.MessageTypeFormat, wire.FormatMessage{Format: "versioned:uint64_t timestamp;float old_value;uint32_t new_value;"})
	data.message(t, wire.MessageTypeSubscription, wire.SubscriptionMessage{MessageID: 1, MessageName: "versioned"})

	payload := binary.LittleEndian.AppendUint64(nil, 99)
	payload = binary.LittleEndian.AppendUint32(payload, math.Float32bits(12.5))
	data.message(t, wire.MessageTypeData, wire.DataMessage{MessageID: 1, Data: payload})

	reader, err := NewReader(bytes.NewReader(data.bytes()))
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
	if got, want := len(values), 2; got != want {
		t.Fatalf("len(Values()) = %d, want %d", got, want)
	}
	if _, err := reader.Record().Value("new_value"); err == nil {
		t.Fatal("Value(new_value) succeeded for an omitted trailing field")
	}
}

func TestReaderRejectsUnknownDataMessageID(t *testing.T) {
	data := newULogFixture(t, 0)
	data.message(t, wire.MessageTypeData, wire.DataMessage{MessageID: 99})

	reader, err := NewReader(bytes.NewReader(data.bytes()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if reader.Next() {
		t.Fatal("Next() = true, want false")
	}
	if err := reader.Err(); err == nil || !strings.Contains(err.Error(), "message ID 99") {
		t.Fatalf("Err() = %v, want unknown message ID error", err)
	}
}

func TestReaderRejectsPartiallyEncodedFields(t *testing.T) {
	data := newULogFixture(t, 0)
	data.message(t, wire.MessageTypeFormat, wire.FormatMessage{Format: "partial:uint32_t value;"})
	data.message(t, wire.MessageTypeSubscription, wire.SubscriptionMessage{MessageID: 1, MessageName: "partial"})
	data.message(t, wire.MessageTypeData, wire.DataMessage{MessageID: 1, Data: []byte{1, 2}})

	reader, err := NewReader(bytes.NewReader(data.bytes()))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if !reader.Next() {
		t.Fatalf("Next() = false, error = %v", reader.Err())
	}
	if _, err := reader.Record().Values(); err == nil {
		t.Fatal("Values() succeeded for a partially encoded field")
	}
}

type ulogFixture struct {
	data []byte
}

func newULogFixture(t *testing.T, timestamp uint64) *ulogFixture {
	t.Helper()

	var magic [7]byte
	copy(magic[:], wire.FileMagic)
	data, err := binary.Append(nil, binary.LittleEndian, wire.FileHeader{
		Magic:     magic,
		Version:   wire.FileVersion,
		Timestamp: timestamp,
	})
	if err != nil {
		t.Fatalf("append file header: %v", err)
	}

	fixture := &ulogFixture{data: data}
	flagBits, err := binary.Append(nil, binary.LittleEndian, wire.FlagBitsMessage{})
	if err != nil {
		t.Fatalf("append flag bits: %v", err)
	}
	fixture.rawMessage(t, wire.MessageTypeFlagBits, flagBits)
	return fixture
}

func (f *ulogFixture) message(t *testing.T, messageType wire.MessageType, message encoding.BinaryAppender) {
	t.Helper()
	payload, err := message.AppendBinary(nil)
	if err != nil {
		t.Fatalf("append %q message: %v", messageType, err)
	}
	f.rawMessage(t, messageType, payload)
}

func (f *ulogFixture) rawMessage(t *testing.T, messageType wire.MessageType, payload []byte) {
	t.Helper()
	if len(payload) > math.MaxUint16 {
		t.Fatalf("payload size %d exceeds ULog limit", len(payload))
	}
	header := wire.MessageHeader{Size: uint16(len(payload)), Type: messageType} // #nosec G115 -- bounded above.
	var err error
	f.data, err = binary.Append(f.data, binary.LittleEndian, header)
	if err != nil {
		t.Fatalf("append message header: %v", err)
	}
	f.data = append(f.data, payload...)
}

func (f *ulogFixture) bytes() []byte {
	return bytes.Clone(f.data)
}
