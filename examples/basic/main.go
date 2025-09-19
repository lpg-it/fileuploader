package main

import (
	"log"

	"github.com/lpg-it/fileuploader/syncer"
	"github.com/sirupsen/logrus"
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
