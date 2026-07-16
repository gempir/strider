package analyze_cases

import (
	"encoding/binary"
	"io"
)

func writeSupportedBinaryValue(value uint32) {
	binary.Write(io.Discard, binary.LittleEndian, value)
}
