package gisquick

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

type FileInfo struct {
	Path  string `json:"path"`
	Hash  string `json:"hash"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

func DBHash(path string) (string, error) {
	cmdOut, err := exec.Command("dbhash", path).Output()
	if err != nil {
		log.Println("ErrNotFound", errors.Is(err, exec.ErrNotFound))
		return "", fmt.Errorf("executing dbhash command: %w", err)
	}
	hash := strings.Split(string(cmdOut), " ")[0]
	return hash, nil
}

// Computes SHA-1 hash of file
func Sha1(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	h := sha1.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Computes hash of the file (SHA-1 or dbhash)
func Checksum(path string) (string, error) {
	if strings.ToLower(filepath.Ext(path)) == ".gpkg" {
		dbhash, err := DBHash(path)
		if err == nil {
			return "dbhash:" + dbhash, nil
		}
	}
	return Sha1(path)
}

// Collects information about files in given directory
func ListDir(root string, checksum bool) (*[]FileInfo, error) {
	var files []FileInfo = []FileInfo{}
	excludeExtRegex := regexp.MustCompile(`(?i).*\.(gpkg-wal|gpkg-shm)$`)
	defaultFileFilter := func(path string) bool {
		return !strings.HasSuffix(path, "~") && !excludeExtRegex.Match([]byte(path))
	}
	fileFilter := defaultFileFilter

	matcher, err := ignore.CompileIgnoreFile(filepath.Join(root, ".gisquickignore"))
	if err == nil {
		fileFilter = func(path string) bool {
			return defaultFileFilter(path) && !matcher.MatchesPath(path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return &files, fmt.Errorf("parsing .gisquickignore file: %w", err)
	}

	root, _ = filepath.Abs(root)
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath := path[len(root)+1:]
			if fileFilter(relPath) {
				hash := ""
				if checksum {
					if hash, err = Checksum(path); err != nil {
						return err
					}
				}
				files = append(files, FileInfo{relPath, hash, info.Size(), info.ModTime().Unix()})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &files, nil
}

// Saves content from given reader into the file
func SaveToFile(src io.Reader, filename string) (err error) {
	err = os.MkdirAll(filepath.Dir(filename), os.ModePerm)
	if err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	// more verbose but with better errors propagation
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, err := io.Copy(file, src); err != nil {
		return err
	}
	return nil
}

// Writes content of the file into given writer
func CopyFile(dest io.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(dest, file)
	return err
}
