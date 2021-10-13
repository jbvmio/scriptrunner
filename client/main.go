package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jbvmio/scriptrunner"
	"github.com/jbvmio/scriptrunner/powershell"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	configFile   = `config.yaml`
	scriptsDir   = `scripts`
	workspaceDir = `workspace`
	certsDir     = `certs`
)

var (
	config         string
	homeBaseURL    string
	caCertFile     string
	clientCertFile string
	clientKeyFile  string
	buildTime      string
	commitHash     string
)

func main() {
	pf := pflag.NewFlagSet("scriptrunner", pflag.ExitOnError)
	pf.StringVarP(&config, `config`, `c`, "", "Path of Config File to Use, Overwriting Defaults.")
	pf.StringVarP(&homeBaseURL, `homebase`, `h`, "", "Alternate HomeBase URL to use, Overwrites Config HomeBase Value.")
	pf.Parse(os.Args[1:])

	l := scriptrunner.ConfigureLogger(scriptrunner.ConfigureLevel(`info`), os.Stdout)
	L := l.With(zap.String(`process`, `scriptrunner`))
	L.Info("Starting ...", zap.String(`Version`, buildTime), zap.String(`Commit`, commitHash))

	cwd, err := scriptrunner.GetCWD()
	if err != nil {
		L.Fatal("error retrieving cwd", zap.Error(err))
	}
	configPath := filepath.Join(cwd, configFile)
	if config != "" {
		configPath = config
	}

	scripts := filepath.Join(cwd, scriptsDir)
	workspace := filepath.Join(cwd, workspaceDir)
	certs := filepath.Join(cwd, certsDir)
	config, err := scriptrunner.GetConfig(configPath)
	switch {
	case err != nil:
		L.Error("error retrieving config", zap.Error(err))
	default:
		if config.ScriptsDir != "" {
			scripts = filepath.Join(cwd, config.ScriptsDir)
		}
		if config.WorkspaceDir != "" {
			workspace = filepath.Join(cwd, config.WorkspaceDir)
		}
		if config.CertsDir != "" {
			certs = filepath.Join(cwd, config.CertsDir)
		}
		if homeBaseURL == "" {
			homeBaseURL = config.HomeBase
		}
	}
	L.Info("scripts directory", zap.String("directory", scripts))
	L.Info("workspace directory", zap.String("directory", workspace))
	L.Info("certs directory", zap.String("directory", certs))

	for _, d := range []string{scripts, workspace, certs} {
		if err := scriptrunner.CreateDir(d); err != nil {
			L.Error("could not create directory", zap.String("directory", d))
		}
	}

	files, err := scriptrunner.GetArchiveFiles(scripts)
	if err != nil {
		L.Fatal("error retrieving scripts directory", zap.String("directory", scripts), zap.Error(err))
	}
	L.Info("script archives discovered", zap.Int("archives", len(files)), zap.Strings("scripts", files))

	pwsh := powershell.New(workspace)
	for _, f := range files {
		archive := filepath.Join(scripts, f)
		L.Info("processing archive", zap.String("archive", f))
		scriptrunner.UnZip(archive, workspace)

		scripts, err := scriptrunner.GetDirFiles(workspace)
		switch {
		case err != nil:
			L.Error("error list workspace", zap.Error(err))
		default:
			for _, script := range scripts {
				if !script.IsDir {
					L.Info("executing script", zap.String("script", script.FullPath))
					stdOut, stdErr, err := pwsh.Execute(script.FullPath)
					switch {
					case err != nil:
						var errMsg string
						if stdErr != "" {
							errMsg += stdErr + `; `
						}
						errMsg += err.Error()
						L.Error("error running script", zap.String("script", script.Name), zap.String(`error`, errMsg))
					case stdErr != "":
						if stdOut != "" {
							fmt.Println(stdOut)
						}
						L.Error("error running script", zap.String("script", script.Name), zap.String(`error`, stdErr))
					case stdOut != "":
						fmt.Println(stdOut)
					}
				}
			}
		}
		err = scriptrunner.CleanDirectory(workspace)
		if err != nil {
			L.Error("error cleaning workspace", zap.Error(err))
		}
	}

}
