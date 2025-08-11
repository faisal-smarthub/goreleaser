# Makeself Archive Configuration

GoReleaser now supports advanced makeself archive configuration through the `goreleaser.yaml` file. This allows you to customize self-extracting archive behavior, install scripts, labels, and other makeself-specific options.

## Configuration Structure

```yaml
archives:
  - id: makeself-archive
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - makeself
    makeself:
      label: "{{ .ProjectName }} {{ .Version }} Installer"
      install_script: |
        #!/bin/bash
        echo "Installing {{ .ProjectName }}..."
        # Installation commands here
      install_script_file: "scripts/custom_install.sh"
      no_compression: true
      extra_args:
        - "--notemp"
        - "--license"
        - "LICENSE"
      # Optional: Provide LSM via inline template or external file
      lsm_template: |
        Begin4
        Title:          {{ .ProjectName }}
        Version:        {{ .Version }}
        Entered-date:   {{ .Date }}
        Description:    Example LSM
        Keywords:       example
        Author:         Example <example@example.com>
        Primary-site:   example.com
        Platforms:      {{ .Os }}-{{ .Arch }}
        Copying-policy: Proprietary
        End
      # lsm_file: "path/to/archive.lsm"
```

## Makeself Configuration Options

### `label`
- **Type**: `string`
- **Description**: Archive label shown when the self-extracting archive runs
- **Templates**: Supported
- **Example**: `"{{ .ProjectName }} {{ .Version }} Installer"`

### `install_script`
- **Type**: `string` (multiline)
- **Description**: Custom installation script to run after extraction
- **Templates**: Supported
- **Notes**: If not provided, a default script will be created
- **Example**:
  ```yaml
  install_script: |
    #!/bin/bash
    echo "Installing {{ .ProjectName }} {{ .Version }}..."
    mkdir -p /usr/local/bin
    cp {{ .ProjectName }} /usr/local/bin/
    chmod +x /usr/local/bin/{{ .ProjectName }}
    echo "Installation completed successfully!"
  ```

### `install_script_file`
- **Type**: `string`
- **Description**: Path to a script file to use as the installation script
- **Templates**: Supported
- **Priority**: Takes precedence over `install_script` if both are provided
- **Example**: `"scripts/install.sh"`

### `no_compression`
- **Type**: `boolean`
- **Description**: Disable compression for makeself archives
- **Default**: `true` (good for pre-compressed binaries)
- **Example**: `true`

### `extra_args`
- **Type**: `[]string`
- **Description**: Additional arguments to pass to the makeself command
- **Templates**: Supported
- **Example**: `["--notemp", "--noprogress", "--license", "LICENSE"]`

## Complete Examples

### Basic Makeself Archive

```yaml
archives:
  - id: linux-installer
    name_template: "{{ .ProjectName }}_{{ .Version }}_installer"
    formats:
      - makeself
    format_overrides:
      - goos: linux
        formats:
          - makeself
    makeself:
      label: "{{ .ProjectName }} Installer"
      install_script: |
        #!/bin/bash
        echo "Installing {{ .ProjectName }}..."
        cp {{ .ProjectName }} /usr/local/bin/
        chmod +x /usr/local/bin/{{ .ProjectName }}
        echo "Done!"
```

### Advanced Makeself Archive with External Script

```yaml
archives:
  - id: advanced-installer
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - makeself
    makeself:
      label: "{{ .ProjectName }} {{ .Version }} Professional Installer"
      install_script_file: "packaging/install.sh"
      no_compression: false
      extra_args:
        - "--license"
        - "LICENSE"
        - "--notemp"
        - "--noprogress"
```

### Multiple Archive Formats

```yaml
archives:
  - id: default-archives
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
      - zip
    
  - id: linux-installer
    name_template: "{{ .ProjectName }}_{{ .Version }}_installer"
    formats:
      - makeself
    format_overrides:
      - goos: linux
        formats:
          - makeself
    makeself:
      label: "{{ .ProjectName }} Self-Installing Package"
      install_script: |
        #!/bin/bash
        echo "{{ .ProjectName }} {{ .Version }} Installer"
        echo "======================================"
        
        # Create installation directory
        INSTALL_DIR="/usr/local/bin"
        mkdir -p "$INSTALL_DIR"
        
        # Copy binary
        cp "{{ .ProjectName }}" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/{{ .ProjectName }}"
        
        # Create symbolic link if desired
        if [ ! -e "/usr/bin/{{ .ProjectName }}" ]; then
          ln -s "$INSTALL_DIR/{{ .ProjectName }}" "/usr/bin/{{ .ProjectName }}"
        fi
        
        echo "Installation completed successfully!"
        echo "Run '{{ .ProjectName }} --help' to get started."
      no_compression: true
```

## Template Variables

All makeself configuration fields support Go templates with access to the same variables available in other GoReleaser templates:

- `{{ .ProjectName }}` - Project name
- `{{ .Version }}` - Release version
- `{{ .Tag }}` - Git tag
- `{{ .Os }}` - Target operating system
- `{{ .Arch }}` - Target architecture
- And many more...

## Best Practices

1. **Use `no_compression: true`** for binaries that are already compressed or when you want faster extraction
2. **Keep install scripts simple** and handle errors gracefully
3. **Use `install_script_file`** for complex installation logic to keep your YAML clean
4. **Test your installers** on target systems before release
5. **Include helpful user messages** in your install scripts
6. **Consider using `--notemp`** and `--noprogress`** extra args for cleaner output

## Integration with Existing Features

The makeself configuration integrates seamlessly with other GoReleaser features:

- **Format overrides**: Use `format_overrides` to create makeself archives only for specific platforms
- **Multiple archives**: Create both traditional archives and makeself installers
- **Templating**: Use the full power of GoReleaser's templating system
- **File inclusion**: Include additional files using the standard `files` configuration
- **Hooks**: Use before/after hooks as usual

This enhancement makes GoReleaser a powerful tool for creating professional, user-friendly software installers for Linux systems while maintaining compatibility with all existing functionality.
