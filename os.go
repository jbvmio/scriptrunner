package scriptrunner

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirFile contains details for a file.
type DirFile struct {
	Name     string
	FullPath string
	IsDir    bool
}

// CleanDirectory takes the given directory path and deletes everything under that directory.
func CleanDirectory(path string) error {
	files, err := GetDirFiles(path)
	if err != nil {
		return fmt.Errorf("error retrieving directoy files: %w", err)
	}
	var errFiles []string
	var errd error
	for _, f := range files {
		switch {
		case f.IsDir:
			err = os.RemoveAll(f.FullPath)
		default:
			err = os.Remove(f.FullPath)
		}
		if err != nil {
			errd = err
			errFiles = append(errFiles, f.Name)
		}
	}
	if len(errFiles) > 0 {
		return fmt.Errorf("error deleting %d files: %v : lasterr: %v", len(errFiles), errFiles, errd)
	}
	return nil
}

// GetArchiveFiles retrives all the files within the given directory.
func GetArchiveFiles(dir string) ([]string, error) {
	var filenames []string
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return filenames, err
	}
	for _, f := range files {
		if !f.IsDir() {
			fType := GetFileContentType(filepath.Join(dir, f.Name()))
			switch fType {
			case "":
				fmt.Fprintf(os.Stderr, "error determining type for %q, skipping...\n", f.Name())
			case `application/zip`:
				filenames = append(filenames, f.Name())
			}
		}
	}
	sort.SliceStable(filenames, func(i, j int) bool {
		return filenames[i] < filenames[j]
	})
	return filenames, nil
}

// GetDirFiles retrives all the files within the given directory and returns file mapping
// indicating whether the file is a Directory.
func GetDirFiles(dir string) ([]DirFile, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return []DirFile{}, err
	}
	filenames := make([]DirFile, 0, len(files))
	for _, f := range files {
		filenames = append(filenames, DirFile{
			Name:     f.Name(),
			FullPath: filepath.Join(dir, f.Name()),
			IsDir:    f.IsDir(),
		})
	}
	sort.SliceStable(filenames, func(i, j int) bool {
		return filenames[i].Name < filenames[j].Name
	})
	return filenames, nil
}

// GetCWD returns the current working directory.
func GetCWD() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	exPath := filepath.Dir(ex)
	return exPath, nil
}

// UnZip unzips a file.
func UnZip(srcFile, dstDir string) error {
	archive, err := zip.OpenReader(srcFile)
	if err != nil {
		return fmt.Errorf("error opening archive file: %w", err)
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(dstDir, f.Name)
		fmt.Println("unzipping file ", filePath)

		if !strings.HasPrefix(filePath, filepath.Clean(dstDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path")
		}
		if f.FileInfo().IsDir() {
			fmt.Println("creating directory...")
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return fmt.Errorf("error creating directory: %w", err)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("error opening file: %w", err)
		}

		fileInArchive, err := f.Open()
		if err != nil {
			return fmt.Errorf("error opening archive file: %w", err)
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return fmt.Errorf("error processing archive file: %w", err)
		}

		dstFile.Close()
		fileInArchive.Close()
	}
	return nil
}

func GetFileContentType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buffer := make([]byte, 512)
	_, err = f.Read(buffer)
	if err != nil {
		return ""
	}
	contentType := http.DetectContentType(buffer)
	return contentType
}

func CreateDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, 0754)
		if err != nil {
			return err
		}
		err = os.Chmod(path, 0754)
		if err != nil {
			return err
		}
	}
	return nil
}
