package main

import (
	"log"

	"github.com/lpg-it/fileuploader/syncer"
	"github.com/sirupsen/logrus"
)

func main() {
	// Setup logger
	logger := logrus.New()

	// Connect to SSH server directly
	sshClient, sftpClient, err := syncer.ConnectSSH(
		"your-server-host.com", // host
		22,                     // port
		"your-username",        // user
		"your-password",        // password
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sshClient.Close()
	defer sftpClient.Close()

	// Create sync config directly
	syncConfig := syncer.SyncConfig{
		LocalPath:  "/path/to/your/local/directory",
		RemotePath: "/path/to/your/remote/directory",
		Mode:       "full", // or "incremental"
		Workers:    10,
	}

	// Create syncer
	fileSyncer := syncer.New(sftpClient, syncConfig, logger)

	// Perform synchronization
	if err := fileSyncer.Sync(); err != nil {
		log.Fatal(err)
	}
}
