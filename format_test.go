package ulog

import (
	"reflect"
	"testing"
)

func TestParseFormat(t *testing.T) {
	format, err := ParseFormat("vehicle_attitude:uint64_t timestamp;float[4] q;bool valid;")
	if err != nil {
		t.Fatalf("ParseFormat() error = %v", err)
	}

	if format.Name != "vehicle_attitude" {
		t.Errorf("Name = %q, want %q", format.Name, "vehicle_attitude")
	}

	wantFields := []Field{
		{Name: "timestamp", Type: TypeUint64},
		{Name: "q", Type: TypeFloat32, ArrayLength: 4},
		{Name: "valid", Type: TypeBool},
	}
	if !reflect.DeepEqual(format.Fields, wantFields) {
		t.Errorf("Fields = %#v, want %#v", format.Fields, wantFields)
	}

	if got, want := format.String(), "vehicle_attitude:uint64_t timestamp;float[4] q;bool valid;"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestParseFormatRejectsMalformedDefinitions(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{name: "missing separator", format: "vehicle_attitude"},
		{name: "empty name", format: ":uint64_t timestamp;"},
		{name: "empty fields", format: "vehicle_attitude:"},
		{name: "missing field name", format: "vehicle_attitude:uint64_t;"},
		{name: "zero array", format: "vehicle_attitude:float[0] q;"},
		{name: "array exceeds data message", format: "vehicle_attitude:uint8_t[65534] values;"},
		{name: "wide array exceeds data message", format: "vehicle_attitude:uint64_t[8192] values;"},
		{name: "invalid message name", format: "vehicle attitude:uint64_t timestamp;"},
		{name: "invalid field name", format: "vehicle_attitude:uint64_t time-stamp;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseFormat(tt.format); err == nil {
				t.Fatalf("ParseFormat(%q) succeeded, want error", tt.format)
			}
		})
	}
}

func TestParseFormatAcceptsNestedTypes(t *testing.T) {
	format, err := ParseFormat("outer:uint64_t timestamp;inner[2] samples;")
	if err != nil {
		t.Fatalf("ParseFormat() error = %v", err)
	}

	want := Field{Name: "samples", Type: "inner", ArrayLength: 2}
	if got := format.Fields[1]; got != want {
		t.Errorf("nested field = %#v, want %#v", got, want)
	}
}
