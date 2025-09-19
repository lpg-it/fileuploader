package syncer

import (
	"fmt"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// ConnectSSH establishes an SSH connection
func ConnectSSH(host string, port int, user, password string) (*ssh.Client, *sftp.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", host, port)

	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial SSH: %v", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("failed to create SFTP client: %v", err)
	}

	return client, sftpClient, nil
}
