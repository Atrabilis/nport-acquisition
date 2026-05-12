package passive

import (
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/modbusrtu"
)

const (
	lufftWSXExpectedFrames = 53
	lufftWSXMinFrames      = 25
)

var lufftWSXAnchorValues = map[uint16]struct{}{
	4160: {},
	4166: {},
}

type lufftWSXCycle struct {
	frames [][]byte
}

func (d *FrameDecoder) decodeLufftWSXFrame(summary modbusrtu.Summary, frame []byte, slave config.SlaveConfig) ([]string, []RegisterValue, []string) {
	if len(frame) != 7 {
		return nil, nil, nil
	}
	if summary.CRCValid == nil || !*summary.CRCValid {
		return nil, nil, nil
	}
	if summary.ByteCount == nil || *summary.ByteCount != 2 {
		return nil, nil, nil
	}

	value := binary.BigEndian.Uint16(frame[3:5])
	cycle := d.lufftWSX[summary.SlaveID]
	if cycle == nil {
		cycle = &lufftWSXCycle{frames: make([][]byte, 0, lufftWSXExpectedFrames)}
		d.lufftWSX[summary.SlaveID] = cycle
	}

	var lines []string
	var values []RegisterValue
	if isLufftWSXAnchor(value) {
		if len(cycle.frames) > 0 {
			var err error
			values, err = decodeLufftWSXCycle(cycle.frames)
			if err != nil {
				lines = append(lines, fmt.Sprintf("  lufft_wsx: %v", err))
			} else {
				lines = append(lines, fmt.Sprintf("  lufft_wsx cycle slave=%s frames=%d", slave.Name, len(cycle.frames)))
				lines = append(lines, lufftWSXRegisterLines(values)...)
			}
		}
		cycle.frames = cycle.frames[:0]
		cycle.frames = append(cycle.frames, copyFrame(frame))
		return nil, values, lines
	}

	if len(cycle.frames) == 0 {
		return nil, nil, nil
	}
	cycle.frames = append(cycle.frames, copyFrame(frame))
	if len(cycle.frames) == lufftWSXExpectedFrames {
		values, err := decodeLufftWSXCycle(cycle.frames)
		if err != nil {
			cycle.frames = cycle.frames[:0]
			return nil, nil, []string{fmt.Sprintf("  lufft_wsx: %v", err)}
		}
		lines := []string{fmt.Sprintf("  lufft_wsx cycle slave=%s frames=%d", slave.Name, len(cycle.frames))}
		lines = append(lines, lufftWSXRegisterLines(values)...)
		cycle.frames = cycle.frames[:0]
		return nil, values, lines
	}
	return nil, nil, nil
}

func decodeLufftWSXCycle(frames [][]byte) ([]RegisterValue, error) {
	if len(frames) < lufftWSXMinFrames {
		return nil, fmt.Errorf("incomplete cycle: expected %d frames, got %d", lufftWSXExpectedFrames, len(frames))
	}
	if len(frames) != lufftWSXExpectedFrames {
		return nil, fmt.Errorf("invalid cycle length: expected %d frames, got %d", lufftWSXExpectedFrames, len(frames))
	}

	values := make([]RegisterValue, 0, len(frames))
	for idx, frame := range frames {
		if len(frame) < 5 {
			return nil, fmt.Errorf("invalid frame at index %d", idx)
		}
		raw := binary.BigEndian.Uint16(frame[3:5])
		values = append(values, RegisterValue{
			Register: idx,
			Name:     "value_" + strconv.Itoa(idx),
			Type:     "uint16",
			Value:    float64(raw),
		})
	}
	return values, nil
}

func isLufftWSXAnchor(value uint16) bool {
	_, ok := lufftWSXAnchorValues[value]
	return ok
}

func lufftWSXRegisterLines(values []RegisterValue) []string {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, fmt.Sprintf("reg=%d name=%s uint16=%d", value.Register, value.Name, int64(value.Value)))
	}
	return lines
}
