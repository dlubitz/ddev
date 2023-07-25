package ddevapp

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/ddev/ddev/pkg/archive"
	"github.com/ddev/ddev/pkg/fileutil"
	"github.com/ddev/ddev/pkg/nodeps"
	"github.com/ddev/ddev/pkg/output"
	"github.com/ddev/ddev/pkg/util"
)

// createNeosFlowSettingsFile creates the app's LocalConfiguration.php and
// AdditionalConfiguration.php, adding things like database host, name, and
// password. Returns the fullpath to settings file and error
func createNeosFlowSettingsFile(app *DdevApp) (string, error) {
	if filepath.Dir(app.SiteDdevSettingsFile) == app.AppRoot {
		// As long as the final settings folder is not defined, early return
		return app.SiteDdevSettingsFile, nil
	}

	if !isNeosFlowApp(app) {
		util.Warning("Neos Flow does not seem to have been set up yet, missing .flow")
	}

	// Neos/Flow ddev settings file will be AdditionalConfiguration.php (app.SiteDdevSettingsFile).
	// Check if the file already exists.
	if fileutil.FileExists(app.SiteDdevSettingsFile) {
		// Check if the file is managed by ddev.
		signatureFound, err := fileutil.FgrepStringInFile(app.SiteDdevSettingsFile, nodeps.DdevFileSignature)
		if err != nil {
			return "", err
		}

		// If the signature wasn't found, warn the user and return.
		if !signatureFound {
			util.Warning("%s already exists and is managed by the user.", filepath.Base(app.SiteDdevSettingsFile))
			return app.SiteDdevSettingsFile, nil
		}
	}

	output.UserOut.Printf("Generating %s file for database connection.", filepath.Base(app.SiteDdevSettingsFile))
	if err := writeNeosFlowSettingsFile(app); err != nil {
		return "", fmt.Errorf("failed to write Neos Flow Settings.ddev.yaml file: %v", err.Error())
	}

	return app.SiteDdevSettingsFile, nil
}

// writeNeosFlowSettingsFile produces Settings.ddev.yaml file
// It's assumed that the LocalConfiguration.php already exists, and we're
// overriding the db config values in it.
func writeNeosFlowSettingsFile(app *DdevApp) error {
	filePath := app.SiteDdevSettingsFile

	// Ensure target directory is writable.
	dir := filepath.Dir(filePath)
	var perms os.FileMode = 0755
	if err := os.Chmod(dir, perms); err != nil {
		if !os.IsNotExist(err) {
			// The directory exists, but chmod failed.
			return err
		}

		// The directory doesn't exist, create it with the appropriate permissions.
		if err := os.MkdirAll(dir, perms); err != nil {
			return err
		}
	}
	dbDriver := "mysqli" // mysqli is the driver used in default LocalConfiguration.php
	if app.Database.Type == nodeps.Postgres {
		dbDriver = "pdo_pgsql"
	}
	settings := map[string]interface{}{"DBHostname": "db", "DBDriver": dbDriver, "DBPort": GetExposedPort(app, "db")}

	// Ensure target directory exists and is writable
	if err := os.Chmod(dir, 0755); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}

	t, err := template.New("Settings.ddev.yaml").ParseFS(bundledAssets, "neosFlow/Settings.ddev.yaml")
	if err != nil {
		return err
	}

	if err = t.Execute(f, settings); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}

func setNeosFlowSiteSettingsPaths(app *DdevApp) {
	settingsFileBasePath := filepath.Join(app.AppRoot, app.ComposerRoot)
	var localSettingsFilePath string

	if isNeosFlowApp(app) {
		localSettingsFilePath = filepath.Join(settingsFileBasePath, "Configuration", "Development", "Ddev", "Settings.ddev.yaml")
	} else {
		// As long as Neos/Flow is not installed, the file paths are set to the
		// AppRoot to avoid the creation of the .gitignore in the wrong location.
		localSettingsFilePath = filepath.Join(settingsFileBasePath, "Settings.ddev.yaml")
	}

	// Update file paths
	app.SiteDdevSettingsFile = localSettingsFilePath
}

// neosFlowImportFilesAction defines the Neos/Flow workflow for importing project files.
// The NeosFlow import-files workflow is currently identical to the Drupal workflow.
func neosFlowImportFilesAction(app *DdevApp, uploadDir, importPath, extPath string) error {
	destPath := app.calculateHostUploadDirFullPath(uploadDir)

	// parent of destination dir should exist
	if !fileutil.FileExists(filepath.Dir(destPath)) {
		return fmt.Errorf("unable to import to %s: parent directory does not exist", destPath)
	}

	// parent of destination dir should be writable.
	if err := os.Chmod(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// If the destination path exists, remove it as was warned
	if fileutil.FileExists(destPath) {
		if err := os.RemoveAll(destPath); err != nil {
			return fmt.Errorf("failed to cleanup %s before import: %v", destPath, err)
		}
	}

	if isTar(importPath) {
		if err := archive.Untar(importPath, destPath, extPath); err != nil {
			return fmt.Errorf("failed to extract provided archive: %v", err)
		}

		return nil
	}

	if isZip(importPath) {
		if err := archive.Unzip(importPath, destPath, extPath); err != nil {
			return fmt.Errorf("failed to extract provided archive: %v", err)
		}

		return nil
	}

	//nolint: revive
	if err := fileutil.CopyDir(importPath, destPath); err != nil {
		return err
	}

	return nil
}

// isNeosFlowApp returns true if the app is of type neos-flow
func isNeosFlowApp(app *DdevApp) bool {
	neosFlowExecuteable := filepath.Join(app.AppRoot, app.ComposerRoot, "flow")

	// Check if the folder exists, fails if a symlink target does not exist.
	if _, err := os.Stat(neosFlowExecuteable); !os.IsNotExist(err) {
		return true
	}

	return false
}

func neosFlowConfigOverrideAction(app *DdevApp) error {

	if app.Docroot == "" {
		// set default to "Web"
		app.Docroot = "Web"
	}

	app.UploadDirs = append(app.UploadDirs, "../Data/Persistent")

	app.WebEnvironment = append(app.WebEnvironment, "FLOW_CONTEXT=Development/Ddev")
	app.WebEnvironment = append(app.WebEnvironment, "FLOW_PATH_TEMPORARY_BASE=/tmp/Flow")
	app.WebEnvironment = append(app.WebEnvironment, "FLOW_REWRITEURLS=1")

	return nil;
}
