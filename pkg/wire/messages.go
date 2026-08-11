package wire

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const maxMessagePayloadSize = 1<<16 - 1

var (
	_ encoding.BinaryAppender    = FormatMessage{}
	_ encoding.BinaryUnmarshaler = (*FormatMessage)(nil)
	_ encoding.BinaryAppender    = InformationMessage{}
	_ encoding.BinaryUnmarshaler = (*InformationMessage)(nil)
	_ encoding.BinaryAppender    = MultiInformationMessage{}
	_ encoding.BinaryUnmarshaler = (*MultiInformationMessage)(nil)
	_ encoding.BinaryAppender    = ParameterMessage{}
	_ encoding.BinaryUnmarshaler = (*ParameterMessage)(nil)
	_ encoding.BinaryAppender    = DefaultParameterMessage{}
	_ encoding.BinaryUnmarshaler = (*DefaultParameterMessage)(nil)
	_ encoding.BinaryAppender    = SubscriptionMessage{}
	_ encoding.BinaryUnmarshaler = (*SubscriptionMessage)(nil)
	_ encoding.BinaryAppender    = DataMessage{}
	_ encoding.BinaryUnmarshaler = (*DataMessage)(nil)
	_ encoding.BinaryAppender    = LoggingMessage{}
	_ encoding.BinaryUnmarshaler = (*LoggingMessage)(nil)
	_ encoding.BinaryAppender    = TaggedLoggingMessage{}
	_ encoding.BinaryUnmarshaler = (*TaggedLoggingMessage)(nil)
)

// FormatMessage is the complete payload identified by [MessageTypeFormat].
type FormatMessage struct {
	// Format contains the message name followed by its semicolon-delimited field declarations.
	Format string
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m FormatMessage) AppendBinary(dst []byte) ([]byte, error) {
	if m.Format == "" {
		return dst, errors.New("format must not be empty")
	}
	if err := validatePayloadSize(len(m.Format)); err != nil {
		return dst, err
	}

	return append(dst, m.Format...), nil
}

// UnmarshalBinary decodes a ULog format payload into m.
func (m *FormatMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal format message into nil receiver")
	}
	if len(data) == 0 {
		return errors.New("format must not be empty")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	*m = FormatMessage{Format: string(data)}
	return nil
}

// InformationMessage is the complete payload identified by
// [MessageTypeInformation].
type InformationMessage struct {
	// Key contains the value type and key name.
	Key string
	// Value contains the key's encoded value bytes.
	Value []byte
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m InformationMessage) AppendBinary(dst []byte) ([]byte, error) {
	keyLength, err := validateKeyValue(m.Key, len(m.Value), binary.Size(InformationHeader{}))
	if err != nil {
		return dst, err
	}

	header := InformationHeader{KeyLength: keyLength}
	return appendKeyValue(dst, header, m.Key, m.Value)
}

// UnmarshalBinary decodes a ULog information payload into m.
func (m *InformationMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal information message into nil receiver")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	header, tail, err := decodePrefix[InformationHeader](data)
	if err != nil {
		return fmt.Errorf("decode information header: %w", err)
	}
	key, value, err := decodeKeyValue(header.KeyLength, tail)
	if err != nil {
		return err
	}

	*m = InformationMessage{Key: key, Value: value}
	return nil
}

// MultiInformationMessage is the complete payload identified by
// [MessageTypeMultiInformation].
type MultiInformationMessage struct {
	// IsContinued is 1 when Value continues the previous message with the same Key.
	IsContinued uint8
	// Key contains the value type and key name.
	Key string
	// Value contains this part of the key's encoded value bytes.
	Value []byte
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m MultiInformationMessage) AppendBinary(dst []byte) ([]byte, error) {
	if err := validateContinuation(m.IsContinued); err != nil {
		return dst, err
	}
	keyLength, err := validateKeyValue(m.Key, len(m.Value), binary.Size(MultiInformationHeader{}))
	if err != nil {
		return dst, err
	}

	header := MultiInformationHeader{
		IsContinued: m.IsContinued,
		KeyLength:   keyLength,
	}
	return appendKeyValue(dst, header, m.Key, m.Value)
}

// UnmarshalBinary decodes a ULog multi-information payload into m.
func (m *MultiInformationMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal multi-information message into nil receiver")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	header, tail, err := decodePrefix[MultiInformationHeader](data)
	if err != nil {
		return fmt.Errorf("decode multi-information header: %w", err)
	}
	if err := validateContinuation(header.IsContinued); err != nil {
		return err
	}
	key, value, err := decodeKeyValue(header.KeyLength, tail)
	if err != nil {
		return err
	}

	*m = MultiInformationMessage{
		IsContinued: header.IsContinued,
		Key:         key,
		Value:       value,
	}
	return nil
}

// ParameterMessage is the complete payload identified by [MessageTypeParameter].
type ParameterMessage struct {
	// Key contains the parameter type and name.
	Key string
	// Value contains the parameter's encoded value bytes.
	Value []byte
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m ParameterMessage) AppendBinary(dst []byte) ([]byte, error) {
	keyLength, err := validateKeyValue(m.Key, len(m.Value), binary.Size(ParameterHeader{}))
	if err != nil {
		return dst, err
	}

	header := ParameterHeader{KeyLength: keyLength}
	return appendKeyValue(dst, header, m.Key, m.Value)
}

// UnmarshalBinary decodes a ULog parameter payload into m.
func (m *ParameterMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal parameter message into nil receiver")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	header, tail, err := decodePrefix[ParameterHeader](data)
	if err != nil {
		return fmt.Errorf("decode parameter header: %w", err)
	}
	key, value, err := decodeKeyValue(header.KeyLength, tail)
	if err != nil {
		return err
	}

	*m = ParameterMessage{Key: key, Value: value}
	return nil
}

// DefaultParameterMessage is the complete payload identified by
// [MessageTypeDefaultParameter].
type DefaultParameterMessage struct {
	// Types contains [DefaultParameterTypes] scopes; at least one bit must be set.
	Types DefaultParameterTypes
	// Key contains the parameter type and name.
	Key string
	// Value contains the parameter's encoded default value bytes.
	Value []byte
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m DefaultParameterMessage) AppendBinary(dst []byte) ([]byte, error) {
	if m.Types == 0 {
		return dst, errors.New("default parameter types must not be zero")
	}
	keyLength, err := validateKeyValue(m.Key, len(m.Value), binary.Size(DefaultParameterHeader{}))
	if err != nil {
		return dst, err
	}

	header := DefaultParameterHeader{
		Types:     m.Types,
		KeyLength: keyLength,
	}
	return appendKeyValue(dst, header, m.Key, m.Value)
}

// UnmarshalBinary decodes a ULog default-parameter payload into m.
func (m *DefaultParameterMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal default-parameter message into nil receiver")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	header, tail, err := decodePrefix[DefaultParameterHeader](data)
	if err != nil {
		return fmt.Errorf("decode default-parameter header: %w", err)
	}
	if header.Types == 0 {
		return errors.New("default parameter types must not be zero")
	}
	key, value, err := decodeKeyValue(header.KeyLength, tail)
	if err != nil {
		return err
	}

	*m = DefaultParameterMessage{Types: header.Types, Key: key, Value: value}
	return nil
}

// SubscriptionMessage is the complete payload identified by
// [MessageTypeSubscription].
type SubscriptionMessage struct {
	// MultiID identifies an instance of a message format; the first and default instance is zero.
	MultiID uint8
	// MessageID uniquely identifies this subscription in [DataMessage.MessageID].
	MessageID uint16
	// MessageName identifies a previously defined [FormatMessage].
	MessageName string
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m SubscriptionMessage) AppendBinary(dst []byte) ([]byte, error) {
	if m.MessageName == "" {
		return dst, errors.New("subscription message name must not be empty")
	}
	if err := validatePayloadSize(binary.Size(SubscriptionHeader{}) + len(m.MessageName)); err != nil {
		return dst, err
	}

	header := SubscriptionHeader{MultiID: m.MultiID, MessageID: m.MessageID}
	encoded, err := binary.Append(dst, binary.LittleEndian, header)
	if err != nil {
		return dst, fmt.Errorf("append subscription header: %w", err)
	}
	return append(encoded, m.MessageName...), nil
}

// UnmarshalBinary decodes a ULog subscription payload into m.
func (m *SubscriptionMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal subscription message into nil receiver")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	header, tail, err := decodePrefix[SubscriptionHeader](data)
	if err != nil {
		return fmt.Errorf("decode subscription header: %w", err)
	}
	if len(tail) == 0 {
		return errors.New("subscription message name must not be empty")
	}

	*m = SubscriptionMessage{
		MultiID:     header.MultiID,
		MessageID:   header.MessageID,
		MessageName: string(tail),
	}
	return nil
}

// DataMessage is the complete payload identified by [MessageTypeData].
type DataMessage struct {
	// MessageID identifies the [SubscriptionMessage.MessageID] that defines Data.
	MessageID uint16
	// Data contains bytes encoded according to the subscription's format.
	Data []byte
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m DataMessage) AppendBinary(dst []byte) ([]byte, error) {
	if err := validatePayloadSize(binary.Size(DataHeader{}) + len(m.Data)); err != nil {
		return dst, err
	}

	header := DataHeader{MessageID: m.MessageID}
	encoded, err := binary.Append(dst, binary.LittleEndian, header)
	if err != nil {
		return dst, fmt.Errorf("append data header: %w", err)
	}
	return append(encoded, m.Data...), nil
}

// UnmarshalBinary decodes a ULog data payload into m.
func (m *DataMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal data message into nil receiver")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	header, tail, err := decodePrefix[DataHeader](data)
	if err != nil {
		return fmt.Errorf("decode data header: %w", err)
	}

	*m = DataMessage{MessageID: header.MessageID, Data: bytes.Clone(tail)}
	return nil
}

// LoggingMessage is the complete payload identified by [MessageTypeLogging].
type LoggingMessage struct {
	// Level is the [LogLevel] for the message.
	Level LogLevel
	// Timestamp is the message timestamp in microseconds.
	Timestamp uint64
	// Message contains the log text without a terminating null byte.
	Message string
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m LoggingMessage) AppendBinary(dst []byte) ([]byte, error) {
	if err := validateLogLevel(m.Level); err != nil {
		return dst, err
	}
	if err := validatePayloadSize(binary.Size(LoggingHeader{}) + len(m.Message)); err != nil {
		return dst, err
	}

	header := LoggingHeader{Level: m.Level, Timestamp: m.Timestamp}
	encoded, err := binary.Append(dst, binary.LittleEndian, header)
	if err != nil {
		return dst, fmt.Errorf("append logging header: %w", err)
	}
	return append(encoded, m.Message...), nil
}

// UnmarshalBinary decodes a ULog logging payload into m.
func (m *LoggingMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal logging message into nil receiver")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	header, tail, err := decodePrefix[LoggingHeader](data)
	if err != nil {
		return fmt.Errorf("decode logging header: %w", err)
	}
	if err := validateLogLevel(header.Level); err != nil {
		return err
	}

	*m = LoggingMessage{Level: header.Level, Timestamp: header.Timestamp, Message: string(tail)}
	return nil
}

// TaggedLoggingMessage is the complete payload identified by
// [MessageTypeTaggedLogging].
type TaggedLoggingMessage struct {
	// Level is the [LogLevel] for the message.
	Level LogLevel
	// Tag identifies the source of the message, such as a process, thread, or class.
	Tag uint16
	// Timestamp is the message timestamp in microseconds.
	Timestamp uint64
	// Message contains the log text without a terminating null byte.
	Message string
}

// AppendBinary appends the ULog payload representation of m to dst.
func (m TaggedLoggingMessage) AppendBinary(dst []byte) ([]byte, error) {
	if err := validateLogLevel(m.Level); err != nil {
		return dst, err
	}
	if err := validatePayloadSize(binary.Size(TaggedLoggingHeader{}) + len(m.Message)); err != nil {
		return dst, err
	}

	header := TaggedLoggingHeader{Level: m.Level, Tag: m.Tag, Timestamp: m.Timestamp}
	encoded, err := binary.Append(dst, binary.LittleEndian, header)
	if err != nil {
		return dst, fmt.Errorf("append tagged logging header: %w", err)
	}
	return append(encoded, m.Message...), nil
}

// UnmarshalBinary decodes a ULog tagged-logging payload into m.
func (m *TaggedLoggingMessage) UnmarshalBinary(data []byte) error {
	if m == nil {
		return errors.New("unmarshal tagged-logging message into nil receiver")
	}
	if err := validatePayloadSize(len(data)); err != nil {
		return err
	}

	header, tail, err := decodePrefix[TaggedLoggingHeader](data)
	if err != nil {
		return fmt.Errorf("decode tagged logging header: %w", err)
	}
	if err := validateLogLevel(header.Level); err != nil {
		return err
	}

	*m = TaggedLoggingMessage{
		Level:     header.Level,
		Tag:       header.Tag,
		Timestamp: header.Timestamp,
		Message:   string(tail),
	}
	return nil
}

func appendKeyValue[T any](dst []byte, header T, key string, value []byte) ([]byte, error) {
	encoded, err := binary.Append(dst, binary.LittleEndian, header)
	if err != nil {
		return dst, fmt.Errorf("append key-value header: %w", err)
	}
	encoded = append(encoded, key...)
	return append(encoded, value...), nil
}

func decodePrefix[T any](data []byte) (T, []byte, error) {
	var header T
	size := binary.Size(header)
	if len(data) < size {
		return header, nil, io.ErrUnexpectedEOF
	}
	if _, err := binary.Decode(data[:size], binary.LittleEndian, &header); err != nil {
		return header, nil, err
	}
	return header, data[size:], nil
}

func decodeKeyValue(keyLength uint8, data []byte) (string, []byte, error) {
	if keyLength == 0 {
		return "", nil, errors.New("key must not be empty")
	}
	if int(keyLength) > len(data) {
		return "", nil, fmt.Errorf("key length %d exceeds remaining payload size %d", keyLength, len(data))
	}

	return string(data[:keyLength]), bytes.Clone(data[keyLength:]), nil
}

func validateContinuation(value uint8) error {
	if value > 1 {
		return fmt.Errorf("is_continued must be 0 or 1, got %d", value)
	}
	return nil
}

func validateKeyValue(key string, valueSize, prefixSize int) (uint8, error) {
	if key == "" {
		return 0, errors.New("key must not be empty")
	}
	if len(key) > 1<<8-1 {
		return 0, fmt.Errorf("key is %d bytes: maximum is %d", len(key), 1<<8-1)
	}
	if err := validatePayloadSize(prefixSize + len(key) + valueSize); err != nil {
		return 0, err
	}
	return uint8(len(key)), nil // #nosec G115 -- len(key) is bounded above.
}

func validateLogLevel(level LogLevel) error {
	if level < LogLevelEmergency || level > LogLevelDebug {
		return fmt.Errorf("invalid log level %q", level)
	}
	return nil
}

func validatePayloadSize(size int) error {
	if size > maxMessagePayloadSize {
		return fmt.Errorf("message payload is %d bytes: maximum is %d", size, maxMessagePayloadSize)
	}
	return nil
}
