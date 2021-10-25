package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type config struct {
	IntuneWinFile        string                 `yaml:"intuneWinFile"`
	DisplayName          string                 `yaml:"displayName"`
	Description          string                 `yaml:"description"`
	Developer            string                 `yaml:"developer"`
	Publisher            string                 `yaml:"publisher"`
	InstallCommandLine   string                 `yaml:"installCommandLine"`
	UninstallCommandLine string                 `yaml:"uninstallCommandLine"`
	RunAsAccount         string                 `yaml:"runAsAccount"`
	Detection            map[string]interface{} `yaml:"detection"`
}

type packageContents struct {
	setupFile     string
	configFile    string
	detectionFile string
}

func (p *packageContents) createConfig() (*config, error) {
	intuneWinFile := p.setupFile
	if intuneWinFile == "" {
		return &config{}, fmt.Errorf("invalid setupFile in packageContents: %+v", *p)
	}
	intuneWinFile = strings.TrimSuffix(intuneWinFile, `.exe`)
	intuneWinFile = strings.TrimSuffix(intuneWinFile, `.EXE`)
	intuneWinFile = strings.TrimSuffix(intuneWinFile, `.msi`)
	intuneWinFile = strings.TrimSuffix(intuneWinFile, `.MSI`)
	intuneWinFile += intuneWinFile + `.intunewin`
	_, intuneWinFile = filepath.Split(intuneWinFile)

	var cf, df []byte
	var err error
	if FileExists(p.configFile) {
		cf, err = ioutil.ReadFile(p.configFile)
		if err != nil {
			return &config{}, fmt.Errorf("error reading %q: %w", p.configFile, err)
		}
	}
	if FileExists(p.detectionFile) {
		df, err = ioutil.ReadFile(p.detectionFile)
		if err != nil {
			return &config{}, fmt.Errorf("error reading %q: %w", p.detectionFile, err)
		}
	}
	var cfg config
	if len(cf) > 0 {
		err = yaml.Unmarshal(cf, &cfg)
		if err != nil {
			return &cfg, fmt.Errorf("error unmarshaling %q: %w", p.configFile, err)
		}
	}
	var detection map[string]interface{}
	if len(df) > 0 {
		switch {
		case strings.HasSuffix(p.detectionFile, `.yaml`):
			err = yaml.Unmarshal(df, &detection)
			if err != nil {
				return &cfg, fmt.Errorf("error unmarshaling %q: %w", p.detectionFile, err)
			}
			detection[`@odata.type`] = `#microsoft.graph.win32LobAppProductCodeDetection`
		case strings.HasSuffix(p.detectionFile, `.ps1`):
			detection = make(map[string]interface{})
			detection[`@odata.type`] = `#microsoft.graph.win32LobAppPowerShellScriptDetection`
			detection[`enforceSignatureCheck`] = false
			detection[`runAs32Bit`] = false
			detection[`scriptContent`] = base64.StdEncoding.EncodeToString(df)
		}
	}
	cfg.IntuneWinFile = intuneWinFile
	cfg.Detection = detection
	return &cfg, nil
}

func (p *packageContents) assignSetup(path string) {
	p.setupFile = path
}

func (p *packageContents) assignconfig(path string) {
	p.configFile = path
}
func (p *packageContents) assignDetection(path string) {
	p.detectionFile = path
}

type packages map[string]*packageContents

func (p packages) createConfigs() ([]*config, error) {
	var configs []*config
	for k := range p {
		c, err := p[k].createConfig()
		if err != nil {
			return configs, fmt.Errorf("error creating config %q: %w", k, err)
		}
		configs = append(configs, c)
	}
	return configs, nil
}

func (p packages) createConfig(k string) (*config, error) {
	return p[k].createConfig()
}

func (p packages) filter(fullPath string) {
	dir, file := filepath.Split(fullPath)
	if _, ok := p[dir]; !ok {
		p[dir] = &packageContents{}
	}
	switch {
	case strings.HasSuffix(file, `yaml`):
		switch {
		case strings.HasPrefix(file, `config`):
			p[dir].assignconfig(fullPath)
		default:
			p[dir].assignDetection(fullPath)
		}
	case strings.HasSuffix(file, `ps1`):
		p[dir].assignDetection(fullPath)
	default:
		p[dir].assignSetup(fullPath)
	}
}
