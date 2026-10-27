package store

import (
	"context"
	"testing"
	"time"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/passive"
	"github.com/Atrabilis/nport-acquisition/internal/storage"
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

type recordingWriter struct {
	rows []storage.ShadowRow
}

func (w *recordingWriter) Name() string { return "recording" }
func (w *recordingWriter) Close()       {}
func (w *recordingWriter) Write(_ context.Context, row storage.ShadowRow) error {
	w.rows = append(w.rows, row)
	return nil
}

func testCoordinator(w Writer) *Coordinator {
	cfg := config.Config{
		Site: "plant_a",
		NPorts: []config.NPortConfig{
			{Name: "port01", DetectedSlaves: []uint8{1, 2, 3}},
		},
	}
	return NewCoordinator(cfg, []Writer{w}, nil)
}

func capture(c *Coordinator, slaveID uint8) {
	c.RecordFrame(config.NPortConfig{Name: "port01"}, slaveID, "sensor", "kipp_zonen",
		[]passive.RegisterValue{{Name: "value_0", Type: "uint16", Value: float64(slaveID)}}, time.Now())
}

func TestFlushPendingWritesCapturedSlaves(t *testing.T) {
	w := &recordingWriter{}
	c := testCoordinator(w)
	capture(c, 1)
	capture(c, 2)
	if len(w.rows) != 0 {
		t.Fatalf("nothing should be written before every slave is seen, got %d rows", len(w.rows))
	}

	c.FlushPending()
	if len(w.rows) != 2 || w.rows[0].SlaveID != 1 || w.rows[1].SlaveID != 2 {
		t.Fatalf("expected slaves 1 and 2 written in order, got %+v", w.rows)
	}

	// A frame arriving after the flush, or a second flush, writes nothing more.
	capture(c, 3)
	c.FlushPending()
	if len(w.rows) != 2 {
		t.Fatalf("expected no writes after the flush, got %d rows", len(w.rows))
	}
}

func TestFlushPendingAfterCompleteCaptureDoesNotWriteTwice(t *testing.T) {
	w := &recordingWriter{}
	c := testCoordinator(w)
	capture(c, 1)
	capture(c, 2)
	capture(c, 3) // all seen: written here
	c.FlushPending()
	if len(w.rows) != 3 {
		t.Fatalf("expected 3 rows written once, got %d", len(w.rows))
	}
}

func TestFlushPendingWithoutCapturesWritesNothing(t *testing.T) {
	w := &recordingWriter{}
	testCoordinator(w).FlushPending()
	if len(w.rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(w.rows))
	}
	var nilCoordinator *Coordinator
	nilCoordinator.FlushPending() // read-only mode has no coordinator
}
