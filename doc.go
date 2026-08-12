// Package ulog reads and writes PX4 ULog streams.
//
// [Reader] exposes bounded-memory dynamic records using the schemas defined by
// each file. [FormatFor], [Decode], and [Register] provide optional typed Go
// adapters over that dynamic model. Eager analysis lives in pkg/dataset; Apache
// Arrow and Parquet conversion lives in pkg/columnar.
package ulog
