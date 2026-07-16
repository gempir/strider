package analyze_cases

import (
	"encoding/binary"
	"io"
)

func writeUnsupportedBinaryValue(value int) {
	binary.Write(io.Discard, binary.LittleEndian, value)
}
