// Package buildidentity provides a stable identity for the exact Strider
// executable. It deliberately hashes the executable for development builds:
// a VCS revision plus a "modified" bit cannot distinguish two different dirty
// worktrees built from the same commit.
//
//strider:ignore-file no-package-var,single-case-switch
package buildidentity

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
)

var (
	identityOnce sync.Once
	identity     string
	revisionOnce sync.Once
	revision     string
)

// Identity returns a collision-resistant identity for the running executable.
func Identity() string {
	identityOnce.Do(
		func() {
			executable, err := os.Executable()
			if err == nil {
				file, openErr := os.Open(executable)
				if openErr == nil {
					hash := sha256.New()
					_, copyErr := io.Copy(hash, file)
					closeErr := file.Close()
					if copyErr == nil && closeErr == nil {
						identity = "sha256:" + hex.EncodeToString(hash.Sum(nil))
						return
					}
				}
			}
			fallback := Revision()
			if fallback == "" {
				fallback = "development-unknown"
			}
			identity = fallback
		},
	)
	return identity
}

// Revision returns the VCS revision and dirty state embedded by the Go
// toolchain, when available.
func Revision() string {
	revisionOnce.Do(
		func() {
			info, ok := debug.ReadBuildInfo()
			if !ok {
				return
			}
			modified := false
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					revision = setting.Value
				case "vcs.modified":
					modified = setting.Value == "true"
				}
			}
			if modified && revision != "" {
				revision += "-dirty"
			}
			revision = strings.TrimSpace(revision)
		},
	)
	return revision
}
