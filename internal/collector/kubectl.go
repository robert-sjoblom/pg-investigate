package collector

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fortnox/pg-investigate/internal/config"
	"github.com/fortnox/pg-investigate/internal/kubectl"
)

type KubectlCollector struct {
	commands []config.CommandItem
}

func NewKubectl(commands []config.CommandItem) *KubectlCollector {
	return &KubectlCollector{commands: commands}
}

func (c *KubectlCollector) Collect(outputDir string) error {
	for _, cmd := range c.commands {
		fmt.Printf("Running: %s\n", cmd.Command)
		dest := filepath.Join(outputDir, cmd.Name)
		out, err := kubectl.Run(cmd.Command)
		if err != nil {
			fmt.Printf("  -> %s (exit error: %s)\n", dest, err)
		} else {
			fmt.Printf("  -> %s\n", dest)
		}
		os.WriteFile(dest, out, 0644)
	}
	return nil
}
