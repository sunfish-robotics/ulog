package ulog_test

import (
	"bytes"
	"fmt"
	"os"

	"github.com/sunfish-robotics/ulog"
)

func ExampleReader() {
	source, err := os.Open("flight.ulg")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			panic(err)
		}
	}()

	reader, err := ulog.NewReader(source)
	if err != nil {
		panic(err)
	}
	for reader.Next() {
		record := reader.Record()
		timestamp, err := record.Value("timestamp")
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s[%d] timestamp=%v\n", record.Name(), record.MultiID(), timestamp)
	}
	if err := reader.Err(); err != nil {
		panic(err)
	}
}

func ExampleRegister() {
	type sensorSample struct {
		Timestamp uint64
		Pressure  float32
		Valid     bool
	}

	var destination bytes.Buffer
	writer, err := ulog.NewWriter(&destination, ulog.WithStartTimestamp(1_000_000))
	if err != nil {
		panic(err)
	}
	stream, err := ulog.Register[sensorSample](writer)
	if err != nil {
		panic(err)
	}
	if err := stream.Write(sensorSample{Timestamp: 1_000_100, Pressure: 101.25, Valid: true}); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}

	file, err := ulog.Read(bytes.NewReader(destination.Bytes()))
	if err != nil {
		panic(err)
	}
	dataset, err := file.Dataset("sensor_sample", 0)
	if err != nil {
		panic(err)
	}
	pressure, ok := dataset.Column("pressure")
	if !ok {
		panic("pressure column is missing")
	}
	fmt.Println(pressure.Values())

	// Output:
	// [101.25]
}

func ExampleRead() {
	source, err := os.Open("flight.ulg")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			panic(err)
		}
	}()

	file, err := ulog.Read(source)
	if err != nil {
		panic(err)
	}
	dataset, err := file.Dataset("vehicle_attitude", 0)
	if err != nil {
		panic(err)
	}
	timestamps, ok := dataset.Column("timestamp")
	if !ok {
		panic("timestamp column is missing")
	}
	values := timestamps.Values().([]uint64)
	fmt.Printf("%d attitude samples, from %d µs\n", dataset.Len(), values[0])
}

func ExampleDecode() {
	type vehicleAttitude struct {
		Timestamp uint64
		Q         [4]float32
	}

	source, err := os.Open("flight.ulg")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			panic(err)
		}
	}()

	reader, err := ulog.NewReader(source)
	if err != nil {
		panic(err)
	}
	for reader.Next() {
		if reader.Record().Name() != "vehicle_attitude" {
			continue
		}
		attitude, err := ulog.Decode[vehicleAttitude](reader.Record())
		if err != nil {
			panic(err)
		}
		fmt.Printf("%d: %v\n", attitude.Timestamp, attitude.Q)
	}
	if err := reader.Err(); err != nil {
		panic(err)
	}
}
