package analyze_cases

import (
	"errors"
	"io"
)

func errorsIsArguments(err error) bool {
	return errors.Is(err, io.EOF)
}
