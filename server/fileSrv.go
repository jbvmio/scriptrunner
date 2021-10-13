package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type fileSrv struct {
	httpSrv http.Server
	lock    sync.Mutex
	wg      sync.WaitGroup
	logger  *zap.Logger
}

// NewFIleSrv .
func NewFileSrv(host, port, srvFiles string, caCertFile string, certOpt tls.ClientAuthType, L *zap.Logger) *fileSrv {
	F := &fileSrv{
		lock:   sync.Mutex{},
		wg:     sync.WaitGroup{},
		logger: L.With(zap.String(`process`, `HomeBase FileServer`)),
	}
	F.makeHTTPSrv(host, port, srvFiles, caCertFile, certOpt)
	return F
}

// Start .
func (fs *fileSrv) Start(certFile, keyFile string) {
	fs.wg.Add(1)
	go func() {
		defer fs.wg.Done()
		fs.logger.Info("Starting ... Listening on " + fs.httpSrv.Addr)
		if err := fs.httpSrv.ListenAndServeTLS(certFile, keyFile); err != nil {
			if !strings.Contains(err.Error(), `Server closed`) {
				fs.logger.Fatal("http server encountered an error", zap.Error(err))
			}
		}
		fs.logger.Info("HTTP Server Stopped.")
	}()
}

// Stop .
func (fs *fileSrv) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := fs.httpSrv.Shutdown(ctx)
	if err != nil {
		fs.logger.Error("error shutting down", zap.Error(err))
	}
	fs.logger.Info("Stopping ...")
	<-ctx.Done()
	fs.wg.Wait()
	fs.logger.Info("All Processes Stopped.")
}

func (fs *fileSrv) makeHTTPSrv(host, port, srvFiles, caCertFile string, certOpt tls.ClientAuthType) {
	r := mux.NewRouter()
	r.HandleFunc(`/`, handleStatus)
	r.HandleFunc(`/upload`, uploadFileHandler)
	r.HandleFunc(`/upload/{filename}`, uploadFileHandler)
	r.PathPrefix(filesPath).Handler(http.StripPrefix(filesPath, http.FileServer(http.Dir(srvFiles))))
	fs.httpSrv = http.Server{
		Handler:      r,
		Addr:         `:` + port,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		TLSConfig:    getTLSConfig(host, caCertFile, certOpt),
	}
}
