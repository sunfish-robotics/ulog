// Package dataset loads ULog streams into eager, column-oriented views.
//
// Use [Read] when an entire log fits in memory and analysis needs repeated or
// column-wise access. Use [ulog.Reader] when data records can be processed
// without materialising the complete log. Numeric arrays and nested formats are
// flattened into stable column paths; character arrays remain string columns.
package dataset
