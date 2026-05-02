package collector

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fortnox/pg-investigate/internal/config"
	"github.com/fortnox/pg-investigate/internal/ssh"
)

type SSHCollector struct {
	client   *ssh.Client
	commands []config.CommandItem
	files    []config.FileItem
}

func NewSSH(client *ssh.Client, commands []config.CommandItem, files []config.FileItem) *SSHCollector {
	return &SSHCollector{client: client, commands: commands, files: files}
}

func (c *SSHCollector) Collect(outputDir string) error {
	for _, cmd := range c.commands {
		fmt.Printf("Running: %s\n", cmd.Command)
		dest := filepath.Join(outputDir, cmd.Name)
		out, err := c.client.Run(cmd.Command)
		if err != nil {
			fmt.Printf("  -> %s (exit error: %s)\n", dest, err)
		} else {
			fmt.Printf("  -> %s\n", dest)
		}
		os.WriteFile(dest, out, 0644)
	}

	for _, f := range c.files {
		cmd := "sudo cat " + f.Path
		if f.Gzip {
			cmd = "sudo zcat " + f.Path
		}
		fmt.Printf("Running: %s\n", cmd)
		dest := filepath.Join(outputDir, f.Name)
		out, err := c.client.Run(cmd)
		if err != nil {
			if f.Optional {
				fmt.Printf("  -> skipped (optional): %s\n", err)
				continue
			}
			fmt.Printf("FAILED: %s\nError: %s\nOutput: %s\n", cmd, err, string(out))
			return err
		}
		os.WriteFile(dest, out, 0644)
		fmt.Printf("  -> %s\n", dest)
	}

	return nil
}
