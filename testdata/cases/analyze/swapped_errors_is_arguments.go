package analyze_cases

import (
	"errors"
	"io"
)

func swappedErrorsIsArguments(err error) bool {
	return errors.Is(io.EOF, err)
}
