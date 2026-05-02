package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fortnox/pg-investigate/internal/collector"
	"github.com/fortnox/pg-investigate/internal/config"
	"github.com/fortnox/pg-investigate/internal/output"
	"github.com/fortnox/pg-investigate/internal/ssh"
)

var cli struct {
	Investigation string `short:"i" required:"" help:"Investigation name"`
	Time          string `short:"t" required:"" help:"Incident time"`
	Output        string `short:"o" default:"./investigation" help:"Output dir"`
	Host          string `name:"host" required:"" help:"SSH target"`
	Vm            string `name:"vm" required:"" help:"Harvester VM name"`
	Namespace     string `name:"ns" required:"" help:"Kubernetes namespace"`
	Insecure      bool   `long:"insecure" help:"Skip SSH host key verification"`
	PgVersion     string `long:"pg-version" required:"" help:"PostgreSQL version"`
}

func main() {
	_ = kong.Parse(&cli)

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Failed to load config:", err)
		os.Exit(1)
	}

	incidentTime, err := time.ParseInLocation("2006-01-02 15:04", cli.Time, time.Local)
	if err != nil {
		fmt.Println("Invalid time format, expected: YYYY-MM-DD HH:MM")
		os.Exit(1)
	}

	vars := config.TemplateVars{
		IncidentTime: incidentTime,
		Since:        incidentTime.Add(-1 * time.Hour).Format("2006-01-02 15:04:05"),
		Until:        incidentTime.Add(1 * time.Hour).Format("2006-01-02 15:04:05"),
		PgVersion:    cli.PgVersion,
	}

	client, err := ssh.Connect(cli.Host, cfg.SSH.User, cli.Insecure)
	if err != nil {
		fmt.Println("SSH connection failed:", err)
		os.Exit(1)
	}

	outputDir, err := output.BuildOutputPath(cli.Output, cli.Investigation, cli.Vm)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	commands, err := cfg.SSHCommands(vars)
	if err != nil {
		fmt.Println("Failed to render commands:", err)
		os.Exit(1)
	}

	files, err := cfg.SSHFiles(vars)
	if err != nil {
		fmt.Println("Failed to render files:", err)
		os.Exit(1)
	}

	c := collector.NewSSH(client, commands, files)
	if err := c.Collect(outputDir); err != nil {
		fmt.Println("Collection failed:", err)
		os.Exit(1)
	}

	fmt.Println("Done:", outputDir)
}
