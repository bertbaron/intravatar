package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// FileStorage is backed by the local file system
type FileStorage struct {
	root string
}

// NewFileStorage creates a new file storage using the local file system
func NewFileStorage(root string) *FileStorage {
	return &FileStorage{root: root}
}

func (s *FileStorage) createPath(filename string) string {
	if path.IsAbs(filename) {
		return filename
	}
	return path.Join(s.root, filename)
}

// Load a file from local file system
func (s *FileStorage) Load(filename string) (*bytes.Buffer, error) {
	path := s.createPath(filename)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b := new(bytes.Buffer)
	if _, e := io.Copy(b, file); e != nil {
		return nil, e
	}
	return b, nil
}

// Save a file to the local file system
func (s *FileStorage) Save(filename string, data *bytes.Buffer) error {
	path := s.createPath(filename)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, data)
	return err
}

// Rename renames the file with path from to to
func (s *FileStorage) Rename(from string, to string) error {
	return os.Rename(s.createPath(from), s.createPath(to))
}

// Find the first filename in the given directory with name ending with the given prefix
func (s *FileStorage) Find(dir string, prefix string) (string, error) {
	files, err := ioutil.ReadDir(s.createPath(dir))
	if err != nil {
		return "", err
	}
	for _, file := range files {
		filename := file.Name()
		if strings.HasPrefix(filename, prefix) {
			return filename, nil
		}
	}
	return "", nil
}

// FullName creates the full name for the given path for logging and debugging purposes
func (s *FileStorage) FullName(path string) string {
	return s.createPath(path)
}