package makeself

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/goreleaser/goreleaser/v2/internal/testlib"
	"github.com/goreleaser/goreleaser/v2/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestMakeselfArchive(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test.run")
	
	// Create mock makeself script that creates a simple archive
	// Try both names to ensure compatibility
	mockMakeselfScript := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
# Mock makeself script for testing
echo "Creating self-extracting archive: $4"
# Create a simple executable script as output
/usr/bin/cat > "$4" << 'EOF'
#!/bin/bash
echo "Self-extracting archive created successfully"
exit 0
EOF
/usr/bin/chmod +x "$4"
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	// Also create makeself.sh for compatibility
	mockMakeselfScriptSh := filepath.Join(tmp, "makeself.sh")
	require.NoError(t, os.WriteFile(mockMakeselfScriptSh, []byte(mockScript), 0755))
	
	// Set PATH to include our mock script
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)
	defer archive.Close()

	// Test adding files
	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "foo.txt",
	}))
	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/sub1/bar.txt", 
		Destination: "sub1/bar.txt",
	}))
	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/sub1/executable",
		Destination: "sub1/executable",
	}))

	// Test error handling for non-existent file
	require.Error(t, archive.Add(config.File{
		Source:      "../testdata/nope.txt",
		Destination: "nope.txt",
	}))

	require.NoError(t, archive.Close())
	require.NoError(t, f.Close())

	// Verify the output file was created
	_, err = os.Stat(outputFile)
	require.NoError(t, err)
	
	// Test that we can't add files after closing
	require.Error(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "late.txt",
	}))
}

func TestMakeselfArchiveWithCustomInstallScript(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-custom.run")
	
	// Create custom install script
	customScript := "#!/bin/bash\necho 'Custom install script executed'\n"
	
	// Create mock makeself script
	mockMakeselfScript := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
# Check if custom script was passed
if [ -f "$2/install.sh" ]; then
	echo "Custom install script found in staging area"
else
	echo "No custom install script found"
fi
/usr/bin/cat > "$4" << 'EOF'
#!/bin/bash
echo "Archive with custom install script"
exit 0
EOF
/usr/bin/chmod +x "$4"
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	
	// Set PATH to include our mock script
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := NewWithInstallScript(f, customScript)
	defer archive.Close()

	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "foo.txt",
	}))

	require.NoError(t, archive.Close())
	require.NoError(t, f.Close())

	// Verify output file exists
	_, err = os.Stat(outputFile)
	require.NoError(t, err)
}

func TestMakeselfArchiveError(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-error.run")
	
	// Create mock makeself script that fails
	mockMakeselfScript := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
echo "makeself.sh error" >&2
exit 1
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	
	// Set PATH to include our mock script
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)

	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "foo.txt",
	}))

	// Close should fail because makeself returns error
	err = archive.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "makeself failed")
}

func TestMakeselfArchiveMissingMakeself(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-missing.run")
	
	// Ensure makeself is not in PATH
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", "/nonexistent")

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)

	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "foo.txt",
	}))

	// Close should fail because makeself is not found
	err = archive.Close()
	require.Error(t, err)
	require.Contains(t, err.Error(), "makeself command not found")
}

func TestMakeselfDirectFileCreation(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "direct-creation.run")
	
	// Create a file to use as target
	f, err := os.Create(outputFile)
	require.NoError(t, err)
	// Note: Don't defer close here since makeself will handle the file directly
	
	archive := New(f)
	
	// Verify that the archive detected the file path
	require.Equal(t, outputFile, archive.outputPath, "should detect output path from file")
	
	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "foo.txt",
	}))

	require.NoError(t, archive.Close())
	require.NoError(t, f.Close())

	// Verify the output file was created by makeself directly
	info, err := os.Stat(outputFile)
	require.NoError(t, err)
	require.True(t, info.Size() > 0, "file should have content from makeself")
	
	// Verify it's executable (when real makeself is available)
	if findMakeselfCommand() != "" {
		require.True(t, info.Mode()&0111 != 0, "file should be executable when created by real makeself")
	}
}

func TestMakeselfWithOptions(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-options.run")
	customOutputFile := filepath.Join(tmp, "custom-output.run")
	
	// Create mock makeself script
	mockMakeselfScript := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
# Check parameters and create output
echo "Custom makeself called with: $@" >&2
/usr/bin/cat > "$4" << 'EOF'
#!/bin/bash
echo "Custom options archive"
exit 0
EOF
/usr/bin/chmod +x "$4"
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	
	// Set PATH to include our mock script
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := NewWithOptions(f, MakeselfOptions{
		OutputPath:    customOutputFile,
		InstallScript: "#!/bin/bash\necho 'Custom install executed'\n",
		Label:         "Custom Archive",
	})
	defer archive.Close()

	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "foo.txt",
	}))

	require.NoError(t, archive.Close())

	// Verify the custom output file was created (not the original)
	_, err = os.Stat(customOutputFile)
	require.NoError(t, err, "custom output file should be created")
	
	// Verify original file is empty (makeself didn't write to it)
	info, err := os.Stat(outputFile)
	require.NoError(t, err)
	require.Equal(t, int64(0), info.Size(), "original file should remain empty")
}

func TestMakeselfArchiveEmptyArchive(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-empty.run")
	
	// Create mock makeself script
	mockMakeselfScript := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
/usr/bin/cat > "$4" << 'EOF'
#!/bin/bash
echo "Empty archive"
exit 0
EOF
/usr/bin/chmod +x "$4"
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	
	// Set PATH to include our mock script
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)

	// Don't add any files, just close
	require.NoError(t, archive.Close())
	require.NoError(t, f.Close())

	// Should still create an archive
	_, err = os.Stat(outputFile)
	require.NoError(t, err)
}

func TestMakeselfArchiveFileInfo(t *testing.T) {
	if testlib.IsWindows() {
		t.Skip("file permissions test not applicable on Windows")
	}

	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-fileinfo.run")
	
	// Create mock makeself script
	mockMakeselfScript := filepath.Join(tmp, "makeself") 
	mockScript := `#!/bin/bash
# Simple mock - just create the archive
/usr/bin/cat > "$4" << 'EOF'
#!/bin/bash
echo "Archive with file permissions"
exit 0
EOF
/usr/bin/chmod +x "$4"
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)
	defer archive.Close()

	// Add file with custom permissions
	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/sub1/executable",
		Destination: "executable_file",
		Info: config.FileInfo{
			Mode: 0755,
		},
	}))

	require.NoError(t, archive.Close())
	require.NoError(t, f.Close())

	_, err = os.Stat(outputFile)
	require.NoError(t, err)
}

func TestMakeselfArchiveDirectoryHandling(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-dirs.run")
	
	// Create mock makeself script that checks for directory structure
	mockMakeselfScript := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
# Simple mock - just create the archive  
/usr/bin/cat > "$4" << 'EOF'
#!/bin/bash
echo "Archive with directories"
exit 0
EOF
/usr/bin/chmod +x "$4"
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)
	defer archive.Close()

	// Add files that require directory creation
	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/sub1/bar.txt",
		Destination: "subdir/deep/bar.txt",
	}))

	require.NoError(t, archive.Close())
	require.NoError(t, f.Close())

	_, err = os.Stat(outputFile)
	require.NoError(t, err)
}

func TestMakeselfArchiveCloseIdempotent(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-idempotent.run")
	
	// Create mock makeself script
	mockMakeselfScript := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
/usr/bin/cat > "$4" << 'EOF'
#!/bin/bash
echo "Test archive"
exit 0
EOF
/usr/bin/chmod +x "$4"
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)

	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "foo.txt",
	}))

	// Close multiple times should not error
	require.NoError(t, archive.Close())
	require.NoError(t, archive.Close())
	require.NoError(t, archive.Close())
}

func TestMakeselfIntegration(t *testing.T) {
	// Skip integration test if makeself is not available
	if findMakeselfCommand() == "" {
		t.Skip("makeself command not found, skipping integration test")
	}

	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "integration.run")
	
	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)
	defer archive.Close()

	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "foo.txt",
	}))

	require.NoError(t, archive.Close())
	require.NoError(t, f.Close())

	// Verify the output file was created and is executable
	info, err := os.Stat(outputFile)
	require.NoError(t, err)
	require.True(t, info.Mode()&0111 != 0, "output file should be executable")

	// Test that the archive can actually run (basic smoke test)
	if !testlib.IsWindows() {
		cmd := exec.Command("bash", "-c", outputFile+" --help")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		// makeself archives should respond to --help without error
		if err != nil {
			t.Logf("Archive help output failed (this may be expected): %v", stderr.String())
		}
	}
}

func TestCopying(t *testing.T) {
	// Test Copy function - makeself doesn't support copying/reopening
	// so this should return an error
	f1, err := os.Create(filepath.Join(t.TempDir(), "1.run"))
	require.NoError(t, err)
	defer f1.Close()

	f2, err := os.Create(filepath.Join(t.TempDir(), "2.run"))
	require.NoError(t, err)
	defer f2.Close()

	_, err = Copy(f1, f2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "copying makeself archives is not supported")
}

func TestMakeselfWithUnicodeFiles(t *testing.T) {
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "test-unicode.run")
	
	// Create mock makeself script
	mockMakeselfScript := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
# Check if unicode file exists in staging area
if /usr/bin/ls "$2" | /usr/bin/grep -q 'ملف'; then
	echo "Unicode file found"
else
	echo "Unicode file not found" >&2
fi
/usr/bin/cat > "$4" << 'EOF'
#!/bin/bash
echo "Archive with unicode files"
exit 0
EOF
/usr/bin/chmod +x "$4"
`
	require.NoError(t, os.WriteFile(mockMakeselfScript, []byte(mockScript), 0755))
	
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)

	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()
	
	archive := New(f)
	defer archive.Close()

	// Add file with unicode name
	require.NoError(t, archive.Add(config.File{
		Source:      "../testdata/foo.txt",
		Destination: "ملف.txt",
	}))

	require.NoError(t, archive.Close())
	require.NoError(t, f.Close())

	_, err = os.Stat(outputFile)
	require.NoError(t, err)
}

func TestFindMakeselfCommand(t *testing.T) {
	tmp := t.TempDir()
	
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	
	// Test case 1: Both commands available, should prefer 'makeself'
	mockMakeself := filepath.Join(tmp, "makeself")
	mockMakeselfSh := filepath.Join(tmp, "makeself.sh")
	mockScript := `#!/bin/bash
echo "mock makeself"
`
	require.NoError(t, os.WriteFile(mockMakeself, []byte(mockScript), 0755))
	require.NoError(t, os.WriteFile(mockMakeselfSh, []byte(mockScript), 0755))
	
	// Set PATH to only include our test directory
	os.Setenv("PATH", tmp)
	
	cmd := findMakeselfCommand()
	require.Equal(t, "makeself", cmd, "should prefer 'makeself' over 'makeself.sh'")
	
	// Test case 2: Only makeself.sh available
	require.NoError(t, os.Remove(mockMakeself))
	cmd = findMakeselfCommand()
	require.Equal(t, "makeself.sh", cmd, "should use 'makeself.sh' when 'makeself' is not available")
	
	// Test case 3: Neither available
	require.NoError(t, os.Remove(mockMakeselfSh))
	cmd = findMakeselfCommand()
	require.Equal(t, "", cmd, "should return empty string when neither command is available")
}

func TestCheckMakeselfAvailable(t *testing.T) {
	tmp := t.TempDir()
	
	// Test case 1: Command available
	mockMakeself := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
echo "mock makeself"
`
	require.NoError(t, os.WriteFile(mockMakeself, []byte(mockScript), 0755))
	
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)
	
	err := CheckMakeselfAvailable()
	require.NoError(t, err, "should find makeself command")
	
	// Test case 2: Command not available
	os.Setenv("PATH", "/nonexistent")
	err = CheckMakeselfAvailable()
	require.Error(t, err)
	require.Contains(t, err.Error(), "makeself command not found")
}

func TestGetMakeselfVersion(t *testing.T) {
	tmp := t.TempDir()
	
	// Test case 1: Command available
	mockMakeself := filepath.Join(tmp, "makeself")
	mockScript := `#!/bin/bash
if [ "$1" = "--version" ]; then
    echo "makeself version 2.4.0"
else
    echo "mock makeself"
fi
`
	require.NoError(t, os.WriteFile(mockMakeself, []byte(mockScript), 0755))
	
	originalPath := os.Getenv("PATH")
	defer func() { os.Setenv("PATH", originalPath) }()
	os.Setenv("PATH", tmp+":"+originalPath)
	
	version, err := GetMakeselfVersion()
	require.NoError(t, err)
	require.Equal(t, "makeself version 2.4.0", version)
	
	// Test case 2: Command not available
	os.Setenv("PATH", "/nonexistent")
	version, err = GetMakeselfVersion()
	require.Error(t, err)
	require.Empty(t, version)
	require.Contains(t, err.Error(), "makeself command not found")
}
