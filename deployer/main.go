package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jbvmio/go-msgraph"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const (
	manifestFile = `manifest.json`
	configFile   = `configuration.yaml`
)

var (
	certFile     string
	keyFile      string
	caCertFile   string
	tenantID     string
	appID        string
	appSecret    string
	hbCfg        string
	appRegex     string
	waitInterval time.Duration
)

func main() {
	pf := pflag.NewFlagSet(`deployer`, pflag.ExitOnError)
	pf.StringVar(&hbCfg, "config", "", "Path to HomeBase Config.")
	pf.StringVar(&caCertFile, "cacert", "./ca.crt", "Filepath to Signing Certificate CA.")
	pf.StringVar(&certFile, "cert", "client.crt", "Filepath to Client Certificate.")
	pf.StringVar(&keyFile, "key", "client.key", "Filepath to Client Key.")
	pf.StringVar(&tenantID, "T", "", "Tenant ID.")
	pf.StringVar(&appID, "I", "", "App ID.")
	pf.StringVar(&appSecret, "S", "", "App Secret.")
	pf.StringVar(&appRegex, "filter", "", "App (intunewin filename) Filter Regex.")
	pf.DurationVarP(&waitInterval, "wait", "w", time.Second*10, "Wait Interval between App Deployments.")
	pf.Parse(os.Args[1:])

	// Add more validation here:
	if hbCfg == "" {
		log.Fatalf("hbConfig not provided!\n")
	}

	var regex *regexp.Regexp
	var err error
	if appRegex != "" {
		regex, err = regexp.Compile(appRegex)
		if err != nil {
			log.Fatalf("invalid appFilter regex %q: %v\n", appRegex, err)
		}
	}

	var HB hbConfig
	err = HB.readConfig(hbCfg)
	if err != nil {
		log.Fatalf("error reading hbConfig: %v\n", err)
	}
	rootURL := HB.UploadURL
	rootURL = strings.Replace(rootURL, `upload`, `files/intune`, 1)

	tenant, err := createContext(tenantID, appID, appSecret, HB.Defaults)
	if err != nil {
		log.Fatalf("error creating tenant context: %v\n", err)
	}

	client := http.Client{
		Timeout: time.Minute * 1,
		Transport: &http.Transport{
			TLSClientConfig: getTLSConfig(certFile, keyFile, caCertFile),
		},
	}

	packages, err := getManifest(&client, rootURL+`/`+manifestFile)
	if err != nil {
		log.Fatalf("error retreiving manifest: %v\n", err)
	}
	fmt.Println("Package Manifest:", packages)

	configs, err := getConfig(&client, rootURL+`/`+configFile)
	if err != nil {
		log.Fatalf("error retreiving config: %v\n", err)
	}

	if regex != nil {
		configs = filterConfigs(configs, regex)
	}

	switch len(configs) {
	case 0:
		log.Println("no configurations to deploy, exiting.")
		os.Exit(0)
	case 1:
		waitInterval = time.Millisecond * 500
	}

	for _, c := range configs {
		file, err := getIntuneWinFile(&client, rootURL+`/`+c.IntuneWinFile)
		if err != nil {
			log.Printf("error retreiving file %q from %q: %v\n", c.IntuneWinFile, rootURL, err)
			continue
		}
		err = tenant.deployIntuneWin(c, file)
		if err != nil {
			log.Printf("error deploying file %q from %q: %v\n", c.IntuneWinFile, rootURL, err)
			continue
		}
		file.Close()
		time.Sleep(waitInterval)
	}
}

func getIntuneWinFile(client *http.Client, URL string) (io.ReadCloser, error) {
	req, err := http.NewRequest(`GET`, URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		if resp.Body != nil {
			resp.Body.Close()
		}
		return nil, err
	}
	return resp.Body, nil
}

func getConfig(client *http.Client, URL string) ([]config, error) {
	var packages []config
	req, err := http.NewRequest(`GET`, URL, nil)
	if err != nil {
		return packages, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return packages, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return packages, err
	}
	err = yaml.Unmarshal(b, &packages)
	return packages, err
}

func filterConfigs(configs []config, regex *regexp.Regexp) []config {
	var tmp []config
	for _, c := range configs {
		if regex.MatchString(c.IntuneWinFile) {
			tmp = append(tmp, c)
		}
	}
	return tmp
}

func getManifest(client *http.Client, URL string) ([]string, error) {
	var packages []string
	req, err := http.NewRequest(`GET`, URL, nil)
	if err != nil {
		return packages, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return packages, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return packages, err
	}
	err = json.Unmarshal(b, &packages)
	return packages, err
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

type config struct {
	IntuneWinFile        string                 `yaml:"intuneWinFile"`
	DisplayName          string                 `yaml:"displayName"`
	Description          string                 `yaml:"description"`
	Developer            string                 `yaml:"developer"`
	Publisher            string                 `yaml:"publisher"`
	InstallCommandLine   string                 `yaml:"installCommandLine"`
	UninstallCommandLine string                 `yaml:"uninstallCommandLine"`
	Detection            map[string]interface{} `yaml:"detection"`
	DisplayNamePrefix    string                 `json:"displayNamePrefix"`
}

type hbConfig struct {
	IPAddress string `json:"ipAddress"`
	UploadURL string `json:"uploadURL"`
	Defaults  config `json:"defaults"`
}

func (h *hbConfig) readConfig(path string) error {
	if h == nil {
		h = new(hbConfig)
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, h)
}

// Context contains the connecting details for a tenant.
type context struct {
	TenantID  string `yaml:"tenantID" json:"tenantID"`
	AppID     string `yaml:"appID" json:"appID"`
	AppSecret string `yaml:"appSecret" json:"appSecret"`

	graphClient *msgraph.GraphClient
	Defaults    config
}

func (C *context) setDefaults(appReq *msgraph.Win32LobAppRequest) {
	if appReq.DisplayName == "" {
		appReq.DisplayName = C.Defaults.DisplayName
	}
	if C.Defaults.DisplayNamePrefix != "" {
		appReq.DisplayName = C.Defaults.DisplayNamePrefix + appReq.DisplayName
	}
	if appReq.Developer == "" || appReq.Developer == `GoMSGraph` {
		appReq.Developer = C.Defaults.Developer
	}
	if appReq.Publisher == "" || appReq.Publisher == `GoMSGraph` {
		appReq.Publisher = C.Defaults.Publisher
	}
}

func createContext(tenantID, appID, secretID string, defaults config) (*context, error) {
	graphClient, err := msgraph.NewGraphClient(tenantID, appID, appSecret)
	if err != nil {
		return &context{}, fmt.Errorf("credentials are probably wrong or system time is not synced: %w", err)
	}
	return &context{
		TenantID:    tenantID,
		AppID:       appID,
		AppSecret:   appSecret,
		graphClient: graphClient,
		Defaults:    defaults,
	}, nil
}

func (C *context) deployIntuneWin(cfg config, intuneWinFile io.Reader) error {
	fb, err := ioutil.ReadAll(intuneWinFile)
	if err != nil {
		return fmt.Errorf("read file error: %w", err)
	}
	xmlMeta, err := msgraph.GetIntuneWin32AppMetadata(bytes.NewReader(fb), false)
	if err != nil {
		return fmt.Errorf("xmlMeta Err: %w", err)
	}
	appReq := msgraph.NewWin32LobAppRequest(xmlMeta)
	processAppRequest(&appReq, &cfg)
	C.setDefaults(&appReq)
	app, err := C.graphClient.CreateWin32LobApp(appReq)
	if err != nil {
		return fmt.Errorf("create new win32lobapp error: %w", err)
	}
	fileContentReq := msgraph.NewMobileAppContentFileRequest(xmlMeta)
	fileContent, err := app.CreateContentFile(fileContentReq)
	if err != nil {
		return fmt.Errorf("create content file error: %w", err)
	}
	err = fileContent.UploadIntuneWin(bytes.NewReader(fb))
	if err != nil {
		return fmt.Errorf("upload file error: %w", err)
	}
	return nil
}

func processAppRequest(appReq *msgraph.Win32LobAppRequest, cfg *config) {
	if cfg.Developer != "" {
		appReq.Developer = cfg.Developer
	}
	if cfg.Publisher != "" {
		appReq.Publisher = cfg.Publisher
	}
	if cfg.DisplayName != "" {
		appReq.DisplayName = cfg.DisplayName
	}
	if cfg.Description != "" {
		appReq.Description = cfg.Description
	}
	if cfg.InstallCommandLine != "" {
		appReq.InstallCommandLine = cfg.InstallCommandLine
	}
	if cfg.UninstallCommandLine != "" {
		appReq.UninstallCommandLine = cfg.UninstallCommandLine
	}
	if len(cfg.Detection) > 0 {
		appReq.DetectionRules = []msgraph.Win32LobAppDetection{cfg.Detection}
	}
}
