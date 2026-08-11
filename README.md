# ulog

[![CI][ci-badge]][ci]
[![Go Reference][reference-badge]][go-reference]
[![License: Apache-2.0][license-badge]][license]

A pure-Go reader, writer, and analysis library for [the PX4 ULog format][reference]. It reads file-defined schemas dynamically, with reflection and generics available as optional conveniences for Go types.

```console
go get github.com/sunfish-robotics/ulog
```

## Read a stream

`Reader` keeps bounded state and returns one dynamically typed data record at a time. ULog `F` format messages remain authoritative, so files do not need matching Go structs.

```go
source, err := os.Open("flight.ulg")
if err != nil {
    return err
}
defer source.Close()

reader, err := ulog.NewReader(source)
if err != nil {
    return err
}
for reader.Next() {
    record := reader.Record()
    timestamp, err := record.Value("timestamp")
    if err != nil {
        return err
    }
    fmt.Printf("%s[%d] timestamp=%v\n", record.Name(), record.MultiID(), timestamp)
}
if err := reader.Err(); err != nil {
    return err
}
```

Arrays and nested formats use flattened paths such as `q[0]` and `position.x`.

## Use typed Go values

`FormatFor[T]`, `Decode[T]`, and `Register[T]` adapt Go structs to the dynamic schema model. They do not replace the schema carried by a ULog file.

```go
type Sample struct {
    Timestamp uint64
    Pressure  float32
    Valid     bool
}

var output bytes.Buffer
writer, err := ulog.NewWriter(&output, ulog.WithStartTimestamp(1_000_000))
if err != nil {
    return err
}
stream, err := ulog.Register[Sample](writer)
if err != nil {
    return err
}
if err := stream.Write(Sample{Timestamp: 1_000_100, Pressure: 101.25, Valid: true}); err != nil {
    return err
}
if err := writer.Close(); err != nil {
    return err
}
```

The writer also accepts dynamic `Format` values and raw, format-validated payloads through `RegisterFormat`.

## Analyse datasets

`Read` eagerly groups records by format name and `multi_id`, preserving primitive widths, null trailing fields, information entries, parameters, logs, and dropouts.

```go
file, err := ulog.Read(source)
if err != nil {
    return err
}
dataset, err := file.Dataset("vehicle_attitude", 0)
if err != nil {
    return err
}
timestamps, ok := dataset.Column("timestamp")
if !ok {
    return errors.New("timestamp column is missing")
}
values := timestamps.Values().([]uint64)
```

For Arrow and Parquet, import the separate adapter package:

```go
record, err := columnar.ToArrow(dataset, nil)
if err != nil {
    return err
}
defer record.Release()

if err := columnar.WriteParquet(destination, dataset); err != nil {
    return err
}
```

The root `ulog` package uses only the Go standard library. Importing `columnar` adds Apache Arrow.

## Compatibility

Wire codecs have golden-byte tests. A committed file written by `pyulog` is read during ordinary Go tests, while a separate pinned CI job verifies both directions semantically:

```text
Go writer → pyulog reader
Go writer → pyulog rewrite → Go reader
```

The pinned environment and fixture provenance live under [`integration/pyulog`](integration/pyulog) and [`testdata/interoperability`](testdata/interoperability).

## Current scope

- ULog version 1 files
- dynamic primitive, fixed-array, and nested-format data
- streaming and eager reads
- typed and dynamic writing
- information, initial parameters, logging, and dropouts
- Arrow record batches and Parquet output

Appended data sections are rejected rather than silently misread. Multi-information, default-parameter, and tagged-log writing are available at the lower-level `pkg/wire` boundary but do not yet have root-package writer conveniences.

## License

This project is released under the [Apache License, Version 2.0](LICENSE).

[ci]: https://github.com/sunfish-robotics/ulog/actions/workflows/ci.yml
[ci-badge]: https://github.com/sunfish-robotics/ulog/actions/workflows/ci.yml/badge.svg
[go-reference]: https://pkg.go.dev/github.com/sunfish-robotics/ulog
[license]: LICENSE
[license-badge]: https://img.shields.io/badge/license-Apache--2.0-blue.svg
[reference]: https://docs.px4.io/main/en/dev_log/ulog_file_format
[reference-badge]: https://pkg.go.dev/badge/github.com/sunfish-robotics/ulog.svg
