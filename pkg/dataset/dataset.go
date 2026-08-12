package dataset

import (
	"errors"
	"fmt"
	"io"

	"github.com/sunfish-robotics/ulog"
)

// File is an eager, column-oriented view of a ULog stream.
type File struct {
	header      ulog.Header
	datasets    []*Dataset
	byKey       map[datasetKey]*Dataset
	information []ulog.KeyValue
	parameters  []ulog.KeyValue
	defaults    []ulog.DefaultParameter
	logs        []ulog.LogEntry
	dropouts    []ulog.Dropout
}

type datasetKey struct {
	name    string
	multiID uint8
}

// Read consumes source and groups its data records into datasets.
func Read(source io.Reader) (*File, error) {
	reader, err := ulog.NewReader(source)
	if err != nil {
		return nil, err
	}
	file := &File{
		header: reader.Header(),
		byKey:  make(map[datasetKey]*Dataset),
	}
	for reader.Next() {
		record := reader.Record()
		key := datasetKey{name: record.Name(), multiID: record.MultiID()}
		dataset, ok := file.byKey[key]
		if !ok {
			dataset = newDataset(record)
			file.byKey[key] = dataset
			file.datasets = append(file.datasets, dataset)
		}
		if err := dataset.appendRecord(record); err != nil {
			return nil, fmt.Errorf("append %q multi ID %d record: %w", record.Name(), record.MultiID(), err)
		}
	}
	if err := reader.Err(); err != nil {
		return nil, err
	}
	file.information = reader.Information()
	file.parameters = reader.Parameters()
	file.defaults = reader.DefaultParameters()
	file.logs = reader.Logs()
	file.dropouts = reader.Dropouts()
	return file, nil
}

// Header returns the file header.
func (f *File) Header() ulog.Header {
	if f == nil {
		return ulog.Header{}
	}
	return f.header
}

// Information returns typed information entries in file order.
func (f *File) Information() []ulog.KeyValue {
	if f == nil {
		return nil
	}
	return cloneKeyValues(f.information)
}

// Parameters returns parameter entries in file order, including changes.
func (f *File) Parameters() []ulog.KeyValue {
	if f == nil {
		return nil
	}
	return cloneKeyValues(f.parameters)
}

// DefaultParameters returns default parameter entries in file order.
func (f *File) DefaultParameters() []ulog.DefaultParameter {
	if f == nil {
		return nil
	}
	return cloneDefaultParameters(f.defaults)
}

// Logs returns tagged and untagged text messages in file order.
func (f *File) Logs() []ulog.LogEntry {
	if f == nil {
		return nil
	}
	return append([]ulog.LogEntry(nil), f.logs...)
}

// Dropouts returns logging dropouts in file order.
func (f *File) Dropouts() []ulog.Dropout {
	if f == nil {
		return nil
	}
	return append([]ulog.Dropout(nil), f.dropouts...)
}

// Datasets returns datasets in order of their first data record.
func (f *File) Datasets() []*Dataset {
	if f == nil {
		return nil
	}
	return append([]*Dataset(nil), f.datasets...)
}

// Dataset returns the dataset for a format name and instance identifier.
func (f *File) Dataset(name string, multiID uint8) (*Dataset, error) {
	if f == nil {
		return nil, errors.New("nil ULog file")
	}
	dataset, ok := f.byKey[datasetKey{name: name, multiID: multiID}]
	if !ok {
		return nil, fmt.Errorf("dataset %q multi ID %d not found", name, multiID)
	}
	return dataset, nil
}

// Dataset contains all records for one format name and instance identifier.
type Dataset struct {
	name        string
	multiID     uint8
	format      ulog.Format
	columns     []Column
	columnIndex map[string]int
	length      int
}

func newDataset(record ulog.Record) *Dataset {
	dataset := &Dataset{
		name:        record.Name(),
		multiID:     record.MultiID(),
		format:      record.Format(),
		columnIndex: make(map[string]int),
	}
	for _, field := range record.Fields() {
		dataset.columnIndex[field.Name] = len(dataset.columns)
		dataset.columns = append(dataset.columns, newColumn(field.Name, field.Type))
	}
	return dataset
}

func (d *Dataset) appendRecord(record ulog.Record) error {
	values, err := record.Values()
	if err != nil {
		return err
	}
	valueIndex := 0
	for i := range d.columns {
		column := &d.columns[i]
		if valueIndex < len(values) && values[valueIndex].Name == column.name {
			if err := column.append(values[valueIndex].Value, true); err != nil {
				return err
			}
			valueIndex++
			continue
		}
		if err := column.append(nil, false); err != nil {
			return err
		}
	}
	d.length++
	return nil
}

// Name returns the dataset's format name.
func (d *Dataset) Name() string {
	if d == nil {
		return ""
	}
	return d.name
}

// MultiID returns the dataset's instance identifier.
func (d *Dataset) MultiID() uint8 {
	if d == nil {
		return 0
	}
	return d.multiID
}

// Format returns an independent copy of the dataset schema.
func (d *Dataset) Format() ulog.Format {
	if d == nil {
		return ulog.Format{}
	}
	return cloneFormat(d.format)
}

// Len returns the number of records in the dataset.
func (d *Dataset) Len() int {
	if d == nil {
		return 0
	}
	return d.length
}

// Columns returns the flattened scalar columns in wire order.
func (d *Dataset) Columns() []Column {
	if d == nil {
		return nil
	}
	return append([]Column(nil), d.columns...)
}

// Column returns a flattened scalar column by name.
func (d *Dataset) Column(name string) (Column, bool) {
	if d == nil {
		return Column{}, false
	}
	index, ok := d.columnIndex[name]
	if !ok {
		return Column{}, false
	}
	return d.columns[index], true
}

// Column is a nullable, primitive-typed dataset column. Values returns one of
// []int8, []uint8, []int16, []uint16, []int32, []uint32, []int64, []uint64,
// []float32, []float64, or []bool according to ulog.Type.
type Column struct {
	name   string
	typeID ulog.Type
	values any
	valid  []bool
}

func newColumn(name string, typeID ulog.Type) Column {
	column := Column{name: name, typeID: typeID}
	switch typeID {
	case ulog.TypeInt8:
		column.values = []int8(nil)
	case ulog.TypeUint8, ulog.TypeChar:
		column.values = []uint8(nil)
	case ulog.TypeInt16:
		column.values = []int16(nil)
	case ulog.TypeUint16:
		column.values = []uint16(nil)
	case ulog.TypeInt32:
		column.values = []int32(nil)
	case ulog.TypeUint32:
		column.values = []uint32(nil)
	case ulog.TypeInt64:
		column.values = []int64(nil)
	case ulog.TypeUint64:
		column.values = []uint64(nil)
	case ulog.TypeFloat32:
		column.values = []float32(nil)
	case ulog.TypeFloat64:
		column.values = []float64(nil)
	case ulog.TypeBool:
		column.values = []bool(nil)
	}
	return column
}

func (c *Column) append(value any, valid bool) error {
	c.valid = append(c.valid, valid)
	switch c.typeID {
	case ulog.TypeInt8:
		return appendColumnValue(c, value, valid, func(values []int8, value int8) any { return append(values, value) })
	case ulog.TypeUint8, ulog.TypeChar:
		return appendColumnValue(c, value, valid, func(values []uint8, value uint8) any { return append(values, value) })
	case ulog.TypeInt16:
		return appendColumnValue(c, value, valid, func(values []int16, value int16) any { return append(values, value) })
	case ulog.TypeUint16:
		return appendColumnValue(c, value, valid, func(values []uint16, value uint16) any { return append(values, value) })
	case ulog.TypeInt32:
		return appendColumnValue(c, value, valid, func(values []int32, value int32) any { return append(values, value) })
	case ulog.TypeUint32:
		return appendColumnValue(c, value, valid, func(values []uint32, value uint32) any { return append(values, value) })
	case ulog.TypeInt64:
		return appendColumnValue(c, value, valid, func(values []int64, value int64) any { return append(values, value) })
	case ulog.TypeUint64:
		return appendColumnValue(c, value, valid, func(values []uint64, value uint64) any { return append(values, value) })
	case ulog.TypeFloat32:
		return appendColumnValue(c, value, valid, func(values []float32, value float32) any { return append(values, value) })
	case ulog.TypeFloat64:
		return appendColumnValue(c, value, valid, func(values []float64, value float64) any { return append(values, value) })
	case ulog.TypeBool:
		return appendColumnValue(c, value, valid, func(values []bool, value bool) any { return append(values, value) })
	default:
		return fmt.Errorf("unsupported column type %q", c.typeID)
	}
}

func appendColumnValue[T any](column *Column, value any, valid bool, appendValue func([]T, T) any) error {
	values, ok := column.values.([]T)
	if !ok {
		return fmt.Errorf("column %q has invalid storage %T", column.name, column.values)
	}
	var typed T
	if valid {
		var valueOK bool
		typed, valueOK = value.(T)
		if !valueOK {
			return fmt.Errorf("column %q requires %T values, got %T", column.name, typed, value)
		}
	}
	column.values = appendValue(values, typed)
	return nil
}

// Name returns the flattened field path for the column.
func (c Column) Name() string { return c.name }

// Type returns the ULog primitive type stored by the column.
func (c Column) Type() ulog.Type { return c.typeID }

// Len returns the number of rows in the column.
func (c Column) Len() int { return len(c.valid) }

// Values returns an independent copy of the typed values. Null rows contain the
// primitive type's zero value; use Value to distinguish them.
func (c Column) Values() any {
	switch values := c.values.(type) {
	case []int8:
		return append([]int8(nil), values...)
	case []uint8:
		return append([]uint8(nil), values...)
	case []int16:
		return append([]int16(nil), values...)
	case []uint16:
		return append([]uint16(nil), values...)
	case []int32:
		return append([]int32(nil), values...)
	case []uint32:
		return append([]uint32(nil), values...)
	case []int64:
		return append([]int64(nil), values...)
	case []uint64:
		return append([]uint64(nil), values...)
	case []float32:
		return append([]float32(nil), values...)
	case []float64:
		return append([]float64(nil), values...)
	case []bool:
		return append([]bool(nil), values...)
	default:
		return nil
	}
}

// Value returns one value and whether it is valid. It returns nil, false for a
// null or out-of-range row.
func (c Column) Value(index int) (any, bool) {
	if index < 0 || index >= len(c.valid) || !c.valid[index] {
		return nil, false
	}
	switch values := c.values.(type) {
	case []int8:
		return values[index], true
	case []uint8:
		return values[index], true
	case []int16:
		return values[index], true
	case []uint16:
		return values[index], true
	case []int32:
		return values[index], true
	case []uint32:
		return values[index], true
	case []int64:
		return values[index], true
	case []uint64:
		return values[index], true
	case []float32:
		return values[index], true
	case []float64:
		return values[index], true
	case []bool:
		return values[index], true
	default:
		return nil, false
	}
}

func cloneFormat(format ulog.Format) ulog.Format {
	format.Fields = append([]ulog.Field(nil), format.Fields...)
	return format
}

func cloneKeyValues(values []ulog.KeyValue) []ulog.KeyValue {
	cloned := append([]ulog.KeyValue(nil), values...)
	for i := range cloned {
		cloned[i].Value = clonePrimitiveSlice(cloned[i].Value)
	}
	return cloned
}

func cloneDefaultParameters(values []ulog.DefaultParameter) []ulog.DefaultParameter {
	cloned := append([]ulog.DefaultParameter(nil), values...)
	for i := range cloned {
		cloned[i].Value = clonePrimitiveSlice(cloned[i].Value)
	}
	return cloned
}

func clonePrimitiveSlice(value any) any {
	switch value := value.(type) {
	case []int8:
		return append([]int8(nil), value...)
	case []uint8:
		return append([]uint8(nil), value...)
	case []int16:
		return append([]int16(nil), value...)
	case []uint16:
		return append([]uint16(nil), value...)
	case []int32:
		return append([]int32(nil), value...)
	case []uint32:
		return append([]uint32(nil), value...)
	case []int64:
		return append([]int64(nil), value...)
	case []uint64:
		return append([]uint64(nil), value...)
	case []float32:
		return append([]float32(nil), value...)
	case []float64:
		return append([]float64(nil), value...)
	case []bool:
		return append([]bool(nil), value...)
	default:
		return value
	}
}
