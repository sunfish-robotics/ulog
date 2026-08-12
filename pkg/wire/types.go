// Package wire defines the byte-level types used by the PX4 ULog file format.
//
// ULog encodes all multi-byte values in little-endian order. A [MessageHeader]
// frames each message and [MessageHeader.Size] bounds its payload. Types ending
// in Header contain only a fixed payload prefix. Types ending in Message
// represent a complete payload; variable-width messages implement
// encoding.BinaryAppender and encoding.BinaryUnmarshaler.
//
// The [PX4 ULog specification] defines the protocol semantics represented here.
//
// [PX4 ULog specification]: https://docs.px4.io/main/en/dev_log/ulog_file_format
package wire

// FileMagic is the seven-byte ULog file signature required at offset zero.
const FileMagic = "ULog\x01\x12\x35"

// FileVersion is the format-version byte emitted by the writer.
const FileVersion uint8 = 1

// SyncMagic is the byte sequence a recovery parser can search for after a
// corrupt message.
const SyncMagic = "\x2f\x73\x13\x20\x25\x0c\xbb\x12"

// PrimitiveType is the case-sensitive type spelling used in a format definition.
type PrimitiveType string

const (
	// PrimitiveTypeInt8 identifies a signed 8-bit integer.
	PrimitiveTypeInt8 PrimitiveType = "int8_t"
	// PrimitiveTypeUint8 identifies an unsigned 8-bit integer.
	PrimitiveTypeUint8 PrimitiveType = "uint8_t"
	// PrimitiveTypeInt16 identifies a signed 16-bit integer.
	PrimitiveTypeInt16 PrimitiveType = "int16_t"
	// PrimitiveTypeUint16 identifies an unsigned 16-bit integer.
	PrimitiveTypeUint16 PrimitiveType = "uint16_t"
	// PrimitiveTypeInt32 identifies a signed 32-bit integer.
	PrimitiveTypeInt32 PrimitiveType = "int32_t"
	// PrimitiveTypeUint32 identifies an unsigned 32-bit integer.
	PrimitiveTypeUint32 PrimitiveType = "uint32_t"
	// PrimitiveTypeInt64 identifies a signed 64-bit integer.
	PrimitiveTypeInt64 PrimitiveType = "int64_t"
	// PrimitiveTypeUint64 identifies an unsigned 64-bit integer.
	PrimitiveTypeUint64 PrimitiveType = "uint64_t"
	// PrimitiveTypeFloat identifies a 32-bit IEEE-754 floating-point value.
	PrimitiveTypeFloat PrimitiveType = "float"
	// PrimitiveTypeDouble identifies a 64-bit IEEE-754 floating-point value.
	PrimitiveTypeDouble PrimitiveType = "double"
	// PrimitiveTypeBool identifies a one-byte boolean value.
	PrimitiveTypeBool PrimitiveType = "bool"
	// PrimitiveTypeChar identifies a one-byte character.
	PrimitiveTypeChar PrimitiveType = "char"
)

// MessageType is the one-byte tag identifying the payload after a
// [MessageHeader].
type MessageType uint8

const (
	// MessageTypeFlagBits identifies a [FlagBitsMessage] payload.
	MessageTypeFlagBits MessageType = 'B'
	// MessageTypeFormat identifies a [FormatMessage] payload.
	MessageTypeFormat MessageType = 'F'
	// MessageTypeInformation identifies an [InformationMessage] payload.
	MessageTypeInformation MessageType = 'I'
	// MessageTypeMultiInformation identifies a [MultiInformationMessage] payload.
	MessageTypeMultiInformation MessageType = 'M'
	// MessageTypeParameter identifies a [ParameterMessage] payload.
	MessageTypeParameter MessageType = 'P'
	// MessageTypeDefaultParameter identifies a [DefaultParameterMessage] payload.
	MessageTypeDefaultParameter MessageType = 'Q'
	// MessageTypeSubscription identifies a [SubscriptionMessage] payload.
	MessageTypeSubscription MessageType = 'A'
	// MessageTypeUnsubscription identifies an [UnsubscriptionMessage] payload.
	MessageTypeUnsubscription MessageType = 'R'
	// MessageTypeData identifies a [DataMessage] payload.
	MessageTypeData MessageType = 'D'
	// MessageTypeLogging identifies a [LoggingMessage] payload.
	MessageTypeLogging MessageType = 'L'
	// MessageTypeTaggedLogging identifies a [TaggedLoggingMessage] payload.
	MessageTypeTaggedLogging MessageType = 'C'
	// MessageTypeSynchronization identifies a [SynchronizationMessage] payload.
	MessageTypeSynchronization MessageType = 'S'
	// MessageTypeDropout identifies a [DropoutMessage] payload.
	MessageTypeDropout MessageType = 'O'
)

// CompatibilityFlags advertises optional features that do not change how an
// older parser reads the rest of the log.
type CompatibilityFlags uint64

const (
	// CompatibilityFlagDefaultParameters indicates that [MessageTypeDefaultParameter] messages are present.
	CompatibilityFlagDefaultParameters CompatibilityFlags = 1 << 0
)

// IncompatibilityFlags advertises features that change how the log must be read.
// A parser must reject any set bit it does not support.
type IncompatibilityFlags uint64

const (
	// IncompatibilityFlagDataAppended indicates that [FlagBitsMessage.AppendedOffsets] contains appended data offsets.
	IncompatibilityFlagDataAppended IncompatibilityFlags = 1 << 0
)

// DefaultParameterTypes identifies the independent configuration scopes to
// which a [DefaultParameterMessage] applies. At least one scope must be set.
type DefaultParameterTypes uint8

const (
	// DefaultParameterSystemWide marks a system-wide default value.
	DefaultParameterSystemWide DefaultParameterTypes = 1 << 0
	// DefaultParameterCurrentConfiguration marks a default for the current configuration.
	DefaultParameterCurrentConfiguration DefaultParameterTypes = 1 << 1
)

// LogLevel is an ASCII-encoded Linux kernel log level.
type LogLevel uint8

const (
	// LogLevelEmergency indicates that the system is unusable.
	LogLevelEmergency LogLevel = '0'
	// LogLevelAlert indicates that action must be taken immediately.
	LogLevelAlert LogLevel = '1'
	// LogLevelCritical indicates a critical condition.
	LogLevelCritical LogLevel = '2'
	// LogLevelError indicates an error condition.
	LogLevelError LogLevel = '3'
	// LogLevelWarning indicates a warning condition.
	LogLevelWarning LogLevel = '4'
	// LogLevelNotice indicates a normal but significant condition.
	LogLevelNotice LogLevel = '5'
	// LogLevelInfo indicates an informational message.
	LogLevelInfo LogLevel = '6'
	// LogLevelDebug indicates a debug message.
	LogLevelDebug LogLevel = '7'
)

// FileHeader is the fixed 16-byte little-endian header at the start of a ULog
// file.
type FileHeader struct {
	// Magic identifies the file as ULog and must equal [FileMagic].
	Magic [7]byte
	// Version is the file format version. [FileVersion] is the current value.
	Version uint8
	// Timestamp is when logging started, in microseconds.
	Timestamp uint64
}

// MessageHeader is the three-byte little-endian frame before every message in
// the definitions and data sections.
type MessageHeader struct {
	// Size is the payload size in bytes, excluding this header.
	Size uint16
	// Type identifies the payload content.
	Type MessageType
}

// FlagBitsMessage tells a parser which optional and incompatible ULog features
// are present. It represents the first 40 payload bytes and must be the first
// message after [FileHeader]. Those bytes can be encoded and decoded directly
// with encoding/binary; parsers must tolerate future trailing bytes. Its message
// type is [MessageTypeFlagBits].
type FlagBitsMessage struct {
	// CompatibilityFlags identifies features compatible with existing parsers.
	CompatibilityFlags CompatibilityFlags
	// IncompatibilityFlags identifies features that require explicit parser support.
	IncompatibilityFlags IncompatibilityFlags
	// AppendedOffsets contains zero-based file offsets for appended data; unused entries are zero.
	AppendedOffsets [3]uint64
}

// FlagBits is the former name of [FlagBitsMessage] and remains for source
// compatibility.
type FlagBits = FlagBitsMessage

// InformationHeader is the fixed prefix of an [InformationMessage].
// [InformationHeader.KeyLength] bytes of key follow it; all remaining payload
// bytes are the value.
type InformationHeader struct {
	// KeyLength is the key length in bytes.
	KeyLength uint8
}

// MultiInformationHeader is the fixed prefix of a [MultiInformationMessage].
// [MultiInformationHeader.IsContinued] is 1 when the value continues the
// previous message with this key.
type MultiInformationHeader struct {
	// IsContinued is 1 when this value continues the previous message with the same key.
	IsContinued uint8
	// KeyLength is the key length in bytes.
	KeyLength uint8
}

// ParameterHeader is the fixed prefix of a [ParameterMessage]. Its key and
// encoded value follow.
type ParameterHeader struct {
	// KeyLength is the key length in bytes.
	KeyLength uint8
}

// DefaultParameterHeader is the fixed prefix of a [DefaultParameterMessage].
// Its key and encoded value follow.
type DefaultParameterHeader struct {
	// Types contains [DefaultParameterTypes] scopes; at least one bit must be set.
	Types DefaultParameterTypes
	// KeyLength is the key length in bytes.
	KeyLength uint8
}

// SubscriptionHeader is the fixed prefix of a [SubscriptionMessage]. The name
// of a previously defined [FormatMessage] follows.
type SubscriptionHeader struct {
	// MultiID identifies an instance of a message format; the first and default instance is zero.
	MultiID uint8
	// MessageID uniquely identifies this subscription in [DataHeader.MessageID].
	MessageID uint16
}

// UnsubscriptionMessage ends the subscription identified by
// [UnsubscriptionMessage.MessageID]. Later [DataMessage] values therefore have
// no active format for that ID. The payload can be encoded and decoded directly
// with encoding/binary. Its message type is
// [MessageTypeUnsubscription].
type UnsubscriptionMessage struct {
	// MessageID identifies the [SubscriptionHeader.MessageID] that will no longer be logged.
	MessageID uint16
}

// Unsubscription is the former name of [UnsubscriptionMessage] and remains for
// source compatibility.
type Unsubscription = UnsubscriptionMessage

// DataHeader is the fixed prefix of a [DataMessage]. The bytes encoded according
// to the selected [FormatMessage] follow.
type DataHeader struct {
	// MessageID identifies the [SubscriptionHeader.MessageID] that defines the following data bytes.
	MessageID uint16
}

// LoggingHeader is the fixed prefix of a [LoggingMessage]. The untagged log text
// follows without a terminating null byte.
type LoggingHeader struct {
	// Level is the [LogLevel] for the message.
	Level LogLevel
	// Timestamp is the message timestamp in microseconds.
	Timestamp uint64
}

// TaggedLoggingHeader is the fixed prefix of a [TaggedLoggingMessage]. The log
// text follows without a terminating null byte.
type TaggedLoggingHeader struct {
	// Level is the [LogLevel] for the message.
	Level LogLevel
	// Tag identifies the source of the message, such as a process, thread, or class.
	Tag uint16
	// Timestamp is the message timestamp in microseconds.
	Timestamp uint64
}

// SynchronizationMessage gives a parser a known byte sequence from which it can
// resume after a corrupt message. It can be encoded and decoded directly with
// encoding/binary. Its message type is [MessageTypeSynchronization].
type SynchronizationMessage struct {
	// Magic is the fixed synchronization sequence in [SyncMagic].
	Magic [8]byte
}

// Synchronization is the former name of [SynchronizationMessage] and remains
// for source compatibility.
type Synchronization = SynchronizationMessage

// DropoutMessage marks a period in which logging messages were lost, often
// because the logging device could not keep up. It can be encoded and decoded
// directly with encoding/binary. Its message type is [MessageTypeDropout].
type DropoutMessage struct {
	// Duration is the period of lost logging messages in milliseconds.
	Duration uint16
}

// Dropout is the former name of [DropoutMessage] and remains for source
// compatibility.
type Dropout = DropoutMessage
