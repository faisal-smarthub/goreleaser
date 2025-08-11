// Package makeself implements the Archive interface providing makeself self-extracting
// archive creation.
//
// This package provides integration with the makeself tool to create self-extracting
// archives that can be executed to automatically extract their contents and run
// installation scripts.
//
// Key Features:
// - Direct file creation: When possible, makeself creates archives directly
//   at the target path, eliminating intermediate copying and ensuring proper
//   executable permissions.
// - Dual binary support: Automatically detects and uses either 'makeself' or
//   'makeself.sh' commands.
// - Custom install scripts: Supports custom installation scripts or provides
//   a sensible default.
// - Template support: Supports custom output paths and advanced configuration
//   via NewWithOptions.
// - Cross-platform: Works with various makeself distributions and package managers.
//
// Example usage:
//   f, _ := os.Create("archive.run")
//   archive := makeself.New(f)
//   archive.Add(config.File{Source: "myapp", Destination: "myapp"})
//   archive.Close()
//   f.Close()
//
// The resulting file will be an executable self-extracting archive.
package makeself

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goreleaser/goreleaser/v2/pkg/config"
)

// Archive as makeself.
type Archive struct {
	tempDir    string
	files      map[string]bool
	target     io.Writer
	outputPath string // Path where makeself should create the archive directly
	closed     bool
	config     MakeselfConfig // Configuration options for makeself
}

// New makeself archive.
func New(target io.Writer) *Archive {
	tempDir, err := os.MkdirTemp("", "makeself-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp directory: %v", err))
	}

	var outputPath string
	// If target is a file, we can use its name directly with makeself
	if file, ok := target.(*os.File); ok {
		outputPath = file.Name()
	}

	return &Archive{
		tempDir:    tempDir,
		files:      map[string]bool{},
		target:     target,
		outputPath: outputPath,
		closed:     false,
	}
}

// NewWithInstallScript creates a makeself archive with a custom install script.
func NewWithInstallScript(target io.Writer, installScript string) *Archive {
	archive := New(target)
	
	// Write custom install script
	installPath := filepath.Join(archive.tempDir, "install.sh")
	if err := os.WriteFile(installPath, []byte(installScript), 0755); err != nil {
		panic(fmt.Sprintf("failed to create install script: %v", err))
	}
	
	return archive
}

// MakeselfConfig holds configuration options for makeself archives.
type MakeselfConfig struct {
	OutputPath        string   // Optional: override output path
	InstallScript     string   // Optional: custom install script content
	InstallScriptFile string   // Optional: path to custom install script file
	Label             string   // Optional: custom label for the archive
	NoCompression     bool     // Optional: disable compression (default: true for binaries)
	ExtraArgs         []string // Optional: extra command line arguments
	// LSM support
	LSMContent string // Optional: inline LSM content
	LSMFile    string // Optional: path to an LSM file
}

// NewWithOptions creates a makeself archive with advanced options (deprecated: use NewWithConfig).
type MakeselfOptions struct {
	OutputPath     string // Optional: override output path
	InstallScript  string // Optional: custom install script
	Label          string // Optional: custom label for the archive
	NoCompression  bool   // Optional: disable compression (default: true for binaries)
}

// NewWithOptions creates a makeself archive with custom options.
func NewWithOptions(target io.Writer, opts MakeselfOptions) *Archive {
	archive := New(target)
	
	// Override output path if provided
	if opts.OutputPath != "" {
		archive.outputPath = opts.OutputPath
	}
	
	// Write custom install script if provided
	if opts.InstallScript != "" {
		installPath := filepath.Join(archive.tempDir, "install.sh")
		if err := os.WriteFile(installPath, []byte(opts.InstallScript), 0755); err != nil {
			panic(fmt.Sprintf("failed to create install script: %v", err))
		}
	}
	
	return archive
}

// NewWithConfig creates a makeself archive with full configuration options.
func NewWithConfig(target io.Writer, outputPath string, cfg MakeselfConfig) *Archive {
	archive := New(target)

	// Override output path if provided
	if cfg.OutputPath != "" {
		archive.outputPath = cfg.OutputPath
	} else if outputPath != "" {
		archive.outputPath = outputPath
	}

	// Handle install script - file takes precedence over content
	if cfg.InstallScriptFile != "" {
		// Copy script file to temp directory
		scriptContent, err := os.ReadFile(cfg.InstallScriptFile)
		if err != nil {
			panic(fmt.Sprintf("failed to read install script file %s: %v", cfg.InstallScriptFile, err))
		}
		installPath := filepath.Join(archive.tempDir, "install.sh")
		if err := os.WriteFile(installPath, scriptContent, 0755); err != nil {
			panic(fmt.Sprintf("failed to create install script: %v", err))
		}
	} else if cfg.InstallScript != "" {
		// Use provided script content
		installPath := filepath.Join(archive.tempDir, "install.sh")
		if err := os.WriteFile(installPath, []byte(cfg.InstallScript), 0755); err != nil {
			panic(fmt.Sprintf("failed to create install script: %v", err))
		}
	}

	// Store configuration for use in Close()
	archive.config = cfg

	return archive
}

// Close creates the makeself archive and writes it to the target.
func (a *Archive) Close() error {
	if a.closed {
		return nil // Idempotent close
	}
	a.closed = true
	
	defer os.RemoveAll(a.tempDir)

	// Check if makeself command is available
	makeselfCmd := findMakeselfCommand()
	if makeselfCmd == "" {
		return fmt.Errorf("makeself command not found in PATH (tried 'makeself' and 'makeself.sh')")
	}

	// Create a basic install script if none exists
	installScript := filepath.Join(a.tempDir, "install.sh")
	if _, err := os.Stat(installScript); os.IsNotExist(err) {
		installContent := `#!/bin/bash
# Default installation script for makeself archive
# This script is executed after extraction

# Make binaries executable
find . -type f -perm -u+x -exec chmod +x {} \;

echo "Archive extracted successfully to $(pwd)"
echo "Files:"
find . -type f | sort
`
		if err := os.WriteFile(installScript, []byte(installContent), 0755); err != nil {
			return fmt.Errorf("failed to create install script: %w", err)
		}
	}

	// Determine output path for makeself
	var outputPath string
	var needsCopy bool = false
	if a.outputPath != "" {
		// Direct file output - makeself creates the file directly
		outputPath = a.outputPath
		
		// Truncate the target file to prepare for makeself to write to it
		// but don't close it yet, as the caller might need to close it
		if file, ok := a.target.(*os.File); ok {
			if err := file.Truncate(0); err != nil {
				return fmt.Errorf("failed to truncate target file: %w", err)
			}
			if _, err := file.Seek(0, 0); err != nil {
				return fmt.Errorf("failed to seek to beginning of file: %w", err)
			}
			// Note: We don't close the file here, let the caller handle it
		}
	} else {
		// Fallback to temp file approach for non-file targets
		tempFile, err := os.CreateTemp("", "makeself-output-*")
		if err != nil {
			return fmt.Errorf("failed to create temp output file: %w", err)
		}
		defer func() {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}()
		outputPath = tempFile.Name()
		needsCopy = true
	}

// Build makeself command with configuration
	args := []string{"--quiet"} // Always run quietly
	
	// Apply compression setting
	if a.config.NoCompression {
		args = append(args, "--nocomp")
	} else {
		// Default to no compression for consistency with existing behavior
		args = append(args, "--nocomp")
	}
	
	// Handle LSM: write or copy into temp and pass --lsm
	if a.config.LSMContent != "" || a.config.LSMFile != "" {
		lsmPath := filepath.Join(a.tempDir, "archive.lsm")
		if a.config.LSMContent != "" {
			if err := os.WriteFile(lsmPath, []byte(a.config.LSMContent), 0644); err != nil {
				return fmt.Errorf("failed to write LSM content: %w", err)
			}
		} else {
			content, err := os.ReadFile(a.config.LSMFile)
			if err != nil {
				return fmt.Errorf("failed to read LSM file %s: %w", a.config.LSMFile, err)
			}
			if err := os.WriteFile(lsmPath, content, 0644); err != nil {
				return fmt.Errorf("failed to stage LSM file: %w", err)
			}
		}
		args = append(args, "--lsm", lsmPath)
	}
	
	// Add any extra arguments from configuration
	args = append(args, a.config.ExtraArgs...)
	
	// Add required positional arguments
	args = append(args, a.tempDir) // Source directory
	args = append(args, outputPath) // Output file
	
	// Use custom label or default
	label := "Self-extracting archive"
	if a.config.Label != "" {
		label = a.config.Label
	}
	args = append(args, label)
	
	// Always use ./install.sh as the startup script
	args = append(args, "./install.sh")
	
	// Create the makeself archive command
	cmd := exec.Command(makeselfCmd, args...)

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", makeselfCmd, err, stderr.String())
	}

	// If we used direct file output, we're done (no copying needed)
	if !needsCopy {
		return nil
	}

	// Fallback: copy from temp file to target writer (for non-file targets)
	tempFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("failed to open temp output file: %w", err)
	}
	defer tempFile.Close()

	if _, err := io.Copy(a.target, tempFile); err != nil {
		return fmt.Errorf("failed to copy makeself archive: %w", err)
	}

	// If the target is a file, make it executable like makeself normally does
	if file, ok := a.target.(*os.File); ok {
		if info, err := file.Stat(); err == nil {
			// Add executable permission for user, group, and other
			newMode := info.Mode() | 0111
			if err := file.Chmod(newMode); err != nil {
				// Don't fail if we can't set permissions - just log it
				// This is because the archive content is more important than permissions
				fmt.Fprintf(os.Stderr, "Warning: failed to make makeself archive executable: %v\n", err)
			}
		}
	}

	return nil
}

// findMakeselfCommand finds the makeself command in PATH, trying both 'makeself' and 'makeself.sh'
func findMakeselfCommand() string {
	// Try 'makeself' first (common on some distributions)
	if _, err := exec.LookPath("makeself"); err == nil {
		return "makeself"
	}
	// Try 'makeself.sh' (traditional name)
	if _, err := exec.LookPath("makeself.sh"); err == nil {
		return "makeself.sh"
	}
	// Not found
	return ""
}

// Add file to the archive.
func (a *Archive) Add(f config.File) error {
	if a.closed {
		return fmt.Errorf("cannot add files to closed archive")
	}
	if _, ok := a.files[f.Destination]; ok {
		return fmt.Errorf("file %s already exists in archive", f.Destination)
	}

	destPath := filepath.Join(a.tempDir, f.Destination)
	
	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(destPath), err)
	}

	// Copy file to temp directory
	src, err := os.Open(f.Source)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", f.Source, err)
	}
	defer src.Close()

	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file %s: %w", f.Source, err)
	}

	if srcInfo.IsDir() {
		return fmt.Errorf("directories are not supported in makeself archives: %s", f.Source)
	}

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file %s to %s: %w", f.Source, destPath, err)
	}

	// Set file permissions
	mode := srcInfo.Mode()
	if f.Info.Mode != 0 {
		mode = f.Info.Mode
	}
	if err := os.Chmod(destPath, mode); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", destPath, err)
	}

	// Set ownership if specified
	if f.Info.Owner != "" || f.Info.Group != "" {
		// Note: Setting ownership requires root privileges in most cases
		// We'll skip this for now but could be implemented with proper privileges
	}

	// Set modification time
	if !f.Info.ParsedMTime.IsZero() {
		if err := os.Chtimes(destPath, f.Info.ParsedMTime, f.Info.ParsedMTime); err != nil {
			return fmt.Errorf("failed to set modification time on %s: %w", destPath, err)
		}
	}

	a.files[f.Destination] = true
	return nil
}

// CheckMakeselfAvailable checks if makeself is available in the system.
func CheckMakeselfAvailable() error {
	makeselfCmd := findMakeselfCommand()
	if makeselfCmd == "" {
		return fmt.Errorf("makeself command not found in PATH (tried 'makeself' and 'makeself.sh'). Please install makeself package")
	}
	return nil
}

// GetMakeselfVersion returns the version of makeself if available.
func GetMakeselfVersion() (string, error) {
	makeselfCmd := findMakeselfCommand()
	if makeselfCmd == "" {
		return "", fmt.Errorf("makeself command not found in PATH (tried 'makeself' and 'makeself.sh')")
	}
	cmd := exec.Command(makeselfCmd, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get makeself version: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Copy is not supported for makeself archives.
func Copy(src io.Reader, dest io.Writer) (*Archive, error) {
	return nil, fmt.Errorf("copying makeself archives is not supported")
}
