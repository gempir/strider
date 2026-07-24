//strider:ignore-file cognitive-complexity,cyclomatic-complexity,function-length
package app

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/gempir/strider/internal/filewrite"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/resultcache"
	"github.com/gempir/strider/internal/source"
	"github.com/gempir/strider/internal/telemetry"
	"github.com/gempir/strider/internal/workspace"
)

type formattedFile struct {
	filename string
	original []byte
	result   formatter.Result
}

type formattedStatus struct {
	filename string
	changed  bool
	ignored  bool
}

func formatFiles(ctx context.Context, files []*workspace.File, options formatter.Options) ([]formattedFile, []error) {
	finish := telemetry.Start("format.file-local")
	defer finish()
	restoreGC := workspace.BeginCSTCollectionWindow()
	defer restoreGC()
	formatted := make([]formattedFile, len(files))
	errorsByFile := make([]error, len(files))
	if len(files) == 0 {
		return formatted, errorsByFile
	}

	session := formatter.NewFormatter()
	admission := workspace.NewCSTAdmission(0)
	workerContext, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int)
	workers := min(runtime.GOMAXPROCS(0), len(files))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for {
				var index int
				var ok bool
				select {
				case <-workerContext.Done():
					return
				case index, ok = <-jobs:
					if !ok {
						return
					}
				}
				file := files[index]
				func() {
					finish := telemetry.Start("format.file-worker")
					defer finish()
					defer file.Release()
					if err := workerContext.Err(); err != nil {
						return
					}
					filename := file.Path()
					original, err := file.Bytes()
					if err != nil {
						errorsByFile[index] = fmt.Errorf("%s: %w", source.DisplayPath(filename), err)
						cancel()
						return
					}
					if formatter.IsIgnored(original) {
						formatted[index] = formattedFile{
							filename: filename,
							original: original,
							result: formatter.Result{
								Ignored: true,
							},
						}
						return
					}
					releaseAdmission, err := admission.Acquire(workerContext, workspace.EstimatedCSTBytes(int64(len(original))))
					if err != nil {
						errorsByFile[index] = err
						cancel()
						return
					}
					defer releaseAdmission()
					tree, err := file.CST()
					if err != nil {
						errorsByFile[index] = fmt.Errorf("%s: %w", source.DisplayPath(filename), err)
						cancel()
						return
					}
					result, err := session.FormatTreeKnownActive(filename, tree, options)
					if err != nil {
						errorsByFile[index] = err
						cancel()
						return
					}
					if err := workerContext.Err(); err != nil {
						return
					}
					formatted[index] = formattedFile{
						filename: filename,
						original: original,
						result:   result,
					}
				}()
			}
		}()
	}
dispatch:
	for index := range files {
		select {
		case <-workerContext.Done():
			break dispatch
		case jobs <- index:
		}
	}
	close(jobs)
	group.Wait()
	if err := ctx.Err(); err != nil {
		errorsByFile = append(errorsByFile, err)
	}
	return formatted, errorsByFile
}

func formatFileStatuses(ctx context.Context, files []*workspace.File, options formatter.Options, root string, excludes []string, cache *resultcache.Cache) (
	[]formattedStatus,
	[]error,
) {
	restoreGC := workspace.BeginCSTCollectionWindow()
	defer restoreGC()
	statuses := make([]formattedStatus, len(files))
	errorsByFile := make([]error, len(files))
	if len(files) == 0 {
		return statuses, errorsByFile
	}

	session := formatter.NewFormatter()
	admission := workspace.NewCSTAdmission(0)
	resolvedRoot := source.ResolveRoot(root)
	cacheConfiguration := formatCacheConfiguration(excludes)
	workerContext, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int)
	workers := min(runtime.GOMAXPROCS(0), len(files))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for {
				var index int
				var ok bool
				select {
				case <-workerContext.Done():
					return
				case index, ok = <-jobs:
					if !ok {
						return
					}
				}
				file := files[index]
				func() {
					defer file.Release()
					filename := file.Path()
					if err := workerContext.Err(); err != nil {
						return
					}
					original, err := file.Bytes()
					if err != nil {
						errorsByFile[index] = fmt.Errorf("%s: %w", source.DisplayPath(filename), err)
						cancel()
						return
					}
					cacheKey := formatStatusCacheKey(
						cache,
						session,
						filename,
						source.DiagnosticPath(resolvedRoot, filename),
						original,
						options,
						cacheConfiguration,
					)
					if cached, hit := cache.Get(cacheKey); hit && cached.FormatKnown {
						statuses[index] = formattedStatus{
							filename: filename,
							changed:  cached.FormatChanged,
							ignored:  cached.FormatIgnored,
						}
						return
					}
					if formatter.IsIgnored(original) {
						statuses[index] = formattedStatus{
							filename: filename,
							ignored:  true,
						}
						cache.Store(cacheKey, resultcache.Entry{
							FormatKnown:   true,
							FormatIgnored: true,
						})
						return
					}
					releaseAdmission, err := admission.Acquire(workerContext, workspace.EstimatedCSTBytes(int64(len(original))))
					if err != nil {
						errorsByFile[index] = err
						cancel()
						return
					}
					defer releaseAdmission()
					tree, err := file.CST()
					if err != nil {
						errorsByFile[index] = fmt.Errorf("%s: %w", source.DisplayPath(filename), err)
						cancel()
						return
					}
					if err := workerContext.Err(); err != nil {
						return
					}
					changed, err := session.WouldChangeTreeKnownActive(filename, tree, options)
					if err != nil {
						errorsByFile[index] = err
						cancel()
						return
					}
					if err := workerContext.Err(); err != nil {
						return
					}
					statuses[index] = formattedStatus{
						filename: filename,
						changed:  changed,
					}
					cache.Store(cacheKey, resultcache.Entry{
						FormatKnown:   true,
						FormatChanged: changed,
					})
				}()
			}
		}()
	}
dispatch:
	for index := range files {
		select {
		case <-workerContext.Done():
			break dispatch
		case jobs <- index:
		}
	}
	close(jobs)
	group.Wait()
	if err := ctx.Err(); err != nil {
		errorsByFile = append(errorsByFile, err)
	}
	return statuses, errorsByFile
}

func formatStatusCacheKey(cache *resultcache.Cache, session *formatter.Formatter, filename, logicalPath string, contents []byte, options formatter.Options, configuration string) string {
	if cache == nil {
		return ""
	}
	target := fmt.Sprintf("goos=%s\ngoarch=%s\ncgo=%s\nskip-generated=true", os.Getenv("GOOS"), os.Getenv("GOARCH"), os.Getenv("CGO_ENABLED"))
	return cache.Key([]byte("format-status"), contents, []byte(logicalPath), []byte(session.CacheIdentity(filename, options)), []byte(configuration), []byte(target))
}

func formatCacheConfiguration(excludes []string) string {
	sortedExcludes := append([]string(nil), excludes...)
	sort.Strings(sortedExcludes)
	var identity strings.Builder
	for _, exclude := range sortedExcludes {
		fmt.Fprintf(&identity, "exclude=%s\n", exclude)
	}
	return identity.String()
}

func writeFormattedFiles(files []formattedFile) error {
	changes := make([]filewrite.Change, 0, len(files))
	for _, file := range files {
		if !file.result.Changed {
			continue
		}
		changes = append(changes, filewrite.Change{
			Path:   file.filename,
			Before: file.original,
			After:  file.result.Source,
		})
	}
	return filewrite.WriteBatch(changes)
}
