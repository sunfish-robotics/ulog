// Package columnar converts ULog datasets to Apache Arrow and Parquet.
package columnar

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	arrowlib "github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"

	"github.com/sunfish-robotics/ulog"
)

// ToArrow converts dataset into one Arrow record batch. The caller must release
// the returned record. A nil allocator uses [memory.DefaultAllocator].
func ToArrow(dataset *ulog.Dataset, allocator memory.Allocator) (arrowlib.RecordBatch, error) {
	if dataset == nil {
		return nil, errors.New("nil ULog dataset")
	}
	if allocator == nil {
		allocator = memory.DefaultAllocator
	}

	columns := dataset.Columns()
	fields := make([]arrowlib.Field, len(columns))
	for i, column := range columns {
		dataType, err := arrowType(column.Type())
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", column.Name(), err)
		}
		fields[i] = arrowlib.Field{Name: column.Name(), Type: dataType, Nullable: true}
	}
	metadata := arrowlib.MetadataFrom(map[string]string{
		"ulog.format":            dataset.Name(),
		"ulog.format_definition": dataset.Format().String(),
		"ulog.multi_id":          strconv.FormatUint(uint64(dataset.MultiID()), 10),
	})
	schema := arrowlib.NewSchema(fields, &metadata)
	builder := array.NewRecordBuilder(allocator, schema)
	defer builder.Release()
	for i, column := range columns {
		valid := make([]bool, column.Len())
		for row := range valid {
			_, valid[row] = column.Value(row)
		}
		if err := appendColumn(builder.Field(i), column, valid); err != nil {
			return nil, err
		}
	}
	return builder.NewRecordBatch(), nil
}

// WriteParquet converts dataset to Arrow and writes a Parquet file to
// destination. It does not close destination.
func WriteParquet(destination io.Writer, dataset *ulog.Dataset) error {
	if destination == nil {
		return errors.New("nil Parquet destination")
	}
	record, err := ToArrow(dataset, memory.DefaultAllocator)
	if err != nil {
		return err
	}
	defer record.Release()

	table := array.NewTableFromRecords(record.Schema(), []arrowlib.RecordBatch{record})
	defer table.Release()
	chunkSize := int64(dataset.Len())
	if chunkSize < 1 {
		chunkSize = 1
	}
	if err := pqarrow.WriteTable(
		table,
		destination,
		chunkSize,
		parquet.NewWriterProperties(),
		pqarrow.DefaultWriterProps(),
	); err != nil {
		return fmt.Errorf("write Parquet table: %w", err)
	}
	return nil
}

func arrowType(typeID ulog.Type) (arrowlib.DataType, error) {
	switch typeID {
	case ulog.TypeInt8:
		return arrowlib.PrimitiveTypes.Int8, nil
	case ulog.TypeUint8, ulog.TypeChar:
		return arrowlib.PrimitiveTypes.Uint8, nil
	case ulog.TypeInt16:
		return arrowlib.PrimitiveTypes.Int16, nil
	case ulog.TypeUint16:
		return arrowlib.PrimitiveTypes.Uint16, nil
	case ulog.TypeInt32:
		return arrowlib.PrimitiveTypes.Int32, nil
	case ulog.TypeUint32:
		return arrowlib.PrimitiveTypes.Uint32, nil
	case ulog.TypeInt64:
		return arrowlib.PrimitiveTypes.Int64, nil
	case ulog.TypeUint64:
		return arrowlib.PrimitiveTypes.Uint64, nil
	case ulog.TypeFloat32:
		return arrowlib.PrimitiveTypes.Float32, nil
	case ulog.TypeFloat64:
		return arrowlib.PrimitiveTypes.Float64, nil
	case ulog.TypeBool:
		return arrowlib.FixedWidthTypes.Boolean, nil
	default:
		return nil, fmt.Errorf("unsupported ULog type %q", typeID)
	}
}

func appendColumn(builder array.Builder, column ulog.Column, valid []bool) error {
	switch column.Type() {
	case ulog.TypeInt8:
		builder, ok := builder.(*array.Int8Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]int8), valid)
	case ulog.TypeUint8, ulog.TypeChar:
		builder, ok := builder.(*array.Uint8Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]uint8), valid)
	case ulog.TypeInt16:
		builder, ok := builder.(*array.Int16Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]int16), valid)
	case ulog.TypeUint16:
		builder, ok := builder.(*array.Uint16Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]uint16), valid)
	case ulog.TypeInt32:
		builder, ok := builder.(*array.Int32Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]int32), valid)
	case ulog.TypeUint32:
		builder, ok := builder.(*array.Uint32Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]uint32), valid)
	case ulog.TypeInt64:
		builder, ok := builder.(*array.Int64Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]int64), valid)
	case ulog.TypeUint64:
		builder, ok := builder.(*array.Uint64Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]uint64), valid)
	case ulog.TypeFloat32:
		builder, ok := builder.(*array.Float32Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]float32), valid)
	case ulog.TypeFloat64:
		builder, ok := builder.(*array.Float64Builder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]float64), valid)
	case ulog.TypeBool:
		builder, ok := builder.(*array.BooleanBuilder)
		if !ok {
			return builderTypeError(column, builder)
		}
		builder.AppendValues(column.Values().([]bool), valid)
	default:
		return fmt.Errorf("column %q has unsupported ULog type %q", column.Name(), column.Type())
	}
	return nil
}

func builderTypeError(column ulog.Column, builder array.Builder) error {
	return fmt.Errorf("column %q has unexpected Arrow builder %T", column.Name(), builder)
}
