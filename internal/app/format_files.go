package app

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/gempir/strider/internal/filewrite"
	"github.com/gempir/strider/internal/formatter"
	"github.com/gempir/strider/internal/source"
	"github.com/gempir/strider/internal/workspace"
)

type formattedFile struct {
	filename string
	original []byte
	result   formatter.Result
}

func formatFiles(files []*workspace.File, options formatter.Options, verify bool) ([]formattedFile, []error) {
	formatted := make([]formattedFile, len(files))
	errorsByFile := make([]error, len(files))
	if len(files) == 0 {
		return formatted, errorsByFile
	}

	session := formatter.NewFormatter()
	jobs := make(chan int)
	workers := min(runtime.GOMAXPROCS(0), len(files))
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				file := files[index]
				func() {
					defer file.Release()
					filename := file.Path()
					original, err := file.Bytes()
					if err != nil {
						errorsByFile[index] = fmt.Errorf("%s: %w", source.DisplayPath(filename), err)
						return
					}
					if formatter.IsIgnored(original) {
						formatted[index] = formattedFile{
							filename: filename,
							original: original,
							result: formatter.Result{
								Source:  append([]byte(nil), original...),
								Ignored: true,
							},
						}
						return
					}
					tree, err := file.CST()
					if err != nil {
						errorsByFile[index] = fmt.Errorf("%s: %w", source.DisplayPath(filename), err)
						return
					}
					var result formatter.Result
					if verify {
						result, err = session.FormatTree(filename, tree, options)
					} else {
						result, err = session.FormatTreeUnverified(filename, tree, options)
					}
					if err != nil {
						errorsByFile[index] = err
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
	for index := range files {
		jobs <- index
	}
	close(jobs)
	group.Wait()
	return formatted, errorsByFile
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
