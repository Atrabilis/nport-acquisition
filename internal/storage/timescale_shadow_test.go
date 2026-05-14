package storage

import "testing"

func TestTimescaleShadowWriterAcceptsFilters(t *testing.T) {
	writer := &TimescaleShadowWriter{
		deviceTypes:  normalizedStringSet([]string{"kipp_zonen"}),
		slaveIDs:     uint8Set([]uint8{2, 3}),
		hasAnyFilter: true,
	}

	tests := []struct {
		name string
		row  ShadowRow
		want bool
	}{
		{
			name: "matching device type and slave id",
			row: ShadowRow{
				SlaveID: 2,
				Tags: map[string]string{
					"device_type": "kipp-zonen",
				},
			},
			want: true,
		},
		{
			name: "wrong device type",
			row: ShadowRow{
				SlaveID: 2,
				Tags: map[string]string{
					"device_type": "dustiq",
				},
			},
			want: false,
		},
		{
			name: "wrong slave id",
			row: ShadowRow{
				SlaveID: 4,
				Tags: map[string]string{
					"device_type": "kipp_zonen",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := writer.Accepts(tt.row); got != tt.want {
				t.Fatalf("Accepts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimescaleShadowWriterAcceptsWithoutFilters(t *testing.T) {
	writer := &TimescaleShadowWriter{}
	if !writer.Accepts(ShadowRow{}) {
		t.Fatal("Accepts() = false, want true for unfiltered writer")
	}
}
