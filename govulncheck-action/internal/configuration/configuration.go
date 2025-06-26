package configuration

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Configuration struct {
	IgnoredVulnerabilities []*Vulnerability `yaml:"ignored-vulnerabilities"`
}

type Vulnerability struct {
	ID           string    `yaml:"id"`
	SilenceUntil time.Time `yaml:"silence-until"`
	Info         string    `yaml:"info"`
}

func New(path string) (Configuration, error) {
	c := Configuration{}
	if path == "" {
		return c, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = yaml.Unmarshal(contents, &c)
	return c, err
}
