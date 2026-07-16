package analyze_cases

import "io"

func seekWithCorrectArguments(seeker io.Seeker) (int64, error) {
	return seeker.Seek(0, io.SeekStart)
}
