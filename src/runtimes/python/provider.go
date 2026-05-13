// Package python implements the Python runtime provider for dtvem.
//
// This file holds the "shim half" of the provider: the methods invoked by the
// shim binary at runtime (Name, DisplayName, Shims, ExecutablePath, IsInstalled,
// InstallPath, ShouldReshimAfter, GetEnvironment) plus init() registration.
// The heavy install/list/migrate methods, along with their dependencies on
// HTTP, manifests, and archive extraction, live in provider_full.go behind a
// //go:build !shim tag so the shim binary never links them.
package python

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// Provider implements the runtime.Provider interface for Python.
type Provider struct{}

// NewProvider creates a new Python runtime provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the runtime name.
func (p *Provider) Name() string {
	return "python"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "Python"
}

// Shims returns the list of shim executables for Python.
func (p *Provider) Shims() []string {
	return []string{"python", "python3", "pip", "pip3"}
}

// ExecutablePath returns the path to the Python executable for a version.
func (p *Provider) ExecutablePath(version string) (string, error) {
	installPath, err := p.InstallPath(version)
	if err != nil {
		return "", err
	}

	var pythonPath string
	if goruntime.GOOS == constants.OSWindows {
		pythonPath = filepath.Join(installPath, "python.exe")
	} else {
		pythonPath = filepath.Join(installPath, "bin", "python")
	}

	if _, err := os.Stat(pythonPath); os.IsNotExist(err) {
		return "", fmt.Errorf("python executable not found at %s", pythonPath)
	}

	return pythonPath, nil
}

// IsInstalled checks if a version is installed.
func (p *Provider) IsInstalled(version string) (bool, error) {
	installPath := config.RuntimeVersionPath("python", version)
	_, err := os.Stat(installPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// InstallPath returns the installation directory for a version.
func (p *Provider) InstallPath(version string) (string, error) {
	return config.RuntimeVersionPath("python", version), nil
}

// ShouldReshimAfter returns true if the command installs or uninstalls packages.
func (p *Provider) ShouldReshimAfter(shimName string, args []string) bool {
	if shimName != "pip" && shimName != "pip3" {
		return false
	}

	if len(args) == 0 {
		return false
	}

	cmd := args[0]
	return cmd == "install" || cmd == "uninstall"
}

// GetEnvironment returns environment variables needed to run Python binaries.
// Python binaries from python-build-standalone are relocatable and don't require
// special environment setup.
func (p *Provider) GetEnvironment(_ string) (map[string]string, error) {
	return map[string]string{}, nil
}

// init registers the Python provider on package load.
func init() {
	if err := runtime.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register Python provider: %v", err))
	}
}
