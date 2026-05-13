// Package node implements the Node.js runtime provider for dtvem.
//
// This file holds the "shim half" of the provider: the methods invoked by the
// shim binary at runtime (Name, DisplayName, Shims, ExecutablePath, IsInstalled,
// InstallPath, ShouldReshimAfter, GetEnvironment) plus init() registration.
// The heavy install/list/migrate methods, along with their dependencies on
// HTTP, manifests, and archive extraction, live in provider_full.go behind a
// //go:build !shim tag so the shim binary never links them.
package node

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// Provider implements the runtime.Provider interface for Node.js.
type Provider struct{}

// NewProvider creates a new Node.js runtime provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the runtime name.
func (p *Provider) Name() string {
	return "node"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "Node.js"
}

// Shims returns the list of shim executables for Node.js.
func (p *Provider) Shims() []string {
	return []string{"node", "npm", "npx"}
}

// ExecutablePath returns the path to the Node.js executable for a version.
func (p *Provider) ExecutablePath(version string) (string, error) {
	installPath, err := p.InstallPath(version)
	if err != nil {
		return "", err
	}

	var nodePath string
	if goruntime.GOOS == constants.OSWindows {
		nodePath = filepath.Join(installPath, "node.exe")
	} else {
		nodePath = filepath.Join(installPath, "bin", "node")
	}

	if _, err := os.Stat(nodePath); os.IsNotExist(err) {
		return "", fmt.Errorf("node executable not found at %s", nodePath)
	}

	return nodePath, nil
}

// IsInstalled checks if a version is installed.
func (p *Provider) IsInstalled(version string) (bool, error) {
	installPath := config.RuntimeVersionPath("node", version)
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
	return config.RuntimeVersionPath("node", version), nil
}

// ShouldReshimAfter returns true if the command installs or uninstalls global
// packages that add/remove executables.
func (p *Provider) ShouldReshimAfter(shimName string, args []string) bool {
	if shimName != "npm" {
		return false
	}

	if len(args) == 0 {
		return false
	}

	cmd := args[0]
	isPackageCommand := cmd == "install" || cmd == "i" ||
		cmd == "uninstall" || cmd == "remove" || cmd == "rm" || cmd == "un"

	if !isPackageCommand {
		return false
	}

	for _, arg := range args {
		if arg == "-g" || arg == "--global" {
			return true
		}
	}

	return false
}

// GetEnvironment returns environment variables needed to run Node.js binaries.
// Node.js binaries are self-contained and don't require special environment setup.
func (p *Provider) GetEnvironment(_ string) (map[string]string, error) {
	return map[string]string{}, nil
}

// init registers the Node.js provider on package load.
func init() {
	if err := runtime.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register Node.js provider: %v", err))
	}
}
