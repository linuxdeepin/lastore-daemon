package fs

import (
	"errors"
	"os"
	pf "path/filepath"
)

func CreateFile(filepath string) (*os.File, error) {
	absPath := pf.Dir(filepath)
	if absPath == "" {
		return nil, errors.New("not found base name")
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		os.MkdirAll(absPath, 0755)
	}
	out, err := os.Create(filepath)
	return out, err
}

func ReadMode(filepath string) (os.FileMode, error) {
	info, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return info.Mode(), nil
}

func CreateDirMode(dir string, mode os.FileMode) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, mode)
	}
	return nil
}
