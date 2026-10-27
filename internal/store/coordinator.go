package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/passive"
	"github.com/Atrabilis/nport-acquisition/internal/storage"
)

type Writer interface {
	Name() string
	Write(ctx context.Context, row storage.ShadowRow) error
	Close()
}

type filteredWriter interface {
	Accepts(row storage.ShadowRow) bool
}

type Coordinator struct {
	plant    string
	expected map[string]map[uint8]struct{}
	last     map[string]map[uint8]storedFrame
	writers  []Writer
	cancel   context.CancelFunc
	mu       sync.Mutex
	done     bool
}

type storedFrame struct {
	nport      config.NPortConfig
	slaveID    uint8
	slaveName  string
	deviceType string
	values     []passive.RegisterValue
	ts         time.Time
}

func NewCoordinator(cfg config.Config, writers []Writer, cancel context.CancelFunc) *Coordinator {
	if len(writers) == 0 {
		return nil
	}
	expected := make(map[string]map[uint8]struct{})
	for _, nport := range cfg.NPorts {
		if len(nport.DetectedSlaves) == 0 {
			continue
		}
		set := make(map[uint8]struct{}, len(nport.DetectedSlaves))
		for _, slaveID := range nport.DetectedSlaves {
			set[slaveID] = struct{}{}
		}
		expected[nport.Name] = set
	}
	if len(expected) == 0 {
		return nil
	}

	return &Coordinator{
		plant:    strings.TrimSpace(cfg.Site),
		expected: expected,
		last:     make(map[string]map[uint8]storedFrame),
		writers:  writers,
		cancel:   cancel,
	}
}

func (c *Coordinator) Close() {
	if c == nil {
		return
	}
	for _, writer := range c.writers {
		writer.Close()
	}
}

func (c *Coordinator) RecordFrame(nport config.NPortConfig, slaveID uint8, slaveName string, deviceType string, values []passive.RegisterValue, ts time.Time) {
	if c == nil || len(values) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return
	}

	expectedSlaves := c.expected[nport.Name]
	if len(expectedSlaves) == 0 {
		return
	}
	if _, wanted := expectedSlaves[slaveID]; !wanted {
		return
	}
	if c.last[nport.Name] == nil {
		c.last[nport.Name] = make(map[uint8]storedFrame)
	}
	c.last[nport.Name][slaveID] = storedFrame{
		nport:      nport,
		slaveID:    slaveID,
		slaveName:  slaveName,
		deviceType: deviceType,
		values:     values,
		ts:         ts,
	}
	fmt.Printf("[store] captured port=%s slave=%d slave_name=%s values=%d\n", nport.Name, slaveID, slaveName, len(values))

	if !c.allSeenLocked() {
		return
	}
	c.flushLocked()
	c.done = true
	if c.cancel != nil {
		c.cancel()
	}
}

// FlushPending writes the slaves captured so far when the run ends before
// every expected slave was seen (store_timeout_seconds or shutdown), and logs
// the missing ones. Without it, one disconnected sensor lost the data of
// every other sensor on the same port on every run.
func (c *Coordinator) FlushPending() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return
	}
	c.done = true

	missing := strings.Join(c.missingLocked(), ", ")
	captured := 0
	for _, slaves := range c.last {
		captured += len(slaves)
	}
	if captured == 0 {
		fmt.Printf("[store] WARNING: run ended without capturing any expected slave; missing: %s\n", missing)
		return
	}
	fmt.Printf("[store] WARNING: run ended before every expected slave was captured; writing %d captured, missing: %s\n", captured, missing)
	c.flushLocked()
}

// missingLocked lists expected slaves not captured, as "port/slave_id".
func (c *Coordinator) missingLocked() []string {
	var missing []string
	for port, expectedSlaves := range c.expected {
		for slaveID := range expectedSlaves {
			if _, ok := c.last[port][slaveID]; !ok {
				missing = append(missing, fmt.Sprintf("%s/%d", port, slaveID))
			}
		}
	}
	sort.Strings(missing)
	return missing
}

func (c *Coordinator) allSeenLocked() bool {
	for port, expectedSlaves := range c.expected {
		seen := c.last[port]
		for slaveID := range expectedSlaves {
			if _, ok := seen[slaveID]; !ok {
				return false
			}
		}
	}
	return true
}

func (c *Coordinator) flushLocked() {
	frames := make([]storedFrame, 0)
	for _, slaves := range c.last {
		for _, frame := range slaves {
			frames = append(frames, frame)
		}
	}
	sort.Slice(frames, func(i, j int) bool {
		if frames[i].nport.Name == frames[j].nport.Name {
			return frames[i].slaveID < frames[j].slaveID
		}
		return frames[i].nport.Name < frames[j].nport.Name
	})

	for _, frame := range frames {
		fields := fieldsFromValues(frame.values)
		for _, writer := range c.writers {
			row := storage.ShadowRow{
				Plant:      c.plant,
				TS:         frame.ts,
				DeviceName: frame.nport.Name,
				SlaveName:  frame.slaveName,
				SlaveID:    frame.slaveID,
				Fields:     fields,
				Tags: map[string]string{
					"device_type": frame.deviceType,
				},
			}
			if filter, ok := writer.(filteredWriter); ok && !filter.Accepts(row) {
				continue
			}
			if err := writer.Write(context.Background(), row); err != nil {
				fmt.Printf("[store] writer=%s port=%s slave=%d failed: %v\n", writer.Name(), frame.nport.Name, frame.slaveID, err)
				continue
			}
			fmt.Printf("[store] writer=%s port=%s slave=%d wrote fields=%d\n", writer.Name(), frame.nport.Name, frame.slaveID, len(fields))
		}
	}
}

func fieldsFromValues(values []passive.RegisterValue) map[string]interface{} {
	fields := make(map[string]interface{}, len(values))
	for _, value := range values {
		name := normalizeFieldName(value.Name)
		if name == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(value.Type)) {
		case "float", "float32", "float64":
			fields[name] = value.Value
		default:
			fields[name] = int64(value.Value)
		}
	}
	return fields
}

func normalizeFieldName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "value") && len(trimmed) > len("value") {
		suffix := trimmed[len("value"):]
		for _, ch := range suffix {
			if ch < '0' || ch > '9' {
				return trimmed
			}
		}
		return "value_" + suffix
	}
	return trimmed
}
