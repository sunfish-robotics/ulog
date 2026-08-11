package wire

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"encoding/hex"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestVariableMessageCodecs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     encoding.BinaryAppender
		newValue  func() encoding.BinaryUnmarshaler
		wantValue any
		wire      []byte
	}{
		{
			name:      "format",
			value:     FormatMessage{Format: "foo:uint64_t timestamp;"},
			newValue:  func() encoding.BinaryUnmarshaler { return new(FormatMessage) },
			wantValue: &FormatMessage{Format: "foo:uint64_t timestamp;"},
			wire:      []byte("foo:uint64_t timestamp;"),
		},
		{
			name:      "information",
			value:     InformationMessage{Key: "ab", Value: []byte{0xcd, 0xef}},
			newValue:  func() encoding.BinaryUnmarshaler { return new(InformationMessage) },
			wantValue: &InformationMessage{Key: "ab", Value: []byte{0xcd, 0xef}},
			wire:      []byte{0x02, 'a', 'b', 0xcd, 0xef},
		},
		{
			name: "multi information",
			value: MultiInformationMessage{
				IsContinued: 1,
				Key:         "ab",
				Value:       []byte{0xcd, 0xef},
			},
			newValue: func() encoding.BinaryUnmarshaler { return new(MultiInformationMessage) },
			wantValue: &MultiInformationMessage{
				IsContinued: 1,
				Key:         "ab",
				Value:       []byte{0xcd, 0xef},
			},
			wire: []byte{0x01, 0x02, 'a', 'b', 0xcd, 0xef},
		},
		{
			name:      "parameter",
			value:     ParameterMessage{Key: "ab", Value: []byte{0xcd, 0xef}},
			newValue:  func() encoding.BinaryUnmarshaler { return new(ParameterMessage) },
			wantValue: &ParameterMessage{Key: "ab", Value: []byte{0xcd, 0xef}},
			wire:      []byte{0x02, 'a', 'b', 0xcd, 0xef},
		},
		{
			name: "default parameter",
			value: DefaultParameterMessage{
				Types: DefaultParameterSystemWide | DefaultParameterCurrentConfiguration,
				Key:   "ab",
				Value: []byte{0xcd, 0xef},
			},
			newValue: func() encoding.BinaryUnmarshaler { return new(DefaultParameterMessage) },
			wantValue: &DefaultParameterMessage{
				Types: DefaultParameterSystemWide | DefaultParameterCurrentConfiguration,
				Key:   "ab",
				Value: []byte{0xcd, 0xef},
			},
			wire: []byte{0x03, 0x02, 'a', 'b', 0xcd, 0xef},
		},
		{
			name: "subscription",
			value: SubscriptionMessage{
				MultiID:     0x12,
				MessageID:   0x3456,
				MessageName: "abc",
			},
			newValue: func() encoding.BinaryUnmarshaler { return new(SubscriptionMessage) },
			wantValue: &SubscriptionMessage{
				MultiID:     0x12,
				MessageID:   0x3456,
				MessageName: "abc",
			},
			wire: []byte{0x12, 0x56, 0x34, 'a', 'b', 'c'},
		},
		{
			name:      "data",
			value:     DataMessage{MessageID: 0x3456, Data: []byte{0xab, 0xcd}},
			newValue:  func() encoding.BinaryUnmarshaler { return new(DataMessage) },
			wantValue: &DataMessage{MessageID: 0x3456, Data: []byte{0xab, 0xcd}},
			wire:      []byte{0x56, 0x34, 0xab, 0xcd},
		},
		{
			name: "logging",
			value: LoggingMessage{
				Level:     LogLevelError,
				Timestamp: 0x0102030405060708,
				Message:   "hi",
			},
			newValue: func() encoding.BinaryUnmarshaler { return new(LoggingMessage) },
			wantValue: &LoggingMessage{
				Level:     LogLevelError,
				Timestamp: 0x0102030405060708,
				Message:   "hi",
			},
			wire: []byte{0x33, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01, 'h', 'i'},
		},
		{
			name: "tagged logging",
			value: TaggedLoggingMessage{
				Level:     LogLevelError,
				Tag:       0x3456,
				Timestamp: 0x0102030405060708,
				Message:   "hi",
			},
			newValue: func() encoding.BinaryUnmarshaler { return new(TaggedLoggingMessage) },
			wantValue: &TaggedLoggingMessage{
				Level:     LogLevelError,
				Tag:       0x3456,
				Timestamp: 0x0102030405060708,
				Message:   "hi",
			},
			wire: []byte{0x33, 0x56, 0x34, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01, 'h', 'i'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefix := []byte{0xaa, 0xbb}
			got, err := tt.value.AppendBinary(bytes.Clone(prefix))
			if err != nil {
				t.Fatalf("append binary: %v", err)
			}
			if !bytes.Equal(got[:len(prefix)], prefix) {
				t.Fatalf("AppendBinary changed prefix: got %x, want %x", got[:len(prefix)], prefix)
			}
			if !bytes.Equal(got[len(prefix):], tt.wire) {
				t.Errorf("wire bytes = %x, want %x", got[len(prefix):], tt.wire)
			}

			wireCopy := bytes.Clone(tt.wire)
			decoded := tt.newValue()
			if err := decoded.UnmarshalBinary(wireCopy); err != nil {
				t.Fatalf("unmarshal binary: %v", err)
			}
			if !reflect.DeepEqual(decoded, tt.wantValue) {
				t.Errorf("decoded value = %#v, want %#v", decoded, tt.wantValue)
			}

			for i := range wireCopy {
				wireCopy[i] ^= 0xff
			}
			if !reflect.DeepEqual(decoded, tt.wantValue) {
				t.Errorf("decoded value retained input: got %#v, want %#v", decoded, tt.wantValue)
			}
		})
	}
}

func TestFixedMessageTypesUseEncodingBinary(t *testing.T) {
	t.Parallel()

	var syncMagic [8]byte
	copy(syncMagic[:], SyncMagic)

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name: "flag bits",
			value: FlagBitsMessage{
				CompatibilityFlags:   0x0102030405060708,
				IncompatibilityFlags: 0x1112131415161718,
				AppendedOffsets: [3]uint64{
					0x2122232425262728,
					0x3132333435363738,
					0x4142434445464748,
				},
			},
			want: "08070605040302011817161514131211282726252423222138373635343332314847464544434241",
		},
		{name: "unsubscription", value: UnsubscriptionMessage{MessageID: 0x3456}, want: "5634"},
		{name: "synchronization", value: SynchronizationMessage{Magic: syncMagic}, want: "2f731320250cbb12"},
		{name: "dropout", value: DropoutMessage{Duration: 0x1234}, want: "3412"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := binary.Append(nil, binary.LittleEndian, tt.value)
			if err != nil {
				t.Fatalf("append with encoding/binary: %v", err)
			}
			if gotHex := hex.EncodeToString(got); gotHex != tt.want {
				t.Errorf("wire bytes = %s, want %s", gotHex, tt.want)
			}

			wireBytes, err := hex.DecodeString(tt.want)
			if err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			decoded := reflect.New(reflect.TypeOf(tt.value))
			n, err := binary.Decode(wireBytes, binary.LittleEndian, decoded.Interface())
			if err != nil {
				t.Fatalf("decode with encoding/binary: %v", err)
			}
			if n != len(wireBytes) {
				t.Errorf("decoded %d bytes, want %d", n, len(wireBytes))
			}
			if gotValue := decoded.Elem().Interface(); !reflect.DeepEqual(gotValue, tt.value) {
				t.Errorf("decoded value = %#v, want %#v", gotValue, tt.value)
			}
		})
	}
}

func TestVariableMessageCodecsRejectInvalidData(t *testing.T) {
	t.Parallel()

	tooLarge := strings.Repeat("x", math.MaxUint16+1)
	tooLargePayload := make([]byte, math.MaxUint16+1)
	tests := []struct {
		name   string
		value  encoding.BinaryAppender
		target encoding.BinaryUnmarshaler
		wire   []byte
	}{
		{name: "oversized format", value: FormatMessage{Format: tooLarge}},
		{name: "oversized decoded information", target: new(InformationMessage), wire: tooLargePayload},
		{name: "empty format", value: FormatMessage{}},
		{name: "oversized information key", value: InformationMessage{Key: strings.Repeat("x", math.MaxUint8+1)}},
		{name: "information key exceeds payload", target: new(InformationMessage), wire: []byte{0x02, 'a'}},
		{name: "invalid continuation", value: MultiInformationMessage{IsContinued: 2, Key: "a"}},
		{name: "invalid decoded continuation", target: new(MultiInformationMessage), wire: []byte{0x02, 0x01, 'a'}},
		{name: "truncated parameter", target: new(ParameterMessage)},
		{name: "missing default type", value: DefaultParameterMessage{Key: "a"}},
		{name: "missing decoded default type", target: new(DefaultParameterMessage), wire: []byte{0x00, 0x01, 'a'}},
		{name: "empty subscription name", value: SubscriptionMessage{}},
		{name: "truncated subscription", target: new(SubscriptionMessage), wire: []byte{0x01, 0x02}},
		{name: "truncated data", target: new(DataMessage), wire: []byte{0x01}},
		{name: "invalid log level", value: LoggingMessage{Level: '8'}},
		{name: "truncated logging", target: new(LoggingMessage), wire: make([]byte, 8)},
		{name: "invalid tagged log level", value: TaggedLoggingMessage{Level: '8'}},
		{name: "truncated tagged logging", target: new(TaggedLoggingMessage), wire: make([]byte, 10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.value != nil {
				prefix := []byte{0xaa, 0xbb}
				got, err := tt.value.AppendBinary(bytes.Clone(prefix))
				if err == nil {
					t.Fatal("AppendBinary returned nil error")
				}
				if !bytes.Equal(got, prefix) {
					t.Errorf("AppendBinary modified destination on error: got %x, want %x", got, prefix)
				}
			}

			if tt.target != nil {
				if err := tt.target.UnmarshalBinary(tt.wire); err == nil {
					t.Fatal("UnmarshalBinary returned nil error")
				}
			}
		})
	}
}

func TestVariableMessageCodecBounds(t *testing.T) {
	t.Parallel()

	t.Run("maximum payload", func(t *testing.T) {
		t.Parallel()

		message := FormatMessage{Format: strings.Repeat("x", math.MaxUint16)}
		got, err := message.AppendBinary(nil)
		if err != nil {
			t.Fatalf("append maximum payload: %v", err)
		}
		if len(got) != math.MaxUint16 {
			t.Errorf("payload size = %d, want %d", len(got), math.MaxUint16)
		}

		var decoded FormatMessage
		if err := decoded.UnmarshalBinary(got); err != nil {
			t.Fatalf("decode maximum payload: %v", err)
		}
		if decoded != message {
			t.Error("maximum payload did not decode to its original value")
		}
	})

	t.Run("maximum key", func(t *testing.T) {
		t.Parallel()

		message := InformationMessage{
			Key:   strings.Repeat("k", math.MaxUint8),
			Value: []byte{},
		}
		got, err := message.AppendBinary(nil)
		if err != nil {
			t.Fatalf("append maximum key: %v", err)
		}
		if len(got) != math.MaxUint8+1 {
			t.Errorf("payload size = %d, want %d", len(got), math.MaxUint8+1)
		}

		var decoded InformationMessage
		if err := decoded.UnmarshalBinary(got); err != nil {
			t.Fatalf("decode maximum key: %v", err)
		}
		if !reflect.DeepEqual(decoded, message) {
			t.Errorf("decoded value = %#v, want %#v", decoded, message)
		}
	})
}

func TestUnmarshalBinaryIsAtomic(t *testing.T) {
	t.Parallel()

	message := InformationMessage{Key: "existing", Value: []byte{0x12, 0x34}}
	if err := message.UnmarshalBinary([]byte{0x02, 'a'}); err == nil {
		t.Fatal("UnmarshalBinary returned nil error")
	}

	want := InformationMessage{Key: "existing", Value: []byte{0x12, 0x34}}
	if !reflect.DeepEqual(message, want) {
		t.Errorf("message changed after failed decode: got %#v, want %#v", message, want)
	}
}
