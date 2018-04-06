package main

import (
	"io/ioutil"
	"net/http"
	"os"

	homedir "github.com/mitchellh/go-homedir"

	yaml "gopkg.in/yaml.v2"
)

type pkgCfg struct {
	Name       string `yaml:"name"`
	Single     bool   `yaml:"single"`
	Ver        string `yaml:"version"`
	VersionURL string `yaml:"version_url"`
}

type configFile struct {
	Cwd          string     `yaml:"cwd"`
	GoPath       string     `yaml:"gopath"`
	Packages     []pkgCfg   `yaml:"packages"`
	PreCommands  [][]string `yaml:"pre_commands"`
	PostCommands [][]string `yaml:"post_commands"`
}

func (c *configFile) LoadFromPath(filepath string) error {
	r, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := yaml.NewDecoder(r).Decode(cfgFile); err != nil {
		return err
	}

	if c.Cwd, err = homedir.Expand(c.Cwd); err != nil {
		return err
	}

	if c.GoPath, err = homedir.Expand(c.GoPath); err != nil {
		return err
	}

	return nil
}

func (p *pkgCfg) Version() (string, error) {
	if p.Ver != "" {
		return p.Ver, nil
	}

	if p.VersionURL != "" {
		resp, err := http.Get(p.VersionURL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		v, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		p.Ver = string(v)
		return p.Ver, nil
	}

	return "", nil
}
