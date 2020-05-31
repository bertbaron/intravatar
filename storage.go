package main

import "bytes"

// Storage is an abstraction of a file storage
type Storage interface {
	// Load a file
	Load(filename string) (*bytes.Buffer, error)

	// Save a file
	Save(filename string, data *bytes.Buffer) error

	// Rename renames the file with path from to to
	Rename(from string, to string) error

	// Find the first file in the given directory with name ending with the given prefix
	Find(dir string, prefix string) (string, error)

	// FullName creates the full name for the given path for logging and debugging purposes
	FullName(path string) string
}
