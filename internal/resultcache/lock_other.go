//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package resultcache

import "os"

func lockFileExclusive(_ *os.File) error {
	return nil
}

func unlockFile(_ *os.File) error {
	return nil
}
