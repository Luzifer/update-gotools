package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Luzifer/rconfig"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

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
	if err := rconfig.Parse(&cfg); err != nil {
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

	var err error
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

	if err := cfgFile.LoadFromPath(cfg.Config); err != nil {
		log.WithError(err).Fatal("Unable to parse config file")
	}

	log.WithFields(log.Fields{
		"version": version,
		"num_cpu": runtime.NumCPU(),
	}).Debugf("update-gotools started")

	parallelRunners := runtime.NumCPU() - 1
	if parallelRunners < 1 {
		parallelRunners = 1
	}

	runPreCommands()
	runPackageBuilds(parallelRunners, func(pkg pkgCfg) bool { return !pkg.Single })
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
			logVer, err := pkg.Version()
			if err != nil {
				log.WithFields(log.Fields{
					"pkg": pkg.Name,
				}).WithError(err).Fatal("Unable to fetch version information")
			}

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
