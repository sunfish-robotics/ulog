// Package ulog reads, writes, and analyses PX4 ULog files.
//
// [Reader] exposes bounded-memory dynamic records using the schemas defined by
// each file. [FormatFor], [Decode], and [Register] provide optional typed Go
// adapters over that dynamic model. [Read] builds eager column-oriented
// datasets for analysis; Apache Arrow and Parquet conversion lives in the
// separate pkg/columnar package.
package ulog
