package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// API for HomeBase.
type API struct {
	httpSrv http.Server
	lock    sync.Mutex
	wg      sync.WaitGroup
	logger  *zap.Logger
}

// NewAPI returns a new API.
func NewAPI(host, port, caCertFile string, certOpt tls.ClientAuthType, L *zap.Logger) *API {
	A := &API{
		lock:   sync.Mutex{},
		wg:     sync.WaitGroup{},
		logger: L.With(zap.String(`process`, `HomeBase API`)),
	}
	A.makeHTTPSrv(host, port, caCertFile, certOpt)
	return A
}

// Start starts the API.
func (a *API) Start(certFile, keyFile string) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.logger.Info("Starting ... Listening on " + a.httpSrv.Addr)
		if err := a.httpSrv.ListenAndServeTLS(certFile, keyFile); err != nil {
			if !strings.Contains(err.Error(), `Server closed`) {
				a.logger.Fatal("http server encountered an error", zap.Error(err))
			}
		}
		a.logger.Info("HTTP Server Stopped.")
	}()
}

// Stop stops the API.
func (a *API) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err := a.httpSrv.Shutdown(ctx)
	if err != nil {
		a.logger.Error("error shutting down", zap.Error(err))
	}
	a.logger.Info("Stopping ...")
	<-ctx.Done()
	a.wg.Wait()
	a.logger.Info("All Processes Stopped.")
}

func (a *API) makeHTTPSrv(host, port, caCertFile string, certOpt tls.ClientAuthType) {
	r := mux.NewRouter()
	r.HandleFunc(`/`, handleStatus)
	a.httpSrv = http.Server{
		Handler:      r,
		Addr:         `:` + port,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		TLSConfig:    getTLSConfig(host, caCertFile, certOpt),
	}
}

func getTLSConfig(host, caCertFile string, certOpt tls.ClientAuthType) *tls.Config {
	var caCert []byte
	var err error
	var caCertPool *x509.CertPool
	if certOpt > tls.RequestClientCert {
		caCert, err = ioutil.ReadFile(caCertFile)
		if err != nil {
			log.Fatal("Error opening cert file", caCertFile, ", error ", err)
		}
		caCertPool = x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
	}

	return &tls.Config{
		ServerName: host,
		ClientAuth: certOpt,
		ClientCAs:  caCertPool,
		MinVersion: tls.VersionTLS12, // TLS versions below 1.2 are considered insecure - see https://www.rfc-editor.org/rfc/rfc7525.txt for details
	}
}
