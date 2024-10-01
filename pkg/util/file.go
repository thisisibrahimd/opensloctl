package util

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/log"
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
			log.Info("found file", "file", filename)
			filesFound = append(filesFound, filename)
			continue
		}

		// filename providedis a directory
		log.Info("found directory", "dir", filename)
		if recursive {
			log.Info("recursvinly finding files in directory", "dir", filename)
			if walkErr := filepath.WalkDir(filename, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					extension := filepath.Ext(path)
					if extension == ".yaml" || extension == ".yml" {
						log.Info("found file", "filename", path)
						filesFound = append(filesFound, path)
					}
				}
				return nil
			}); walkErr != nil {
				log.Fatal("unable to recursive read through the directory", "directory", filename, "err", err)
				return nil, walkErr
			}
			continue

		} else {
			log.Info("will not read directory recursively", "dir", filename)
			files, err := os.ReadDir(filename)
			if err != nil {
				log.Fatal("unable to read files in directory", "directory", filename, "err", err)
				return nil, err
			}

			for _, file := range files {
				if !file.IsDir() {
					extension := filepath.Ext(file.Name())
					if extension == ".yaml" || extension == ".yml" {
						log.Info("found file", "filename", path.Join(filename, file.Name()))
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

	log.Info("successfully found files", "number", len(filesFound))
	return filesFound, nil
}
