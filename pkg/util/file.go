package util

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/log"
)

func FindFiles(filenames []string, recursive bool) ([]string, error) {
	var filesFound []string

	// ensure a file name is provided
	if len(filenames) < 1 {
		return nil, fmt.Errorf("at least one filename must be provided")
	}

	for _, filename := range filenames {
		// Ensure we access the file or dir
		fileInfo, err := os.Stat(filename)
		if err != nil {
			log.Fatal("unable to read file", "file", filename, "err", err)
			return nil, err
		}

		// Filename is a FILE
		if !fileInfo.IsDir() {
			log.Info("file was provided", "file", filename)
			filesFound = append(filesFound, filename)
			continue
		}

		// Filename is a DIRECTORY
		log.Info("directory was provided", "dir", filename)
		if recursive {
			log.Info("will read directory recursively", "dir", filename)
			if walkErr := filepath.WalkDir(filename, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() {
					log.Info("found file", "filename", path)
					filesFound = append(filesFound, path)
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
					log.Info("found file", "filename", path.Join(filename, file.Name()))
					filesFound = append(filesFound, path.Join(filename, file.Name()))
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

func LoadFiles(filenames []string, recursive bool) ([][]byte, error) {
	var files [][]byte

	log.Info("starting to load files", "number_of_files_to_load", len(filenames))

	filesToLoad, err := FindFiles(filenames, recursive)
	if err != nil {
		return nil, err
	}

	for _, fileToLoad := range filesToLoad {
		log.Info("loading file", "file", fileToLoad)

		file, err := os.ReadFile(filepath.Clean(fileToLoad))
		if err != nil {
			log.Error("unable to read file", "file", fileToLoad)
			return nil, err
		}

		files = append(files, file)
		log.Info("successfully loaded file", "file", fileToLoad)
	}

	log.Info("successfully loaded files", "number", len(files))

	return files, nil
}
