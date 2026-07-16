package analyze_cases

import "io"

func seekWithSwappedArguments(seeker io.Seeker) (int64, error) {
	return seeker.Seek(io.SeekStart, 0)
}
