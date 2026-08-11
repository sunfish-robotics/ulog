package ulog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/sunfish-robotics/ulog/pkg/wire"
)

// Header contains the fixed information at the start of a ULog file.
type Header struct {
	Version   uint8
	Timestamp uint64
}

// Reader reads data records from a ULog stream. It resolves format definitions
// and subscriptions while advancing through the stream.
type Reader struct {
	source        io.Reader
	header        Header
	formats       map[string]Format
	subscriptions map[uint16]subscription
	record        Record
	err           error
}

type subscription struct {
	messageID uint16
	multiID   uint8
	format    Format
	layout    []layoutField
}

type layoutField struct {
	name   string
	typeID Type
	offset int
	size   int
	hidden bool
}

// NewReader reads and validates the fixed ULog file header from source.
func NewReader(source io.Reader) (*Reader, error) {
	if source == nil {
		return nil, errors.New("nil ULog source")
	}

	data := make([]byte, binary.Size(wire.FileHeader{}))
	if _, err := io.ReadFull(source, data); err != nil {
		return nil, fmt.Errorf("read ULog file header: %w", err)
	}

	var fileHeader wire.FileHeader
	if _, err := binary.Decode(data, binary.LittleEndian, &fileHeader); err != nil {
		return nil, fmt.Errorf("decode ULog file header: %w", err)
	}
	if string(fileHeader.Magic[:]) != wire.FileMagic {
		return nil, fmt.Errorf("invalid ULog file magic %x", fileHeader.Magic)
	}
	if fileHeader.Version != wire.FileVersion {
		return nil, fmt.Errorf("unsupported ULog file version %d", fileHeader.Version)
	}

	return &Reader{
		source: source,
		header: Header{
			Version:   fileHeader.Version,
			Timestamp: fileHeader.Timestamp,
		},
		formats:       make(map[string]Format),
		subscriptions: make(map[uint16]subscription),
	}, nil
}

// Header returns the file header read by [NewReader].
func (r *Reader) Header() Header {
	return r.header
}

// Next advances to the next data record. Definition and state messages are
// consumed internally. It returns false at the end of the stream or on error.
func (r *Reader) Next() bool {
	if r == nil || r.err != nil {
		return false
	}

	for {
		messageType, payload, err := readMessage(r.source)
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			r.err = err
			return false
		}

		record, found, err := r.consume(messageType, payload)
		if err != nil {
			r.err = err
			return false
		}
		if found {
			r.record = record
			return true
		}
	}
}

// Record returns the record selected by the most recent successful [Reader.Next].
func (r *Reader) Record() Record {
	return r.record
}

// Err returns the first error encountered while advancing the reader.
func (r *Reader) Err() error {
	if r == nil {
		return errors.New("nil ULog reader")
	}
	return r.err
}

func readMessage(source io.Reader) (wire.MessageType, []byte, error) {
	var headerBytes [3]byte
	n, err := io.ReadFull(source, headerBytes[:])
	if errors.Is(err, io.EOF) && n == 0 {
		return 0, nil, io.EOF
	}
	if err != nil {
		return 0, nil, fmt.Errorf("read ULog message header: %w", err)
	}

	size := binary.LittleEndian.Uint16(headerBytes[:2])
	payload := make([]byte, int(size))
	if _, err := io.ReadFull(source, payload); err != nil {
		return 0, nil, fmt.Errorf("read ULog %q message payload: %w", headerBytes[2], err)
	}
	return wire.MessageType(headerBytes[2]), payload, nil
}

func (r *Reader) consume(messageType wire.MessageType, payload []byte) (Record, bool, error) {
	switch messageType {
	case wire.MessageTypeFlagBits:
		if len(payload) != binary.Size(wire.FlagBitsMessage{}) {
			return Record{}, false, fmt.Errorf("flag-bits payload has size %d, want %d", len(payload), binary.Size(wire.FlagBitsMessage{}))
		}
		var flags wire.FlagBitsMessage
		if _, err := binary.Decode(payload, binary.LittleEndian, &flags); err != nil {
			return Record{}, false, fmt.Errorf("decode flag bits: %w", err)
		}
		unknown := flags.IncompatibilityFlags &^ wire.IncompatibilityFlagDataAppended
		if unknown != 0 {
			return Record{}, false, fmt.Errorf("unsupported ULog incompatibility flags %#x", unknown)
		}
		if flags.IncompatibilityFlags&wire.IncompatibilityFlagDataAppended != 0 {
			return Record{}, false, errors.New("ULog appended data sections are not supported")
		}
	case wire.MessageTypeFormat:
		var message wire.FormatMessage
		if err := message.UnmarshalBinary(payload); err != nil {
			return Record{}, false, fmt.Errorf("decode format message: %w", err)
		}
		format, err := ParseFormat(message.Format)
		if err != nil {
			return Record{}, false, err
		}
		if existing, ok := r.formats[format.Name]; ok && existing.String() != format.String() {
			return Record{}, false, fmt.Errorf("format %q is redefined incompatibly", format.Name)
		}
		r.formats[format.Name] = cloneFormat(*format)
	case wire.MessageTypeSubscription:
		var message wire.SubscriptionMessage
		if err := message.UnmarshalBinary(payload); err != nil {
			return Record{}, false, fmt.Errorf("decode subscription message: %w", err)
		}
		format, ok := r.formats[message.MessageName]
		if !ok {
			return Record{}, false, fmt.Errorf("subscription %d references unknown format %q", message.MessageID, message.MessageName)
		}
		if _, exists := r.subscriptions[message.MessageID]; exists {
			return Record{}, false, fmt.Errorf("message ID %d is already subscribed", message.MessageID)
		}
		layout, err := resolveLayout(format, r.formats)
		if err != nil {
			return Record{}, false, fmt.Errorf("resolve format %q: %w", format.Name, err)
		}
		r.subscriptions[message.MessageID] = subscription{
			messageID: message.MessageID,
			multiID:   message.MultiID,
			format:    cloneFormat(format),
			layout:    layout,
		}
	case wire.MessageTypeUnsubscription:
		if len(payload) != binary.Size(wire.UnsubscriptionMessage{}) {
			return Record{}, false, fmt.Errorf("unsubscription payload has size %d, want %d", len(payload), binary.Size(wire.UnsubscriptionMessage{}))
		}
		var message wire.UnsubscriptionMessage
		if _, err := binary.Decode(payload, binary.LittleEndian, &message); err != nil {
			return Record{}, false, fmt.Errorf("decode unsubscription message: %w", err)
		}
		delete(r.subscriptions, message.MessageID)
	case wire.MessageTypeData:
		var message wire.DataMessage
		if err := message.UnmarshalBinary(payload); err != nil {
			return Record{}, false, fmt.Errorf("decode data message: %w", err)
		}
		subscription, ok := r.subscriptions[message.MessageID]
		if !ok {
			return Record{}, false, fmt.Errorf("data references unknown message ID %d", message.MessageID)
		}
		return Record{
			messageID: subscription.messageID,
			multiID:   subscription.multiID,
			format:    cloneFormat(subscription.format),
			layout:    append([]layoutField(nil), subscription.layout...),
			data:      bytes.Clone(message.Data),
		}, true, nil
	}

	return Record{}, false, nil
}

func resolveLayout(root Format, formats map[string]Format) ([]layoutField, error) {
	layout, _, err := resolveFormatLayout(root.Name, "", 0, false, formats, make(map[string]bool))
	return layout, err
}

func resolveFormatLayout(
	name string,
	prefix string,
	offset int,
	hidden bool,
	formats map[string]Format,
	active map[string]bool,
) ([]layoutField, int, error) {
	format, ok := formats[name]
	if !ok {
		return nil, 0, fmt.Errorf("unknown nested format %q", name)
	}
	if active[name] {
		return nil, 0, fmt.Errorf("format cycle involving %q", name)
	}
	active[name] = true
	defer delete(active, name)

	start := offset
	var layout []layoutField
	for _, field := range format.Fields {
		count := field.ArrayLength
		if count == 0 {
			count = 1
		}
		for i := range count {
			fieldName := field.Name
			if field.ArrayLength > 0 {
				fieldName += fmt.Sprintf("[%d]", i)
			}
			path := fieldName
			if prefix != "" {
				path = prefix + "." + fieldName
			}
			fieldHidden := hidden || strings.HasPrefix(field.Name, "_padding")

			if size, primitive := primitiveSize(field.Type); primitive {
				layout = append(layout, layoutField{
					name:   path,
					typeID: field.Type,
					offset: offset,
					size:   size,
					hidden: fieldHidden,
				})
				offset += size
				continue
			}

			nested, nestedSize, err := resolveFormatLayout(string(field.Type), path, offset, fieldHidden, formats, active)
			if err != nil {
				return nil, 0, err
			}
			layout = append(layout, nested...)
			offset += nestedSize
		}
	}
	return layout, offset - start, nil
}

func cloneFormat(format Format) Format {
	format.Fields = append([]Field(nil), format.Fields...)
	return format
}

// FieldValue is one dynamically decoded scalar field. Arrays and nested
// formats use paths such as "q[0]" and "position.x".
type FieldValue struct {
	Name  string
	Type  Type
	Value any
}

// Record is one format-resolved ULog data message.
type Record struct {
	messageID uint16
	multiID   uint8
	format    Format
	layout    []layoutField
	data      []byte
}

// Name returns the subscribed format name.
func (r Record) Name() string { return r.format.Name }

// MessageID returns the runtime subscription identifier.
func (r Record) MessageID() uint16 { return r.messageID }

// MultiID returns the subscribed instance identifier.
func (r Record) MultiID() uint8 { return r.multiID }

// Format returns an independent copy of the record's dynamic schema.
func (r Record) Format() Format { return cloneFormat(r.format) }

// Bytes returns an independent copy of the format-defined payload.
func (r Record) Bytes() []byte { return bytes.Clone(r.data) }

// Values decodes every available non-padding scalar field in wire order.
// Missing trailing fields are omitted to support compatible schema extension.
func (r Record) Values() ([]FieldValue, error) {
	values := make([]FieldValue, 0, len(r.layout))
	for _, field := range r.layout {
		if len(r.data) <= field.offset {
			break
		}
		end := field.offset + field.size
		if len(r.data) < end {
			return nil, fmt.Errorf("field %q is truncated: have %d of %d bytes", field.name, len(r.data)-field.offset, field.size)
		}
		if field.hidden {
			continue
		}

		value, err := decodePrimitive(field.typeID, r.data[field.offset:end])
		if err != nil {
			return nil, fmt.Errorf("decode field %q: %w", field.name, err)
		}
		values = append(values, FieldValue{Name: field.name, Type: field.typeID, Value: value})
	}
	return values, nil
}

// Value decodes a scalar field by its flattened path.
func (r Record) Value(name string) (any, error) {
	values, err := r.Values()
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.Name == name {
			return value.Value, nil
		}
	}
	return nil, fmt.Errorf("field %q is not available in record %q", name, r.Name())
}

func decodePrimitive(typeID Type, data []byte) (any, error) {
	switch typeID {
	case TypeInt8:
		return decodeSigned[int8](data)
	case TypeUint8, TypeChar:
		return data[0], nil
	case TypeBool:
		return data[0] != 0, nil
	case TypeInt16:
		return decodeSigned[int16](data)
	case TypeUint16:
		return binary.LittleEndian.Uint16(data), nil
	case TypeInt32:
		return decodeSigned[int32](data)
	case TypeUint32:
		return binary.LittleEndian.Uint32(data), nil
	case TypeInt64:
		return decodeSigned[int64](data)
	case TypeUint64:
		return binary.LittleEndian.Uint64(data), nil
	case TypeFloat32:
		return math.Float32frombits(binary.LittleEndian.Uint32(data)), nil
	case TypeFloat64:
		return math.Float64frombits(binary.LittleEndian.Uint64(data)), nil
	default:
		return nil, fmt.Errorf("unsupported primitive type %q", typeID)
	}
}

func decodeSigned[T int8 | int16 | int32 | int64](data []byte) (T, error) {
	var value T
	if _, err := binary.Decode(data, binary.LittleEndian, &value); err != nil {
		return 0, err
	}
	return value, nil
}
