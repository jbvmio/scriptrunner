package scriptrunner

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// Config defines configuration options.
type Config struct {
	HomeBase     string `yaml:"homeBase"`
	ScriptsDir   string `yaml:"scriptsDir"`
	WorkspaceDir string `yaml:"workspaceDir"`
	CertsDir     string `yaml:"certDir"`
}

// GetConfig creates and returns a Config from the given filepath.
func GetConfig(path string) (*Config, error) {
	var C Config
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return &C, err
	}
	err = yaml.Unmarshal(b, &C)
	return &C, err
}
