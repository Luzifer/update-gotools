package main

import (
	"io"
	"os"
	"os/exec"
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/Luzifer/go_helpers/env"
	"github.com/Luzifer/go_helpers/str"
)

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
	e := env.ListToMap(os.Environ())
	e["GOPATH"] = cfgFile.GoPath

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = cwd
	cmd.Env = env.MapToList(e)
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

	ver, err := pkg.Version()
	if err != nil {
		return err
	}

	if !str.StringInSlice(ver, []string{"", "master"}) {
		pkgLog.Debug("Resetting to specified version")
		pkgPath := path.Join(os.Getenv("GOPATH"), "src", pkg.Name)

		// Fetch required references
		if err := executeCommand([]string{"git", "fetch", "-q", "--tags", "origin", ver}, stdout, stderr, pkgPath); err != nil {
			return err
		}

		// Do the real reset
		if err := executeCommand([]string{"git", "reset", "--hard", ver}, stdout, stderr, pkgPath); err != nil {
			return err
		}
	}

	pkgLog.Debug("Install package using `go install`")
	return executeCommand([]string{"go", "install", pkg.Name}, stdout, stderr, cfgFile.Cwd)
}
