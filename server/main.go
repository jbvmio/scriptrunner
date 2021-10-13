package main

import (
	"crypto/tls"
	"os"
	"os/signal"
	"syscall"

	"github.com/jbvmio/scriptrunner"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const certOpt = tls.RequireAndVerifyClientCert

var (
	host        string
	port        string
	caCertFile  string
	svrCertFile string
	svrKeyFile  string
	srvFiles    string
	buildTime   string
	commitHash  string
)

const (
	filesPath = `/files/`
)

func main() {
	pf := pflag.NewFlagSet(`homebase`, pflag.ExitOnError)
	pf.StringVarP(&host, "host", "h", "localhost", "Name of Host receiving requests.")
	pf.StringVarP(&port, "port", "p", "8080", "Port to Listen on.")
	pf.StringVar(&caCertFile, "cacert", "./ca.crt", "Filepath to Signing Certificate CA.")
	pf.StringVar(&svrCertFile, "cert", "server.crt", "Filepath to Server Certificate.")
	pf.StringVar(&svrKeyFile, "key", "server.key", "Filepath to Server Certificate.")
	pf.StringVar(&srvFiles, "filesrv", "", "Run FileServer using the Given Directory.")
	pf.Parse(os.Args[1:])

	l := scriptrunner.ConfigureLevel(`info`)
	L := scriptrunner.ConfigureLogger(l, os.Stdout)
	L.Info("Starting ...", zap.String(`Version`, buildTime), zap.String(`Commit`, commitHash))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	switch {
	case srvFiles != "":
		fSrv := NewFileSrv(host, port, srvFiles, caCertFile, certOpt, L)
		fSrv.Start(svrCertFile, svrKeyFile)

		<-sigChan

		fSrv.Stop()
		L.Info("Stopped.")

	default:
		api := NewAPI(host, port, caCertFile, certOpt, L)
		api.Start(svrCertFile, svrKeyFile)

		<-sigChan

		api.Stop()
		L.Info("Stopped.")
	}

}
