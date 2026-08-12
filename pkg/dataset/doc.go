// Package dataset builds eager, column-oriented views of ULog streams.
//
// Use [Read] when an entire log fits in memory and analysis needs typed columns.
// Use ulog.Reader directly for bounded-memory streaming access.
package dataset
