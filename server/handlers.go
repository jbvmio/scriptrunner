package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/tidwall/pretty"
)

func handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(w, http.StatusOK, `OK`)
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		vars := mux.Vars(r)
		name := vars[`filename`]
		Q := r.URL.Query()
		if n := Q.Get("filename"); n != "" {
			name = n
		}
		if name == "" {
			writeJSONError(w, "error creating upload file", fmt.Errorf("filename not specified"))
			return
		}
		dirPath := Q.Get("directory")
		switch dirPath {
		case `/`, "":
		default:
			d := filepath.Join(srvFiles, dirPath)
			err := createDir(d)
			if err != nil {
				writeJSONError(w, `error creating upload path "`+d+`"`, err)
				return
			}
		}
		output := filepath.Join(srvFiles, dirPath, name)
		log.Printf("saving uploaded file %s to %s\n", name, output)
		switch {
		case r.Body != nil:
			defer r.Body.Close()
			file, err := os.Create(output)
			if err != nil {
				writeJSONError(w, "error creating upload file", err)
				return
			}
			n, err := io.Copy(file, r.Body)
			if err != nil {
				writeJSONError(w, "error writing upload file", err)
				return
			}
			err = os.Chmod(output, 0777)
			if err != nil {
				writeJSONError(w, "error setting permissions for upload file", err)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("%d bytes recieved.\n", n)))
		default:
			writeJSONError(w, "error uploading file", fmt.Errorf("received empty body"))
		}
	default:
		writeJSONErrorWithCode(w, "method not allowed", fmt.Errorf("invalid method: %v", r.Method), http.StatusMethodNotAllowed)
		return
	}
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if jsonBytes, err := json.Marshal(obj); err != nil {
		writeJSONError(w, `could not encode JSON`, err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(statusCode)
		w.Write(pretty.Pretty(jsonBytes))
	}
}

func writeJSONError(w http.ResponseWriter, msg string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(`{"error":true,"message":"` + msg + `","error":"` + err.Error() + `"}`))
}

func writeJSONErrorWithCode(w http.ResponseWriter, msg string, err error, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write([]byte(`{"error":true,"message":"` + msg + `","error":"` + err.Error() + `"}`))
}

func createDir(path string) error {
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
