package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// validateDirectory sets execution directory and the optional config file.
// It checks for a config file in cwd (or directory provided in project flag)
// or any of the parent directory. If a config file is found, ExecutionDirectory
// and ConfigFile is set.
// If in the current directory or any parent directory (upto filesystem root)
// no configuration file exists, ExecutionDirectory is set to current working
// directory (or directory provided in project flag) and ConfigFile is an empty string.
func (ec *ExecutionContext) setExecutionDirectory() error {
	if len(ec.ConfigFile) != 0 {
		err := ec.validateConfigFile()
		if err != nil {
			return err
		}
	}

	// when project flag is not set, set cwd as execution directory
	if len(ec.ExecutionDirectory) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "error getting current working directory")
		}
		ec.ExecutionDirectory = cwd
	}

	err := validateExecutionDirectory(ec.ExecutionDirectory)
	if err != nil {
		return err
	}
	ec.ExecutionDirectory, _ = filepath.Abs(ec.ExecutionDirectory)

	// check for configuration file if not already set
	if len(ec.ConfigFile) == 0 {
		dir, err := recursivelyFindConfig(ec.ExecutionDirectory)
		if err != nil {
			ec.Logger.Debug("execution directory: ", ec.ExecutionDirectory)
			ec.Logger.Debug("No config file found")
			return nil
		}
		ec.ExecutionDirectory = dir
		ec.ConfigFile = filepath.Join(ec.ExecutionDirectory, "config.yaml")
	}

	ec.Logger.Debug("execution directory: ", ec.ExecutionDirectory)
	ec.Logger.Debug("config file: ", ec.ConfigFile)
	return nil
}

// filesRequiredV1 are the files that are mandatory to qualify for a project
// directory with config V1.
var filesRequiredV1 = []string{
	"config.yaml",
	"migrations",
}

// ConfigFile are the configuration files to look for inn a project directory
var ConfigFile = []string{
	"config.yaml",
}

// validateConfigFile sets config file path and validates it's version.
// if config-file flag is set with an abs path and project flag is not set,
// set execution directory to the parent directory of config file.
// if config-file flag is not set with an abs path and porject flag is set,
// join path to set config file path. If project flag not set, set config
// file relative to cwd.
func (ec *ExecutionContext) validateConfigFile() error {
	if filepath.IsAbs(ec.ConfigFile) {
		if len(ec.ExecutionDirectory) == 0 {
			ec.ExecutionDirectory = filepath.Dir(ec.ConfigFile)
		}
	} else {
		if len(ec.ExecutionDirectory) != 0 {
			ec.ConfigFile = filepath.Join(ec.ExecutionDirectory, ec.ConfigFile)
		} else {
			ec.ExecutionDirectory = filepath.Dir(ec.ConfigFile)
		}
		ec.ConfigFile, _ = filepath.Abs(ec.ConfigFile)
	}
	return nil
}

// validateExecutionDirectory checks if execution directory exists, is a directory
// and is not file system root
func validateExecutionDirectory(dir string) error {
	ed, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(err, "did not find required directory. use 'init'?")
		}
		return errors.Wrap(err, "error getting directory details")
	}
	if !ed.IsDir() {
		return errors.Errorf("'%s' is not a directory", ed.Name())
	}
	// check if execution directory is file system root
	if err = CheckFilesystemBoundary(dir); err != nil {
		return errors.Wrap(err, "invalid execution directory")
	}
	return nil
}

// recursivelyFindConfig tries to parse 'startFrom' as a project
// directory by checking for the 'configFile'. If the parent of 'startFrom'
// (nextDir) is filesystem root, error is returned. Otherwise, 'nextDir' is
// validated, recursively.
func recursivelyFindConfig(startFrom string) (validDir string, err error) {
	err = CheckDirectoryForFiles(startFrom, ConfigFile)
	if err != nil {
		nextDir := filepath.Dir(startFrom)
		if err := CheckFilesystemBoundary(nextDir); err != nil {
			return nextDir, errors.Wrapf(err, "cannot find [%s] | search stopped", strings.Join(ConfigFile, ", "))
		}
		return recursivelyFindConfig(nextDir)
	}
	return startFrom, nil
}

// CheckDirectoryForFiles tries to parse dir for the 'filesToCheck' and returns error
// if any one of them is missing.
func CheckDirectoryForFiles(dir string, filesToCheck []string) error {
	notFound := []string{}
	for _, f := range filesToCheck {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			relpath, e := filepath.Rel(dir, f)
			if e == nil {
				f = relpath
			}
			notFound = append(notFound, f)
		}
	}
	if len(notFound) > 0 {
		return errors.Errorf("cannot validate directory '%s': [%s] not found", dir, strings.Join(notFound, ", "))
	}
	return nil
}

// CheckFilesystemBoundary returns an error if dir is filesystem root
func CheckFilesystemBoundary(dir string) error {
	cleaned := filepath.Clean(dir)
	isWindowsRoot, _ := regexp.MatchString(`^[a-zA-Z]:\\$`, cleaned)
	// return error if filesystem boundary is hit
	if cleaned == "/" || isWindowsRoot {
		return errors.Errorf("filesystem boundary hit")

	}
	return nil
}

// validateV1Directory checks version of config file. If version is V1, the execution
// directory is strictly validated.
// A valid V1 project directory contains the following:
// 1. migrations directory
// 2. config.yaml file
// 3. metadata.yaml (optional)
func (ec *ExecutionContext) validateConfigV1() error {
	// Strict directory check for version 1 config.
	if ec.Config.Version == V1 {
		if err := CheckDirectoryForFiles(ec.ExecutionDirectory, filesRequiredV1); err != nil {
			return err
		}
	}
	return nil
}
