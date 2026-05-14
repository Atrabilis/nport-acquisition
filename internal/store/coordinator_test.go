package store

import (
	"testing"

	"github.com/Atrabilis/nport-acquisition/internal/passive"
)

func TestFieldsFromValuesNormalizesValueNames(t *testing.T) {
	fields := fieldsFromValues([]passive.RegisterValue{
		{Name: "value0", Type: "uint16", Value: 618},
		{Name: "value_1", Type: "uint16", Value: 101},
		{Name: "ir_device_voltage", Type: "float", Value: 12.4},
	})

	if got := fields["value_0"]; got != int64(618) {
		t.Fatalf("fields[value_0] = %#v, want int64(618)", got)
	}
	if got := fields["value_1"]; got != int64(101) {
		t.Fatalf("fields[value_1] = %#v, want int64(101)", got)
	}
	if got := fields["ir_device_voltage"]; got != 12.4 {
		t.Fatalf("fields[ir_device_voltage] = %#v, want 12.4", got)
	}
	if _, ok := fields["value0"]; ok {
		t.Fatal("fields contains legacy value0 key")
	}
}
