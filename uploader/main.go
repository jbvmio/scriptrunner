package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

var (
	certFile        string
	keyFile         string
	caCertFile      string
	destinationURL  string
	uploadDirectory string
)

func main() {
	pf := pflag.NewFlagSet(`uploader`, pflag.ExitOnError)
	pf.StringVar(&destinationURL, "url", "", "Destination URL.")
	pf.StringVar(&uploadDirectory, "dir", "", "Directory of Files to Upload.")
	pf.StringVar(&caCertFile, "cacert", "./ca.crt", "Filepath to Signing Certificate CA.")
	pf.StringVar(&certFile, "cert", "client.crt", "Filepath to Client Certificate.")
	pf.StringVar(&keyFile, "key", "client.key", "Filepath to Client Key.")
	pf.Parse(os.Args[1:])
	args := pf.Args()
	var dirFiles []DirFile
	if uploadDirectory != "" {
		dirFiles = GetAllDirFiles(uploadDirectory)
	}
	for _, a := range args {
		if FileExists(a) {
			fi, err := os.Stat(a)
			switch {
			case err != nil:
				fmt.Fprintf(os.Stderr, "STAT Error for file %q: %v\n", a, err)
			default:
				switch {
				case err != nil:
					fmt.Fprintf(os.Stderr, "ABS Error for file %q: %v\n", a, err)
				default:
					switch {
					case fi.IsDir():
						dirFiles = append(dirFiles, GetAllDirFiles(a)...)
					default:
						name := fi.Name()
						dirFiles = append(dirFiles, DirFile{
							Name:     name,
							FullPath: a,
							OutPath:  name,
						})
					}
				}
			}
		}
	}
	if len(dirFiles) < 1 {
		log.Println("No Files to Upload, Exiting ...")
		os.Exit(0)
	}

	for _, a := range dirFiles {
		fmt.Println(a.OutPath)
	}
	os.Exit(0)

	client := http.Client{
		Timeout: time.Minute * 1,
		Transport: &http.Transport{
			TLSClientConfig: getTLSConfig(certFile, keyFile, caCertFile),
		},
	}

uploadDirFiles:
	for _, file := range dirFiles {
		b, err := ioutil.ReadFile(file.FullPath)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "Error reading file %q: %v\n", file.FullPath, err)
			continue uploadDirFiles
		default:
			params := url.Values{}
			d, f := filepath.Split(file.OutPath)
			if d != "" {
				params.Add(`directory`, d)
			}
			if f != "" {
				params.Add(`filename`, f)
			}
			U := destinationURL + `?` + params.Encode()
			req, err := http.NewRequest(`POST`, U, bytes.NewBuffer(b))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating request for file %q: %v\n", file.FullPath, err)
				continue uploadDirFiles
			}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error sending request for file %q: %v\n", file.FullPath, err)
				continue uploadDirFiles
			}
			defer resp.Body.Close()
			raw, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading response for file %q: %v\n", file.FullPath, err)
				continue uploadDirFiles
			}
			fmt.Printf("%s\n", raw)
		}
	}
}

// GetDirFiles retrives all the immediate files within the given directory and returns file mapping
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

// GetAllDirFiles traverses and retrives all the files within the given directory and returns file mapping
// indicating whether the file is a Directory.
func GetAllDirFiles(dir string) []DirFile {
	var dirFiles []DirFile
	filepath.WalkDir(dir, func(path string, info fs.DirEntry, err error) error {
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "error accessing path %q: %v\n", path, err)
		default:
			switch {
			case info.IsDir():
				switch info.Name() {
				case `.git`:
					return filepath.SkipDir
				}
			default:
				dirFiles = append(dirFiles, DirFile{
					Name:     info.Name(),
					FullPath: path,
					OutPath:  makeOutPath(dir, path),
				})
			}
		}
		return nil
	})
	return dirFiles
}

// FileExists checks for the existence of the file indicated by filename and returns true if it exists.
func FileExists(filename string) bool {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}
	return true
}

// DirFile contains details for a file.
type DirFile struct {
	Name     string
	FullPath string
	OutPath  string
	IsDir    bool
}

func makeOutPath(dir, path string) string {
	dir = strings.ReplaceAll(dir, `\`, `/`)
	path = strings.ReplaceAll(path, `\`, `/`)
	_, root := filepath.Split(dir)
	p := strings.TrimPrefix(path, dir)
	fmt.Printf("DIR: %s\tPATH: %s\tROOT: %s\tPATH: %s\n", dir, path, root, p)
	return filepath.Join(root, p)
}

func getTLSConfig(clientCert, clientKey, caCertFile string) *tls.Config {
	var cert tls.Certificate
	var err error
	if clientCert != "" && clientKey != "" {
		cert, err = tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			log.Fatalf("Error creating x509 keypair from client cert file %q and client key file %q: %v\n", clientCert, clientKey, err)
		}
	}
	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		log.Fatalf("Error opening cert file %q: %v\n", caCertFile, err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}
}
