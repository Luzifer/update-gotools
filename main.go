package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/Luzifer/go_helpers/str"
	"github.com/Luzifer/rconfig"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

type pkgCfg struct {
	Name    string `yaml:"name"`
	Single  bool   `yaml:"single"`
	Version string `yaml:"version"`
}

type configFile struct {
	Cwd          string     `yaml:"cwd"`
	GoPath       string     `yaml:"gopath"`
	Packages     []pkgCfg   `yaml:"packages"`
	PreCommands  [][]string `yaml:"pre_commands"`
	PostCommands [][]string `yaml:"post_commands"`
}

var (
	cfg = struct {
		Config         string `flag:"config,c" default:"~/.config/gotools.yml" description:"Configuration for update-gotools utility"`
		LogLevel       string `flag:"log-level" default:"info" description:"Log level (debug, info, warn, error, fatal)"`
		VersionAndExit bool   `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	version = "dev"

	cfgFile *configFile
)

func init() {
	var err error
	if err = rconfig.Parse(&cfg); err != nil {
		log.WithError(err).Fatal("Unable to parse commandline options")
	}

	if l, err := log.ParseLevel(cfg.LogLevel); err != nil {
		log.WithError(err).Fatal("Unable to parse log level")
	} else {
		log.SetLevel(l)
	}

	if cfg.VersionAndExit {
		fmt.Printf("update-gotools %s\n", version)
		os.Exit(0)
	}

	if cfg.Config, err = homedir.Expand(cfg.Config); err != nil {
		log.WithError(err).Fatal("Unable to expand config path")
	}
}

func main() {
	var err error

	cfgFile, err = defaultConfig()
	if err != nil {
		log.WithError(err).Fatal("Unable to create default config")
	}

	cfgFileRaw, err := ioutil.ReadFile(cfg.Config)
	if err != nil {
		log.WithError(err).Fatal("Unable to read config")
	}

	if err := yaml.Unmarshal(cfgFileRaw, cfgFile); err != nil {
		log.WithError(err).Fatal("Unable to parse config file")
	}

	log.WithFields(log.Fields{
		"version": version,
		"num_cpu": runtime.NumCPU(),
	}).Debugf("update-gotools started")

	runPreCommands()
	runPackageBuilds(runtime.NumCPU()-1, func(pkg pkgCfg) bool { return !pkg.Single })
	runPackageBuilds(1, func(pkg pkgCfg) bool { return pkg.Single })
	runPostCommands()

	log.Info("Installation successful")
}

func runPreCommands() {
	log.Info("Executing pre-commands")
	if err := executeCommands(log.WithFields(log.Fields{
		"step": "pre_commands",
	}), cfgFile.PreCommands); err != nil {
		log.WithError(err).Fatal("Pre-Command failed")
	}
}

func runPackageBuilds(n int, filter func(pkgCfg) bool) {
	limit := newLimiter(n)
	for _, pkg := range cfgFile.Packages {
		if !filter(pkg) {
			continue
		}

		limit.Add()
		go func(pkg pkgCfg) {
			logVer := pkg.Version
			if logVer == "" {
				logVer = "HEAD"
			}

			pkgLog := log.WithFields(log.Fields{
				"pkg": pkg.Name,
				"ver": logVer,
			})

			if err := executePackage(pkg, pkgLog); err != nil {
				pkgLog.WithError(err).Fatal("Failed to install package")
			}
			limit.Done()
		}(pkg)
	}
	limit.Wait()
}

func runPostCommands() {
	log.Info("Executing post-commands")
	if err := executeCommands(log.WithFields(log.Fields{
		"step": "post_commands",
	}), cfgFile.PostCommands); err != nil {
		log.WithError(err).Fatal("Post-Command failed")
	}
}

func defaultConfig() (*configFile, error) {
	h, err := homedir.Dir()
	if err != nil {
		return nil, err
	}

	return &configFile{
		Cwd:          h,
		Packages:     []pkgCfg{},
		PreCommands:  [][]string{},
		PostCommands: [][]string{},
	}, nil
}

func executeCommands(logEntry *log.Entry, commands [][]string) error {
	for i, cmdCfg := range commands {
		stderr := logEntry.WithFields(log.Fields{"cmd_id": i}).WriterLevel(log.ErrorLevel)
		stdout := logEntry.WithFields(log.Fields{"cmd_id": i}).WriterLevel(log.InfoLevel)
		defer stderr.Close()
		defer stdout.Close()

		if err := executeCommand(cmdCfg, stdout, stderr, cfgFile.Cwd); err != nil {
			return err
		}
	}

	return nil
}

func executeCommand(command []string, stdout, stderr io.Writer, cwd string) error {
	env := []string{
		strings.Join([]string{"GOPATH", cfgFile.GoPath}, "="),
		strings.Join([]string{"PATH", os.Getenv("PATH")}, "="),
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = cwd
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func executePackage(pkg pkgCfg, pkgLog *log.Entry) error {
	pkgLog.Info("Started package installation")

	stderr := pkgLog.WriterLevel(log.ErrorLevel)
	stdout := pkgLog.WriterLevel(log.InfoLevel)
	defer stderr.Close()
	defer stdout.Close()

	pkgLog.Debug("Fetching package using `go get`")
	if err := executeCommand([]string{"go", "get", "-d", pkg.Name}, stdout, stderr, cfgFile.Cwd); err != nil {
		return err
	}

	if !str.StringInSlice(pkg.Version, []string{"", "master"}) {
		pkgLog.Debug("Resetting to specified version")
		pkgPath := path.Join(os.Getenv("GOPATH"), "src", pkg.Name)

		// Fetch required references
		if err := executeCommand([]string{"git", "fetch", "-q", "--tags", "origin", pkg.Version}, stdout, stderr, pkgPath); err != nil {
			return err
		}

		// Do the real reset
		if err := executeCommand([]string{"git", "reset", "--hard", pkg.Version}, stdout, stderr, pkgPath); err != nil {
			return err
		}
	}

	pkgLog.Debug("Install package using `go install`")
	return executeCommand([]string{"go", "install", pkg.Name}, stdout, stderr, cfgFile.Cwd)
}
