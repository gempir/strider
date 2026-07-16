package analyze_cases

import "os"

func directByteWrite(bytes []byte) {
	os.Stdout.Write(bytes)
}
