package ssh

import (
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Client struct {
	conn *ssh.Client
}

func Connect(host, user string, insecure bool) (*Client, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	agentConn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, err
	}

	var hostKeyCallback ssh.HostKeyCallback
	if insecure {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		home, _ := os.UserHomeDir()
		knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
		hostKeyCallback, err = knownhosts.New(knownHostsPath)
		if err != nil {
			return nil, err
		}
	}

	agentClient := agent.NewClient(agentConn)
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeysCallback(agentClient.Signers)},
		HostKeyCallback: hostKeyCallback,
	}

	conn, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return nil, err
	}

	return &Client{conn: conn}, nil
}

func (c *Client) Run(cmd string) ([]byte, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	return session.CombinedOutput(cmd)
}
