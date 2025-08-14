// Package archive provides tar.gz and zip archiving
package archive

import (
	"fmt"
	"io"
	"os"

	"github.com/goreleaser/goreleaser/v2/pkg/archive/gzip"
	"github.com/goreleaser/goreleaser/v2/pkg/archive/makeself"
	"github.com/goreleaser/goreleaser/v2/pkg/archive/tar"
	"github.com/goreleaser/goreleaser/v2/pkg/archive/targz"
	"github.com/goreleaser/goreleaser/v2/pkg/archive/tarxz"
	"github.com/goreleaser/goreleaser/v2/pkg/archive/tarzst"
	"github.com/goreleaser/goreleaser/v2/pkg/archive/zip"
	"github.com/goreleaser/goreleaser/v2/pkg/config"
)

// MakeselfOption represents a configuration option for makeself archives.
type MakeselfOption func(*makeself.MakeselfConfig)

// WithMakeselfLabel sets a custom label for the makeself archive.
func WithMakeselfLabel(label string) MakeselfOption {
	return func(cfg *makeself.MakeselfConfig) {
		cfg.Label = label
	}
}

// WithMakeselfScript sets a custom installation script content.
func WithMakeselfScript(script string) MakeselfOption {
	return func(cfg *makeself.MakeselfConfig) {
		cfg.InstallScript = script
	}
}

// WithMakeselfScriptFile sets a path to a custom installation script file.
func WithMakeselfScriptFile(scriptFile string) MakeselfOption {
	return func(cfg *makeself.MakeselfConfig) {
		cfg.InstallScriptFile = scriptFile
	}
}


// WithMakeselfCompression sets the compression format for the makeself archive.
// Supported formats: gzip, bzip2, xz, lzo, compress, none
func WithMakeselfCompression(format string) MakeselfOption {
	return func(cfg *makeself.MakeselfConfig) {
		cfg.Compression = format
	}
}

// WithMakeselfExtraArgs adds extra command line arguments to the makeself command.
func WithMakeselfExtraArgs(args ...string) MakeselfOption {
	return func(cfg *makeself.MakeselfConfig) {
		cfg.ExtraArgs = append(cfg.ExtraArgs, args...)
	}
}

// WithMakeselfLSMTemplate sets an inline LSM content template to be used.
func WithMakeselfLSMTemplate(content string) MakeselfOption {
	return func(cfg *makeself.MakeselfConfig) {
		cfg.LSMContent = content
	}
}

// WithMakeselfLSMFile sets a path to an LSM file to be used.
func WithMakeselfLSMFile(path string) MakeselfOption {
	return func(cfg *makeself.MakeselfConfig) {
		cfg.LSMFile = path
	}
}

// Archive represents a compression archive files from disk can be written to.
type Archive interface {
	Close() error
	Add(f config.File) error
}

// New archive.
func New(w io.Writer, format string) (Archive, error) {
	switch format {
	case "tar.gz", "tgz":
		return targz.New(w), nil
	case "tar":
		return tar.New(w), nil
	case "gz":
		return gzip.New(w), nil
	case "tar.xz", "txz":
		return tarxz.New(w), nil
	case "tar.zst", "tzst":
		return tarzst.New(w), nil
	case "zip":
		return zip.New(w), nil
	case "makeself":
		return makeself.New(w), nil
	}
	return nil, fmt.Errorf("invalid archive format: %s", format)
}

// NewWithOptions creates a new archive with advanced configuration options.
// This is currently only supported for makeself format.
func NewWithOptions(w io.Writer, format, outputPath string, options ...MakeselfOption) (Archive, error) {
	switch format {
	case "makeself":
		cfg := &makeself.MakeselfConfig{}
		for _, opt := range options {
			opt(cfg)
		}
		return makeself.NewWithConfig(w, outputPath, *cfg), nil
	default:
		// For non-makeself formats, ignore options and use regular New
		return New(w, format)
	}
}

// Copy copies the source archive into a new one, which can be appended at.
// Source needs to be in the specified format.
func Copy(r *os.File, w io.Writer, format string) (Archive, error) {
	switch format {
	case "tar.gz", "tgz":
		return targz.Copy(r, w)
	case "tar":
		return tar.Copy(r, w)
	case "zip":
		return zip.Copy(r, w)
	case "makeself":
		return makeself.Copy(r, w)
	}
	return nil, fmt.Errorf("invalid archive format: %s", format)
}
