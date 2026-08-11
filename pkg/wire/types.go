// Package wire defines the byte-level types used by the PX4 ULog file format.
//
// ULog encodes all multi-byte values in little-endian order. Message-specific
// structs contain only the fixed portion of a message payload; callers read the
// common [MessageHeader] first and use [MessageHeader.Size] to bound any
// variable tail.
package wire

// FileMagic is the seven-byte sequence stored in [FileHeader.Magic].
const FileMagic = "ULog\x01\x12\x35"

// FileVersion is the current value of [FileHeader.Version].
const FileVersion uint8 = 1

// SyncMagic is the fixed value of [Synchronization.Magic].
const SyncMagic = "\x2f\x73\x13\x20\x25\x0c\xbb\x12"

// PrimitiveType identifies a primitive type used in a format definition.
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

// MessageType identifies the payload following a [MessageHeader].
type MessageType uint8

const (
	// MessageTypeFlagBits identifies a [FlagBits] payload.
	MessageTypeFlagBits MessageType = 'B'
	// MessageTypeFormat defines a logged message using [PrimitiveType] names.
	MessageTypeFormat MessageType = 'F'
	// MessageTypeInformation identifies a payload beginning with [InformationHeader].
	MessageTypeInformation MessageType = 'I'
	// MessageTypeMultiInformation identifies a payload beginning with [MultiInformationHeader].
	MessageTypeMultiInformation MessageType = 'M'
	// MessageTypeParameter identifies a payload beginning with [ParameterHeader].
	MessageTypeParameter MessageType = 'P'
	// MessageTypeDefaultParameter identifies a payload beginning with [DefaultParameterHeader].
	MessageTypeDefaultParameter MessageType = 'Q'
	// MessageTypeSubscription identifies a payload beginning with [SubscriptionHeader].
	MessageTypeSubscription MessageType = 'A'
	// MessageTypeUnsubscription identifies an [Unsubscription] payload.
	MessageTypeUnsubscription MessageType = 'R'
	// MessageTypeData identifies a payload beginning with [DataHeader].
	MessageTypeData MessageType = 'D'
	// MessageTypeLogging identifies a payload beginning with [LoggingHeader].
	MessageTypeLogging MessageType = 'L'
	// MessageTypeTaggedLogging identifies a payload beginning with [TaggedLoggingHeader].
	MessageTypeTaggedLogging MessageType = 'C'
	// MessageTypeSynchronization identifies a [Synchronization] payload.
	MessageTypeSynchronization MessageType = 'S'
	// MessageTypeDropout identifies a [Dropout] payload.
	MessageTypeDropout MessageType = 'O'
)

// CompatibilityFlags identifies features that older parsers may safely ignore.
type CompatibilityFlags uint64

const (
	// CompatibilityFlagDefaultParameters indicates that [MessageTypeDefaultParameter] messages are present.
	CompatibilityFlagDefaultParameters CompatibilityFlags = 1 << 0
)

// IncompatibilityFlags identifies features that require explicit parser support.
type IncompatibilityFlags uint64

const (
	// IncompatibilityFlagDataAppended indicates that [FlagBits.AppendedOffsets] contains appended data offsets.
	IncompatibilityFlagDataAppended IncompatibilityFlags = 1 << 0
)

// DefaultParameterTypes identifies the scopes to which a default value applies.
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

// FileHeader is the fixed 16-byte header at the start of a ULog file.
type FileHeader struct {
	// Magic identifies the file as ULog and must equal [FileMagic].
	Magic [7]byte
	// Version is the file format version. [FileVersion] is the current value.
	Version uint8
	// Timestamp is when logging started, in microseconds.
	Timestamp uint64
}

// MessageHeader precedes every message in the definitions and data sections.
type MessageHeader struct {
	// Size is the payload size in bytes, excluding this header.
	Size uint16
	// Type identifies the payload content.
	Type MessageType
}

// FlagBits is the fixed 40-byte payload of a flag-bits message.
type FlagBits struct {
	// CompatibilityFlags identifies features compatible with existing parsers.
	CompatibilityFlags CompatibilityFlags
	// IncompatibilityFlags identifies features that require explicit parser support.
	IncompatibilityFlags IncompatibilityFlags
	// AppendedOffsets contains zero-based file offsets for appended data; unused entries are zero.
	AppendedOffsets [3]uint64
}

// InformationHeader precedes an information message's key and value bytes.
// [InformationHeader.KeyLength] bytes of key follow it; all remaining payload
// bytes are the value.
type InformationHeader struct {
	// KeyLength is the key length in bytes.
	KeyLength uint8
}

// MultiInformationHeader precedes a multi-information message's key and value.
// [MultiInformationHeader.IsContinued] is 1 when the value continues the
// previous message with this key.
type MultiInformationHeader struct {
	// IsContinued is 1 when this value continues the previous message with the same key.
	IsContinued uint8
	// KeyLength is the key length in bytes.
	KeyLength uint8
}

// ParameterHeader precedes a parameter message's key and value bytes.
type ParameterHeader struct {
	// KeyLength is the key length in bytes.
	KeyLength uint8
}

// DefaultParameterHeader precedes a default parameter message's key and value.
type DefaultParameterHeader struct {
	// Types contains [DefaultParameterTypes] scopes; at least one bit must be set.
	Types DefaultParameterTypes
	// KeyLength is the key length in bytes.
	KeyLength uint8
}

// SubscriptionHeader precedes the message name in a subscription payload.
type SubscriptionHeader struct {
	// MultiID identifies an instance of a message format; the first and default instance is zero.
	MultiID uint8
	// MessageID uniquely identifies this subscription in [DataHeader.MessageID].
	MessageID uint16
}

// Unsubscription is the complete payload of an unsubscription message.
type Unsubscription struct {
	// MessageID identifies the [SubscriptionHeader.MessageID] that will no longer be logged.
	MessageID uint16
}

// DataHeader precedes the format-defined bytes in a logged data payload.
type DataHeader struct {
	// MessageID identifies the [SubscriptionHeader.MessageID] that defines the following data bytes.
	MessageID uint16
}

// LoggingHeader precedes the message bytes in an untagged logging payload.
type LoggingHeader struct {
	// Level is the [LogLevel] for the message.
	Level LogLevel
	// Timestamp is the message timestamp in microseconds.
	Timestamp uint64
}

// TaggedLoggingHeader precedes the message bytes in a tagged logging payload.
type TaggedLoggingHeader struct {
	// Level is the [LogLevel] for the message.
	Level LogLevel
	// Tag identifies the source of the message, such as a process, thread, or class.
	Tag uint16
	// Timestamp is the message timestamp in microseconds.
	Timestamp uint64
}

// Synchronization is the complete payload of a synchronization message.
type Synchronization struct {
	// Magic is the fixed synchronization sequence in [SyncMagic].
	Magic [8]byte
}

// Dropout is the complete payload of a dropout message. [Dropout.Duration] is
// in milliseconds.
type Dropout struct {
	// Duration is the period of lost logging messages in milliseconds.
	Duration uint16
}
