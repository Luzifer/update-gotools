package main

import (
	"io/ioutil"
	"net/http"
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
