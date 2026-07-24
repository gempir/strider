//strider:ignore-file cognitive-complexity,cyclomatic-complexity,function-length
package app

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/gempir/strider/internal/filewrite"
	"github.com/gempir/strider/internal/formatter"
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
	formatted := make([]formattedFile, len(files))
	errorsByFile := make([]error, len(files))
	if len(files) == 0 {
		return formatted, errorsByFile
	}

	session := formatter.NewFormatter()
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

func formatFileStatuses(ctx context.Context, files []*workspace.File, options formatter.Options) ([]formattedStatus, []error) {
	statuses := make([]formattedStatus, len(files))
	errorsByFile := make([]error, len(files))
	if len(files) == 0 {
		return statuses, errorsByFile
	}

	session := formatter.NewFormatter()
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
				if formatter.IsIgnored(original) {
					statuses[index] = formattedStatus{
						filename: filename,
						ignored:  true,
					}
					continue
				}
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
