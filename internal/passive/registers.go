package passive

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Atrabilis/nport-acquisition/internal/config"
	"github.com/Atrabilis/nport-acquisition/internal/modbusrtu"
)

type RegisterValue struct {
	Register int
	Name     string
	Type     string
	Value    float64
}

type FrameDecoder struct {
	nport    config.NPortConfig
	dustIQ   map[uint8]*dustIQCycle
	lufftWSX map[uint8]*lufftWSXCycle
}

func NewFrameDecoder(nport config.NPortConfig) *FrameDecoder {
	return &FrameDecoder{
		nport:    nport,
		dustIQ:   make(map[uint8]*dustIQCycle),
		lufftWSX: make(map[uint8]*lufftWSXCycle),
	}
}

func (d *FrameDecoder) DecodeFrame(summary modbusrtu.Summary, frame []byte, data []byte) (string, string, []string, []RegisterValue, []string) {
	if d == nil {
		return "", "", nil, nil, nil
	}
	slave := slaveConfigForID(d.nport, summary.SlaveID)
	if slave == nil {
		return "", "", nil, nil, nil
	}

	deviceType := effectiveDeviceType(d.nport, *slave)
	switch normalizeDeviceType(deviceType) {
	case "dustiq":
		registerLines, values, decoderLines := d.decodeDustIQFrame(summary, frame, *slave)
		return slave.Name, deviceType, registerLines, values, decoderLines
	case "lufft_wsx":
		registerLines, values, decoderLines := d.decodeLufftWSXFrame(summary, frame, *slave)
		return slave.Name, deviceType, registerLines, values, decoderLines
	default:
		slaveName, effectiveType, registerLines, values := decodeKnownRegisters(d.nport, summary.SlaveID, data)
		return slaveName, effectiveType, registerLines, values, nil
	}
}

func decodeKnownRegisters(nport config.NPortConfig, slaveID uint8, data []byte) (string, string, []string, []RegisterValue) {
	if len(data)%2 != 0 || len(nport.Slaves) == 0 {
		return "", "", nil, nil
	}

	slave := slaveConfigForID(nport, slaveID)
	if slave == nil || len(slave.Registers) == 0 {
		if slave == nil {
			return "", "", nil, nil
		}
		slave.Registers = defaultRegistersForDevice(effectiveDeviceType(nport, *slave))
	}
	if len(slave.Registers) == 0 {
		return "", effectiveDeviceType(nport, *slave), nil, nil
	}

	lines := make([]string, 0, len(slave.Registers))
	values := make([]RegisterValue, 0, len(slave.Registers))
	for _, reg := range slave.Registers {
		if reg.Register < 0 {
			continue
		}
		count := reg.RegisterCount
		if count == 0 {
			count = 1
		}
		if count != 1 {
			continue
		}

		byteIdx := reg.Register * 2
		if byteIdx+1 >= len(data) {
			continue
		}

		raw := data[byteIdx : byteIdx+2]
		valueU16 := uint16(raw[0])<<8 | uint16(raw[1])
		typ := strings.ToLower(strings.TrimSpace(reg.RegisterType))
		if typ == "" {
			typ = "int16"
		}

		switch typ {
		case "uint16":
			value := float64(valueU16)
			lines = append(lines, fmt.Sprintf("reg=%d name=%s uint16=%d", reg.Register, reg.RegisterName, int64(value)))
			values = append(values, RegisterValue{Register: reg.Register, Name: reg.RegisterName, Type: "uint16", Value: value})
		default:
			value := float64(int16(valueU16))
			lines = append(lines, fmt.Sprintf("reg=%d name=%s int16=%d", reg.Register, reg.RegisterName, int64(value)))
			values = append(values, RegisterValue{Register: reg.Register, Name: reg.RegisterName, Type: "int16", Value: value})
		}
	}

	return slave.Name, effectiveDeviceType(nport, *slave), lines, values
}

func slaveConfigForID(nport config.NPortConfig, slaveID uint8) *config.SlaveConfig {
	for i := range nport.Slaves {
		if nport.Slaves[i].Address == slaveID {
			return &nport.Slaves[i]
		}
	}
	return nil
}

func effectiveDeviceType(nport config.NPortConfig, slave config.SlaveConfig) string {
	if strings.TrimSpace(slave.DeviceType) != "" {
		return slave.DeviceType
	}
	return nport.DeviceType
}

func defaultRegistersForDevice(deviceType string) []config.RegisterConfig {
	switch normalizeDeviceType(deviceType) {
	case "kipp_zonen":
		return sequentialUint16Registers(43)
	default:
		return nil
	}
}

func normalizeDeviceType(deviceType string) string {
	normalized := strings.ToLower(strings.TrimSpace(deviceType))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "kipp_zonnen", "kippzonen", "kipp_zonen":
		return "kipp_zonen"
	case "dust_iq", "dust-iq", "dustiq":
		return "dustiq"
	case "lufftwsx", "lufft_wsx", "lufft-wsx", "lufft":
		return "lufft_wsx"
	default:
		return normalized
	}
}

func sequentialUint16Registers(count int) []config.RegisterConfig {
	registers := make([]config.RegisterConfig, 0, count)
	for i := 0; i < count; i++ {
		registers = append(registers, config.RegisterConfig{
			Register:     i,
			RegisterName: "value" + strconv.Itoa(i),
			RegisterType: "uint16",
		})
	}
	return registers
}
