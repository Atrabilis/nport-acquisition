package passive

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/modbusrtu"
)

const dustIQExpectedFrames = 23

type dustIQCycle struct {
	frames [][]byte
}

type dustIQSpec struct {
	name      string
	converter func([]byte) (float64, bool)
	valueType string
}

var dustIQSpecs = []dustIQSpec{
	{name: "ir_device_type", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_datamodel_version", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_software_version", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_batch_number", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_serial_number", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_hardware_version", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_soiling_ratio_sensor1", converter: dustIQUint16Div10, valueType: "float"},
	{name: "ir_tr_loss_sensor1", converter: dustIQInt16Div10, valueType: "float"},
	{name: "ir_soiling_ratio_sensor2", converter: dustIQUint16Div10, valueType: "float"},
	{name: "ir_tr_loss_sensor2", converter: dustIQInt16Div10, valueType: "float"},
	{name: "", converter: nil, valueType: ""},
	{name: "ir_backpanel_temp", converter: dustIQBackpanelTemp, valueType: "float"},
	{name: "ir_calibration_year", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_calibration_month", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_calibration_day", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_tilt_x_direction", converter: dustIQInt16Div10, valueType: "float"},
	{name: "ir_tilt_y_direction", converter: dustIQInt16Div10, valueType: "float"},
	{name: "ir_calibration_flags", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_device_voltage", converter: dustIQDeviceVoltage, valueType: "float"},
	{name: "ir_operational_mode", converter: dustIQInt16, valueType: "int16"},
	{name: "ir_dust_tilt_sensor_1", converter: dustIQUint16, valueType: "uint16"},
	{name: "ir_dust_tilt_sensor_2", converter: dustIQUint16, valueType: "uint16"},
	{name: "placeholder_22", converter: dustIQUint16, valueType: "uint16"},
}

func (d *FrameDecoder) decodeDustIQFrame(summary modbusrtu.Summary, frame []byte, slave config.SlaveConfig) ([]string, []RegisterValue, []string) {
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
	cycle := d.dustIQ[summary.SlaveID]
	if cycle == nil {
		cycle = &dustIQCycle{frames: make([][]byte, 0, dustIQExpectedFrames)}
		d.dustIQ[summary.SlaveID] = cycle
	}

	var lines []string
	var values []RegisterValue
	var warnings []string
	if value == 800 {
		if len(cycle.frames) > 0 {
			var err error
			values, warnings, err = decodeDustIQCycle(cycle.frames)
			if err != nil {
				lines = append(lines, fmt.Sprintf("  dustiq: %v", err))
			} else {
				lines = append(lines, fmt.Sprintf("  dustiq cycle slave=%s frames=%d", slave.Name, len(cycle.frames)))
				lines = append(lines, dustIQRegisterLines(values)...)
				for _, warning := range warnings {
					lines = append(lines, "  dustiq warning: "+warning)
				}
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
	return nil, nil, nil
}

func decodeDustIQCycle(frames [][]byte) ([]RegisterValue, []string, error) {
	if len(frames) < dustIQExpectedFrames {
		return nil, nil, fmt.Errorf("incomplete cycle: expected %d frames, got %d", dustIQExpectedFrames, len(frames))
	}

	values := make([]RegisterValue, 0, len(dustIQSpecs))
	var warnings []string
	for idx, spec := range dustIQSpecs {
		if spec.name == "" || spec.converter == nil {
			continue
		}
		if idx >= len(frames) {
			warnings = append(warnings, fmt.Sprintf("reg %d (%s) missing frame", idx, spec.name))
			continue
		}
		val, ok := spec.converter(frames[idx])
		if !ok {
			warnings = append(warnings, fmt.Sprintf("reg %d (%s) invalid frame", idx, spec.name))
			continue
		}
		values = append(values, RegisterValue{
			Register: idx,
			Name:     spec.name,
			Type:     spec.valueType,
			Value:    val,
		})
	}
	return values, warnings, nil
}

func dustIQRegisterLines(values []RegisterValue) []string {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, fmt.Sprintf("reg=%d name=%s %s=%s", value.Register, value.Name, value.Type, formatDustIQValue(value.Value, value.Type)))
	}
	return lines
}

func dustIQUint16(frame []byte) (float64, bool) {
	if len(frame) < 5 {
		return 0, false
	}
	return float64(binary.BigEndian.Uint16(frame[3:5])), true
}

func dustIQUint16Div10(frame []byte) (float64, bool) {
	val, ok := dustIQUint16(frame)
	if !ok {
		return 0, false
	}
	return val / 10.0, true
}

func dustIQInt16(frame []byte) (float64, bool) {
	if len(frame) < 5 {
		return 0, false
	}
	return float64(int16(binary.BigEndian.Uint16(frame[3:5]))), true
}

func dustIQInt16Div10(frame []byte) (float64, bool) {
	val, ok := dustIQInt16(frame)
	if !ok {
		return 0, false
	}
	return val / 10.0, true
}

func dustIQBackpanelTemp(frame []byte) (float64, bool) {
	val, ok := dustIQUint16(frame)
	if !ok {
		return 0, false
	}
	return val/10.0 - 273.15, true
}

func dustIQDeviceVoltage(frame []byte) (float64, bool) {
	val, ok := dustIQInt16(frame)
	if !ok {
		return 0, false
	}
	return val / 1000.0, true
}

func formatDustIQValue(v float64, typ string) string {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "float", "float32", "float64":
		return trimFloat(v)
	default:
		return fmt.Sprintf("%d", int64(v))
	}
}

func trimFloat(v float64) string {
	s := fmt.Sprintf("%.4f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

func copyFrame(frame []byte) []byte {
	dup := make([]byte, len(frame))
	copy(dup, frame)
	return dup
}
