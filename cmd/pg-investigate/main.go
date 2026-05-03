package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/fortnox/pg-investigate/internal/collector"
	"github.com/fortnox/pg-investigate/internal/config"
	"github.com/fortnox/pg-investigate/internal/kubectl"
	"github.com/fortnox/pg-investigate/internal/opensearch"
	"github.com/fortnox/pg-investigate/internal/output"
	"github.com/fortnox/pg-investigate/internal/ssh"
)

var cli struct {
	Investigation string        `short:"i" required:"" help:"Investigation name"`
	Time          string        `short:"t" required:"" help:"Incident time"`
	Output        string        `short:"o" default:"./investigation" help:"Output dir"`
	Host          string        `name:"host" required:"" help:"SSH target"`
	Vm            string        `name:"vm" required:"" help:"Harvester VM name"`
	Namespace     string        `name:"ns" required:"" help:"Kubernetes namespace"`
	Insecure      bool          `long:"insecure" help:"Skip SSH host key verification"`
	PgVersion     string        `long:"pg-version" required:"" help:"PostgreSQL version"`
	Window        time.Duration `long:"window" short:"w" default:"10m" help:"Time window around incident"`
	DC            string        `long:"dc" required:"" help:"Datacenter (sto1, sto2, sto3)"`
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

	dc, ok := cfg.Datacenters[cli.DC]
	if !ok {
		fmt.Printf("Unknown datacenter: %s\n", cli.DC)
		os.Exit(1)
	}

	// Get harvester node from VMI
	fmt.Printf("Getting VMI info for %s...\n", cli.Vm)
	harvesterNode, err := kubectl.GetVMINode(dc.KubeContext, cli.Vm, cli.Namespace)
	if err != nil {
		fmt.Println("Failed to get VMI node:", err)
		os.Exit(1)
	}
	fmt.Printf("VM is on harvester node: %s\n", harvesterNode)

	vars := config.TemplateVars{
		IncidentTime:  incidentTime,
		Since:         incidentTime.Add(-cli.Window).Format(time.RFC3339),
		Until:         incidentTime.Add(cli.Window).Format(time.RFC3339),
		PgVersion:     cli.PgVersion,
		Host:          cli.Host,
		Vm:            cli.Vm,
		Namespace:     cli.Namespace,
		KubeContext:   dc.KubeContext,
		HarvesterNode: harvesterNode,
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

	// Run kubectl commands
	kubectlCommands, err := cfg.KubectlCommands(vars)
	if err != nil {
		fmt.Println("Failed to render kubectl commands:", err)
		os.Exit(1)
	}

	if len(kubectlCommands) > 0 {
		kubectlCollector := collector.NewKubectl(kubectlCommands)
		if err := kubectlCollector.Collect(outputDir); err != nil {
			fmt.Println("Kubectl collection failed:", err)
			os.Exit(1)
		}
	}

	// Run SSH commands
	sshCollector := collector.NewSSH(client, commands, files)
	if err := sshCollector.Collect(outputDir); err != nil {
		fmt.Println("SSH collection failed:", err)
		os.Exit(1)
	}

	if len(cfg.OpenSearch.Addresses) > 0 {
		osClient, err := opensearch.Connect(cfg.OpenSearch.Addresses, cfg.OpenSearch.User, cfg.OpenSearch.CACert)
		if err != nil {
			fmt.Println("OpenSearch connection failed:", err)
			os.Exit(1)
		}

		queries, err := cfg.OpensearchQueries(vars)
		if err != nil {
			fmt.Println("Failed to render OpenSearch queries:", err)
			os.Exit(1)
		}

		osCollector := collector.NewOpenSearch(osClient, queries)
		if err := osCollector.Collect(outputDir); err != nil {
			fmt.Println("OpenSearch collection failed:", err)
			os.Exit(1)
		}
	}

	fmt.Println("Done:", outputDir)
}
