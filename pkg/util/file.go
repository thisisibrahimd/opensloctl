package util

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
)

func FindFiles(filenames []string, recursive bool) ([]string, error) {
	// ensure a file name is provided
	if len(filenames) < 1 {
		return nil, errors.New("at least one filename must be provided")
	}

	// go through provided filenames
	var filesFound []string
	for _, filename := range filenames {
		// ensure we can access the file or dir
		fileInfo, err := os.Stat(filename)
		if err != nil {
			return nil, err
		}

		// filename provided is a file
		if !fileInfo.IsDir() {
			slog.Info("found file", "file", filename)
			filesFound = append(filesFound, filename)
			continue
		}

		// filename provided is a directory
		slog.Info("found directory", "dir", filename)
		if recursive {
			slog.Info("recursively finding files in directory", "dir", filename)
			if walkErr := filepath.WalkDir(filename, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					extension := filepath.Ext(path)
					if extension == ".yaml" || extension == ".yml" || extension == ".json" {
						slog.Info("found file", "filename", path)
						filesFound = append(filesFound, path)
					}
				}
				return nil
			}); walkErr != nil {
				slog.Error("unable to recursively read through the directory", "directory", filename, "err", walkErr)
				return nil, walkErr
			}
			continue

		} else {
			slog.Info("will not read directory recursively", "dir", filename)
			files, err := os.ReadDir(filename)
			if err != nil {
				slog.Error("unable to read files in directory", "directory", filename, "err", err)
				return nil, err
			}

			for _, file := range files {
				if !file.IsDir() {
					extension := filepath.Ext(file.Name())
					if extension == ".yaml" || extension == ".yml" {
						slog.Info("found file", "filename", path.Join(filename, file.Name()))
						filesFound = append(filesFound, path.Join(filename, file.Name()))
					}
				}
			}
			continue
		}
	}

	// remove duplicates if present
	slices.Sort(filesFound)
	filesFound = slices.Compact(filesFound)

	slog.Info("successfully found files", "number", len(filesFound))
	return filesFound, nil
}
