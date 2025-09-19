package syncer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
)

// SSHConfig holds SSH connection parameters
type SSHConfig struct {
	Host     string
	Port     int
	User     string
	Password string
}

// SyncConfig holds synchronization parameters
type SyncConfig struct {
	LocalPath  string
	RemotePath string
	Mode       string
	Workers    int
}

// FileInfo represents file information for synchronization
type FileInfo struct {
	Path    string
	RelPath string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// Syncer handles file synchronization
type Syncer struct {
	client     *sftp.Client
	localPath  string
	remotePath string
	mode       string
	workers    int
	bar        *pb.ProgressBar
	totalSize  int64
	syncedSize int64
	mutex      sync.Mutex
	logger     *logrus.Logger
}

// New creates a new Syncer instance with direct configuration
func New(client *sftp.Client, syncConfig SyncConfig, logger *logrus.Logger) *Syncer {
	return &Syncer{
		client:     client,
		localPath:  syncConfig.LocalPath,
		remotePath: syncConfig.RemotePath,
		mode:       syncConfig.Mode,
		workers:    syncConfig.Workers,
		logger:     logger,
	}
}

// Sync performs the file synchronization based on the mode
func (s *Syncer) Sync() error {
	switch s.mode {
	case "full":
		return s.fullSync()
	case "incremental":
		return s.incrementalSync()
	default:
		return fmt.Errorf("unsupported sync mode: %s", s.mode)
	}
}

// fullSync performs a full synchronization (replace everything)
func (s *Syncer) fullSync() error {
	s.logger.Info("Performing full synchronization...")

	// Collect local files
	localFiles, err := s.collectLocalFiles()
	if err != nil {
		return fmt.Errorf("failed to collect local files: %v", err)
	}

	// Calculate total size for progress bar
	s.totalSize = 0
	for _, file := range localFiles {
		if !file.IsDir {
			s.totalSize += file.Size
		}
	}

	// Create progress bar
	s.bar = pb.Full.Start64(s.totalSize)
	s.bar.Set(pb.Bytes, true)
	s.bar.SetWidth(80)
	s.bar.SetRefreshRate(time.Second)
	s.bar.Set(pb.Terminal, false)
	s.bar.Set(pb.Static, false)
	s.bar.SetTemplateString(`\rSync Progress: {{bar . }} {{percent . }} {{speed . }} {{counters . }}`)

	// Create temporary remote directory
	tempRemotePath := filepath.Join(filepath.Dir(s.remotePath), ".sync_tmp_"+time.Now().Format("20060102_150405"))
	s.logger.Infof("Creating temporary directory: %s", tempRemotePath)

	if err := s.client.MkdirAll(tempRemotePath); err != nil {
		s.bar.Finish()
		return fmt.Errorf("failed to create temporary directory: %v", err)
	}

	// Ensure cleanup of temporary directory
	defer func() {
		s.logger.Infof("Cleaning up temporary directory: %s", tempRemotePath)
		if err := s.client.RemoveDirectory(tempRemotePath); err != nil {
			s.logger.Warnf("Failed to remove temporary directory: %v", err)
		}
		s.bar.Finish()
	}()

	// Upload files using worker pool
	if err := s.uploadFiles(localFiles, tempRemotePath); err != nil {
		return fmt.Errorf("failed to upload files: %v", err)
	}

	// Backup existing remote directory
	backupPath := s.remotePath + ".bak_" + time.Now().Format("20060102_150405")
	s.logger.Infof("Creating backup at: %s", backupPath)

	if _, err := s.client.Stat(s.remotePath); err == nil {
		// Remote path exists, rename it to backup
		if err := s.client.Rename(s.remotePath, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
	}

	// Rename temporary directory to target directory
	s.logger.Infof("Renaming %s to %s", tempRemotePath, s.remotePath)
	if err := s.client.Rename(tempRemotePath, s.remotePath); err != nil {
		// Try to restore from backup if rename fails
		if _, backupErr := s.client.Stat(backupPath); backupErr == nil {
			if restoreErr := s.client.Rename(backupPath, s.remotePath); restoreErr != nil {
				return fmt.Errorf("sync failed and restore failed: %v, restore error: %v", err, restoreErr)
			}
			return fmt.Errorf("sync failed, restored from backup: %v", err)
		}
		return fmt.Errorf("failed to rename temporary directory: %v", err)
	}

	// Remove backup directory
	if _, err := s.client.Stat(backupPath); err == nil {
		s.logger.Infof("Removing backup directory: %s", backupPath)
		if err := s.client.RemoveDirectory(backupPath); err != nil {
			s.logger.Warnf("Failed to remove backup directory: %v", err)
		}
	}

	return nil
}

// incrementalSync performs an incremental synchronization
func (s *Syncer) incrementalSync() error {
	s.logger.Info("Performing incremental synchronization...")

	// Collect local files
	localFiles, err := s.collectLocalFiles()
	if err != nil {
		return fmt.Errorf("failed to collect local files: %v", err)
	}

	// Calculate total size for progress bar
	s.totalSize = 0
	for _, file := range localFiles {
		if !file.IsDir {
			s.totalSize += file.Size
		}
	}

	// Create progress bar
	s.bar = pb.Full.Start64(s.totalSize)
	s.bar.Set(pb.Bytes, true)
	s.bar.SetWidth(80)
	s.bar.SetRefreshRate(time.Second)
	s.bar.Set(pb.Terminal, false)
	s.bar.Set(pb.Static, false)
	s.bar.SetTemplateString(`\rSync Progress: {{bar . }} {{percent . }} {{speed . }} {{counters . }}`)

	// Ensure remote directory exists
	if err := s.client.MkdirAll(s.remotePath); err != nil {
		s.bar.Finish()
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	// Upload files using worker pool
	if err := s.uploadFiles(localFiles, s.remotePath); err != nil {
		s.bar.Finish()
		return fmt.Errorf("failed to upload files: %v", err)
	}

	s.bar.Finish()
	return nil
}

// collectLocalFiles walks the local directory and collects file information
func (s *Syncer) collectLocalFiles() ([]FileInfo, error) {
	var files []FileInfo

	s.logger.Infof("Collecting files from: %s", s.localPath)
	err := filepath.Walk(s.localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.localPath, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		files = append(files, FileInfo{
			Path:    path,
			RelPath: relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		})

		if info.IsDir() {
			s.logger.Debugf("Found directory: %s", relPath)
		} else {
			s.logger.Debugf("Found file: %s (%d bytes)", relPath, info.Size())
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk local directory: %v", err)
	}

	s.logger.Infof("Found %d files/directories", len(files))
	return files, nil
}

// uploadFiles uploads files using a worker pool
func (s *Syncer) uploadFiles(files []FileInfo, remoteBasePath string) error {
	// Create channels for work distribution
	jobs := make(chan FileInfo, len(files))
	errors := make(chan error, len(files))

	// Buffer pool for efficient memory usage
	bufPool := sync.Pool{
		New: func() interface{} {
			return make([]byte, 32*1024) // 32KB buffer
		},
	}

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go s.worker(&wg, jobs, errors, remoteBasePath, &bufPool)
	}

	// Send jobs to workers
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Wait for all workers to complete
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			return fmt.Errorf("worker error: %v", err)
		}
	}

	return nil
}

// worker processes file upload jobs
func (s *Syncer) worker(wg *sync.WaitGroup, jobs <-chan FileInfo, errors chan<- error, remoteBasePath string, bufPool *sync.Pool) {
	defer wg.Done()

	for file := range jobs {
		if file.IsDir {
			// Create remote directory
			remoteDirPath := filepath.Join(remoteBasePath, file.RelPath)
			s.logger.Debugf("Creating remote directory: %s", remoteDirPath)

			if err := s.client.MkdirAll(remoteDirPath); err != nil {
				errors <- fmt.Errorf("failed to create remote directory %s: %v", remoteDirPath, err)
				continue
			}
		} else {
			// Upload file
			if err := s.uploadFile(file, remoteBasePath, bufPool); err != nil {
				errors <- fmt.Errorf("failed to upload file %s: %v", file.RelPath, err)
				continue
			}
		}
	}
}

// uploadFile uploads a single file
func (s *Syncer) uploadFile(file FileInfo, remoteBasePath string, bufPool *sync.Pool) error {
	// Open local file
	localFile, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFile.Close()

	// Create remote file path
	remoteFilePath := filepath.Join(remoteBasePath, file.RelPath)
	remoteDir := filepath.Dir(remoteFilePath)

	// Ensure remote directory exists
	if err := s.client.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	// Create remote file
	remoteFile, err := s.client.Create(remoteFilePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close()

	// Copy file content
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)

	for {
		n, err := localFile.Read(buf)
		if n > 0 {
			if _, writeErr := remoteFile.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write to remote file: %v", writeErr)
			}

			// Update progress
			s.mutex.Lock()
			s.syncedSize += int64(n)
			s.bar.SetCurrent(s.syncedSize)
			s.mutex.Unlock()
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read local file: %v", err)
		}
	}

	// Set file permissions and modification time
	if err := s.client.Chmod(remoteFilePath, 0644); err != nil {
		s.logger.Warnf("Failed to set permissions for %s: %v", remoteFilePath, err)
	}

	if err := s.client.Chtimes(remoteFilePath, time.Now(), file.ModTime); err != nil {
		s.logger.Warnf("Failed to set modification time for %s: %v", remoteFilePath, err)
	}

	s.logger.Debugf("Uploaded: %s (%d bytes)", file.RelPath, file.Size)
	return nil
}
