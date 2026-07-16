package analyze_cases

import (
	"io"
	"os"
)

func allocatingByteWrite(bytes []byte) {
	io.WriteString(os.Stdout, string(bytes))
}
