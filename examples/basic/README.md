# Basic Example

This is a basic example showing how to use the fileuploader library in your own Go project.

## Usage

The example demonstrates the direct parameter approach:

1. Edit the main.go file with your specific settings:
   - SSH connection details (host, port, user, password)
   - Local and remote directory paths
   - Sync mode (full or incremental)
   - Number of worker threads

2. Run the example:
   ```bash
   go run main.go
   ```

## Key Points

- The library does NOT require or support configuration files
- All configuration is provided directly as parameters
- You can modify the source code to use environment variables, configuration files, or hardcoded values as needed
- The library is completely agnostic to how you obtain the configuration parameters