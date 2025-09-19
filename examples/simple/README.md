# Simple Example

This is a simple example showing how to use the fileuploader library in your own Go project.

## Usage

1. Copy the config.yaml file from the parent directory:
   ```bash
   cp ../config.yaml .
   ```

2. Edit the config.yaml file with your specific settings

3. Run the example:
   ```bash
   go run main.go
   ```

This example demonstrates the basic usage of the fileuploader library as an imported package.

## Importing the Library

To use this library in your own project, add it as a dependency:

```bash
go get github.com/lpg-it/fileuploader
```

Then import it in your Go code:

```go
import "github.com/lpg-it/fileuploader/syncer"
```