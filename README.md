# Universal File Synchronization Library

A flexible Go-based library for synchronizing local files to a remote server via SSH/SFTP with multiple synchronization modes. This is a library-only package intended to be imported by other Go projects.

## Features

- **Multiple Sync Modes**:
  - Full Replacement: Completely replaces the remote directory with local files
  - Incremental Sync: Only uploads new or modified files
- **Concurrent Uploads**: Uses worker pools for faster uploads
- **Progress Tracking**: Visual progress bar showing upload status
- **Logging**: Detailed logs for troubleshooting
- **Safety Measures**: Automatic backup and rollback capabilities
- **Configuration File**: All settings managed through a single YAML configuration file

## Installation

To use as a library in your Go project, add it as a dependency:

```bash
go get github.com/lpg-it/fileuploader
```

## Usage

```go
package main

import (
    "log"
    
    "github.com/sirupsen/logrus"
    "github.com/lpg-it/fileuploader/syncer"
)

func main() {
    // Load configuration
    config, err := syncer.LoadConfig("config.yaml")
    if err != nil {
        log.Fatal(err)
    }
    
    // Setup logger
    logger := logrus.New()
    
    // Connect to SSH server
    sshClient, sftpClient, err := syncer.ConnectSSH(
        config.SSH.Host,
        config.SSH.Port,
        config.SSH.User,
        config.SSH.Password,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer sshClient.Close()
    defer sftpClient.Close()
    
    // Create syncer
    fileSyncer := syncer.New(sftpClient, config, logger)
    
    // Perform synchronization
    if err := fileSyncer.Sync(); err != nil {
        log.Fatal(err)
    }
}
```

## Package Structure

- `syncer`: Main package containing synchronization logic
  - `syncer.go`: Core synchronization functionality
  - `config.go`: Configuration loading utilities
  - `ssh.go`: SSH connection utilities

## Sync Modes

### Full Replacement (`full`)

This mode completely replaces the remote directory with the local files:

1. Creates a temporary directory on the remote server
2. Uploads all local files to the temporary directory
3. Backs up the existing remote directory (if it exists)
4. Replaces the remote directory with the temporary directory
5. Removes the backup if successful, or restores it if failed

### Incremental Sync (`incremental`)

This mode only uploads files that exist locally but not on the remote server, or have been modified:

1. Walks through the local directory structure
2. For each file:
   - If it doesn't exist remotely, uploads it
   - If it exists but has different size/modification time, uploads it
3. Preserves files that exist remotely but not locally

## Logging

When using this library, you can configure logging through the logger you pass to the [New()](file:///Users/lipeiguan/projects/fileuploader/syncer/syncer.go#L74-L79) function. The library will use your logger for all its operations.

## Security Notes

- When using this library, ensure that passwords and other sensitive information are handled securely
- For production use, consider using SSH key authentication instead of passwords
- The library disables host key verification for convenience (InsecureIgnoreHostKey)
