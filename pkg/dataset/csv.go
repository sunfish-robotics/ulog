package dataset

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// WriteCSV writes the dataset as one CSV table. Column names form the header in
// wire order, records remain in source order, and null values are empty fields.
// WriteCSV does not close destination.
func (d *Dataset) WriteCSV(destination io.Writer) error {
	if d == nil {
		return errors.New("nil ULog dataset")
	}
	if destination == nil {
		return errors.New("nil CSV destination")
	}

	writer := csv.NewWriter(destination)
	header := make([]string, len(d.columns))
	for i, column := range d.columns {
		header[i] = column.name
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	record := make([]string, len(d.columns))
	for row := 0; row < d.length; row++ {
		for i, column := range d.columns {
			value, valid := column.Value(row)
			if !valid {
				record[i] = ""
				continue
			}
			formatted, err := formatCSVValue(value)
			if err != nil {
				return fmt.Errorf("format CSV column %q row %d: %w", column.name, row, err)
			}
			record[i] = formatted
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write CSV row %d: %w", row, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("write CSV: %w", err)
	}
	return nil
}

func formatCSVValue(value any) (string, error) {
	switch value := value.(type) {
	case int8:
		return strconv.FormatInt(int64(value), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(value), 10), nil
	case int16:
		return strconv.FormatInt(int64(value), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(value), 10), nil
	case int32:
		return strconv.FormatInt(int64(value), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(value), 10), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case uint64:
		return strconv.FormatUint(value, 10), nil
	case float32:
		return strconv.FormatFloat(float64(value), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(value, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(value), nil
	case string:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported value type %T", value)
	}
}
