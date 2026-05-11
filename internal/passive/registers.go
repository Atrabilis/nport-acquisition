package passive

import (
	"fmt"
	"strings"

	"github.com/Atrabilis/nport-acquisition/internal/config"
)

type RegisterValue struct {
	Register int
	Name     string
	Type     string
	Value    float64
}

func decodeKnownRegisters(nport config.NPortConfig, slaveID uint8, data []byte) (string, []string, []RegisterValue) {
	if len(data)%2 != 0 || len(nport.Slaves) == 0 {
		return "", nil, nil
	}

	var slave *config.SlaveConfig
	for i := range nport.Slaves {
		if nport.Slaves[i].Address == slaveID {
			slave = &nport.Slaves[i]
			break
		}
	}
	if slave == nil || len(slave.Registers) == 0 {
		return "", nil, nil
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

	return slave.Name, lines, values
}
