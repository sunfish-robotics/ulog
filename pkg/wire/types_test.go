package wire

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestMagicConstants(t *testing.T) {
	t.Parallel()

	if got, want := hex.EncodeToString([]byte(FileMagic)), "554c6f67011235"; got != want {
		t.Fatalf("FileMagic = %s, want %s", got, want)
	}
	if got, want := hex.EncodeToString([]byte(SyncMagic)), "2f731320250cbb12"; got != want {
		t.Fatalf("SyncMagic = %s, want %s", got, want)
	}
	if FileVersion != 1 {
		t.Fatalf("FileVersion = %d, want 1", FileVersion)
	}
}

func TestPrimitiveTypes(t *testing.T) {
	t.Parallel()

	want := map[PrimitiveType]string{
		PrimitiveTypeInt8:   "int8_t",
		PrimitiveTypeUint8:  "uint8_t",
		PrimitiveTypeInt16:  "int16_t",
		PrimitiveTypeUint16: "uint16_t",
		PrimitiveTypeInt32:  "int32_t",
		PrimitiveTypeUint32: "uint32_t",
		PrimitiveTypeInt64:  "int64_t",
		PrimitiveTypeUint64: "uint64_t",
		PrimitiveTypeFloat:  "float",
		PrimitiveTypeDouble: "double",
		PrimitiveTypeBool:   "bool",
		PrimitiveTypeChar:   "char",
	}

	if len(want) != 12 {
		t.Fatalf("got %d primitive types, want 12", len(want))
	}
	for typ, spelling := range want {
		if string(typ) != spelling {
			t.Errorf("primitive type %q = %q, want %q", spelling, typ, spelling)
		}
	}
}

func TestMessageTypes(t *testing.T) {
	t.Parallel()

	want := map[MessageType]byte{
		MessageTypeFlagBits:         'B',
		MessageTypeFormat:           'F',
		MessageTypeInformation:      'I',
		MessageTypeMultiInformation: 'M',
		MessageTypeParameter:        'P',
		MessageTypeDefaultParameter: 'Q',
		MessageTypeSubscription:     'A',
		MessageTypeUnsubscription:   'R',
		MessageTypeData:             'D',
		MessageTypeLogging:          'L',
		MessageTypeTaggedLogging:    'C',
		MessageTypeSynchronization:  'S',
		MessageTypeDropout:          'O',
	}

	if len(want) != 13 {
		t.Fatalf("got %d message types, want 13", len(want))
	}
	for typ, tag := range want {
		if byte(typ) != tag {
			t.Errorf("message type %q = %q, want %q", tag, typ, tag)
		}
	}
}

func TestWireEnums(t *testing.T) {
	t.Parallel()

	if CompatibilityFlagDefaultParameters != 1<<0 {
		t.Errorf("CompatibilityFlagDefaultParameters = %#x, want %#x", CompatibilityFlagDefaultParameters, 1<<0)
	}
	if IncompatibilityFlagDataAppended != 1<<0 {
		t.Errorf("IncompatibilityFlagDataAppended = %#x, want %#x", IncompatibilityFlagDataAppended, 1<<0)
	}
	if DefaultParameterSystemWide != 1<<0 {
		t.Errorf("DefaultParameterSystemWide = %#x, want %#x", DefaultParameterSystemWide, 1<<0)
	}
	if DefaultParameterCurrentConfiguration != 1<<1 {
		t.Errorf("DefaultParameterCurrentConfiguration = %#x, want %#x", DefaultParameterCurrentConfiguration, 1<<1)
	}

	levels := map[LogLevel]byte{
		LogLevelEmergency: '0',
		LogLevelAlert:     '1',
		LogLevelCritical:  '2',
		LogLevelError:     '3',
		LogLevelWarning:   '4',
		LogLevelNotice:    '5',
		LogLevelInfo:      '6',
		LogLevelDebug:     '7',
	}
	if len(levels) != 8 {
		t.Fatalf("got %d log levels, want 8", len(levels))
	}
	for level, value := range levels {
		if byte(level) != value {
			t.Errorf("log level %q = %q, want %q", value, level, value)
		}
	}
}

func TestFixedWireLayouts(t *testing.T) {
	t.Parallel()

	var magic [7]byte
	copy(magic[:], FileMagic)
	var syncMagic [8]byte
	copy(syncMagic[:], SyncMagic)

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name: "file header",
			value: FileHeader{
				Magic:     magic,
				Version:   FileVersion,
				Timestamp: 0x0102030405060708,
			},
			want: "554c6f67011235010807060504030201",
		},
		{
			name:  "message header",
			value: MessageHeader{Size: 0x1234, Type: MessageTypeData},
			want:  "341244",
		},
		{
			name: "flag bits",
			value: FlagBits{
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
		{name: "information header", value: InformationHeader{KeyLength: 0xab}, want: "ab"},
		{
			name:  "multi information header",
			value: MultiInformationHeader{IsContinued: 1, KeyLength: 0xab},
			want:  "01ab",
		},
		{name: "parameter header", value: ParameterHeader{KeyLength: 0xab}, want: "ab"},
		{
			name: "default parameter header",
			value: DefaultParameterHeader{
				Types:     DefaultParameterSystemWide | DefaultParameterCurrentConfiguration,
				KeyLength: 0xab,
			},
			want: "03ab",
		},
		{
			name:  "subscription header",
			value: SubscriptionHeader{MultiID: 0x12, MessageID: 0x3456},
			want:  "125634",
		},
		{name: "unsubscription", value: Unsubscription{MessageID: 0x3456}, want: "5634"},
		{name: "data header", value: DataHeader{MessageID: 0x3456}, want: "5634"},
		{
			name:  "logging header",
			value: LoggingHeader{Level: LogLevelError, Timestamp: 0x0102030405060708},
			want:  "330807060504030201",
		},
		{
			name: "tagged logging header",
			value: TaggedLoggingHeader{
				Level:     LogLevelError,
				Tag:       0x3456,
				Timestamp: 0x0102030405060708,
			},
			want: "3356340807060504030201",
		},
		{name: "synchronization", value: Synchronization{Magic: syncMagic}, want: "2f731320250cbb12"},
		{name: "dropout", value: Dropout{Duration: 0x1234}, want: "3412"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got bytes.Buffer
			if err := binary.Write(&got, binary.LittleEndian, tt.value); err != nil {
				t.Fatalf("encode fixed wire fields: %v", err)
			}
			if gotHex := hex.EncodeToString(got.Bytes()); gotHex != tt.want {
				t.Errorf("wire bytes = %s, want %s", gotHex, tt.want)
			}
		})
	}
}
