// Package ruby implements the Ruby runtime provider for dtvem.
//
// This file holds the "shim half" of the provider: the methods invoked by the
// shim binary at runtime (Name, DisplayName, Shims, ExecutablePath, IsInstalled,
// InstallPath, ShouldReshimAfter, GetEnvironment) plus init() registration.
// The heavy install/list/migrate methods, along with their dependencies on
// HTTP, manifests, and archive extraction, live in provider_full.go behind a
// //go:build !shim tag so the shim binary never links them.
package ruby

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// Provider implements the runtime.Provider interface for Ruby.
type Provider struct{}

// NewProvider creates a new Ruby runtime provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the runtime name.
func (p *Provider) Name() string {
	return "ruby"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "Ruby"
}

// Shims returns the list of shim executables for Ruby.
func (p *Provider) Shims() []string {
	return []string{"ruby", "gem", "irb", "bundle", "rake", "rdoc", "ri"}
}

// ExecutablePath returns the path to the Ruby executable for a version.
func (p *Provider) ExecutablePath(version string) (string, error) {
	installPath, err := p.InstallPath(version)
	if err != nil {
		return "", err
	}

	var rubyPath string
	if goruntime.GOOS == constants.OSWindows {
		rubyPath = filepath.Join(installPath, "bin", "ruby.exe")
	} else {
		rubyPath = filepath.Join(installPath, "bin", "ruby")
	}

	if _, err := os.Stat(rubyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("ruby executable not found at %s", rubyPath)
	}

	return rubyPath, nil
}

// IsInstalled checks if a version is installed.
func (p *Provider) IsInstalled(version string) (bool, error) {
	installPath := config.RuntimeVersionPath("ruby", version)
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
	return config.RuntimeVersionPath("ruby", version), nil
}

// ShouldReshimAfter returns true if the command installs or uninstalls gems.
func (p *Provider) ShouldReshimAfter(shimName string, args []string) bool {
	if shimName == "gem" {
		if len(args) == 0 {
			return false
		}
		cmd := args[0]
		return cmd == "install" || cmd == "uninstall"
	}

	if shimName == "bundle" {
		if len(args) == 0 {
			return false
		}
		cmd := args[0]
		return cmd == "install" || cmd == "update"
	}

	return false
}

// GetEnvironment returns environment variables needed to run Ruby binaries.
// On Unix systems, Ruby from ruby-builder needs LD_LIBRARY_PATH (Linux) or
// DYLD_LIBRARY_PATH (macOS) set to find libruby.so.
func (p *Provider) GetEnvironment(version string) (map[string]string, error) {
	if goruntime.GOOS == constants.OSWindows {
		return map[string]string{}, nil
	}

	installPath, err := p.InstallPath(version)
	if err != nil {
		return nil, err
	}

	libPath := filepath.Join(installPath, "lib")

	env := make(map[string]string)

	if goruntime.GOOS == constants.OSDarwin {
		env["DYLD_LIBRARY_PATH"] = libPath
	} else {
		env["LD_LIBRARY_PATH"] = libPath
	}

	return env, nil
}

// init registers the Ruby provider on package load.
func init() {
	if err := runtime.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register Ruby provider: %v", err))
	}
}
