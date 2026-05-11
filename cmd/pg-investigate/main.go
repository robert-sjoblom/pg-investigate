package main

import (
	"fmt"
	"os"
	"strings"
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
	Investigation  string        `short:"i" required:"" help:"Investigation name"`
	Time           string        `short:"t" required:"" help:"Incident time"`
	Output         string        `short:"o" default:"./investigation" help:"Output dir"`
	Cluster        string        `long:"cluster" help:"Cluster name (e.g., prod-pg-f3c219); discovers all VMs across DCs and investigates each. Overrides --vm/--host/--dc."`
	Host           string        `name:"host" help:"SSH target (required when --cluster is unset)"`
	Vm             string        `name:"vm" help:"VM name (required when --cluster is unset)"`
	Namespace      string        `name:"ns" help:"Kubernetes namespace (derived from --cluster env prefix if unset)"`
	Insecure       bool          `long:"insecure" help:"Skip SSH host key verification"`
	PgVersion      string        `long:"pg-version" required:"" help:"PostgreSQL version"`
	Window         time.Duration `long:"window" short:"w" default:"10m" help:"Time window around incident"`
	DC             string        `long:"dc" help:"Datacenter (sto1, sto2, sto3); required when --cluster is unset"`
	HarvesterNode  string        `long:"harvester-node" help:"Override harvester node (skips VMI lookup; use for post-failover when VMI has moved)"`
	SkipSSH        bool          `long:"skip-ssh" help:"Skip SSH collection"`
	SkipKubectl    bool          `long:"skip-kubectl" help:"Skip kubectl collection (including VMI lookup; requires --harvester-node if templates use it)"`
	SkipOpenSearch bool          `long:"skip-opensearch" help:"Skip OpenSearch queries"`
}

type target struct {
	Vm        string
	Host      string
	Namespace string
	DC        string
}

func main() {
	_ = kong.Parse(&cli)

	cfg, err := config.Load()
	if err != nil {
		fail("Failed to load config: %v", err)
	}

	incidentTime, err := time.ParseInLocation("2006-01-02 15:04", cli.Time, time.Local)
	if err != nil {
		fail("Invalid time format, expected: YYYY-MM-DD HH:MM")
	}

	targets, err := resolveTargets(cfg)
	if err != nil {
		fail("%v", err)
	}

	// Prompt for OpenSearch password once so --cluster mode (multiple targets)
	// doesn't prompt per VM. Skip if all targets will skip OpenSearch.
	var osPassword string
	if !cli.SkipOpenSearch && anyTargetHasOpenSearch(cfg, targets) {
		osPassword, err = opensearch.PromptPassword()
		if err != nil {
			fail("read password: %v", err)
		}
	}

	for i, t := range targets {
		if len(targets) > 1 {
			fmt.Printf("\n=== [%d/%d] %s (%s) ===\n", i+1, len(targets), t.Vm, t.DC)
		}
		if err := investigate(t, cfg, incidentTime, osPassword); err != nil {
			fmt.Printf("Investigation failed for %s: %v\n", t.Vm, err)
			os.Exit(1)
		}
	}
}

func anyTargetHasOpenSearch(cfg *config.Config, targets []target) bool {
	for _, t := range targets {
		dc, ok := cfg.Datacenters[t.DC]
		if !ok {
			continue
		}
		if len(dc.OpenSearchAddresses) > 0 || len(cfg.OpenSearch.Addresses) > 0 {
			return true
		}
	}
	return false
}

func resolveTargets(cfg *config.Config) ([]target, error) {
	if cli.Cluster == "" {
		if cli.Vm == "" || cli.Host == "" || cli.DC == "" || cli.Namespace == "" {
			return nil, fmt.Errorf("--vm, --host, --dc, --ns are required when --cluster is unset")
		}
		return []target{{Vm: cli.Vm, Host: cli.Host, Namespace: cli.Namespace, DC: cli.DC}}, nil
	}

	namespace := cli.Namespace
	if namespace == "" {
		ns, err := namespaceFromCluster(cli.Cluster)
		if err != nil {
			return nil, err
		}
		namespace = ns
	}

	dcByContext := map[string]string{}
	for dc, dcCfg := range cfg.Datacenters {
		dcByContext[dcCfg.KubeContext] = dc
	}
	vms, err := kubectl.FindClusterVMs(cli.Cluster, namespace, dcByContext)
	if err != nil {
		return nil, fmt.Errorf("cluster discovery failed: %w", err)
	}
	if len(vms) == 0 {
		return nil, fmt.Errorf("no VMs found for cluster %s in namespace %s", cli.Cluster, namespace)
	}

	var out []target
	for _, v := range vms {
		out = append(out, target{
			Vm:        v.Name,
			Host:      fmt.Sprintf("%s.%s.fnox.se", v.Name, v.DC),
			Namespace: v.Namespace,
			DC:        v.DC,
		})
	}
	return out, nil
}

// namespaceFromCluster derives the kubernetes namespace from the cluster name's env prefix.
// "prod-pg-f3c219"      -> "db-prod001"
// "infra-public-pg-foo" -> "db-infra-public001"
func namespaceFromCluster(cluster string) (string, error) {
	var env string
	if strings.HasPrefix(cluster, "infra-public-") {
		env = "infra-public"
	} else if i := strings.Index(cluster, "-"); i > 0 {
		env = cluster[:i]
	}
	switch env {
	case "prod", "dev", "acce", "infra", "infra-public":
		return fmt.Sprintf("db-%s001", env), nil
	default:
		return "", fmt.Errorf("cannot derive namespace from cluster %q; pass --ns explicitly", cluster)
	}
}

// resolveHarvesterNodes returns the harvester nodes the VM lived on during the
// time window. Resolution order:
//  1. --harvester-node CLI override → single-node mode.
//  2. OpenSearch discovery (if a client is available) → all historical nodes.
//  3. kubectl VMI lookup → single-node, current placement.
func resolveHarvesterNodes(t target, dc config.DCConfig, osClient *opensearch.Client, since, until string) ([]string, error) {
	if cli.HarvesterNode != "" {
		return []string{cli.HarvesterNode}, nil
	}
	if osClient != nil {
		fmt.Printf("Discovering harvester nodes %s lived on...\n", t.Vm)
		hosts, err := osClient.DiscoverHosts("harvester-*", "virt-launcher-"+t.Vm+"-", since, until)
		if err != nil {
			fmt.Printf("Discovery failed (%v); falling back to VMI lookup\n", err)
		} else if len(hosts) > 0 {
			return hosts, nil
		}
	}
	if cli.SkipKubectl {
		return nil, nil
	}
	fmt.Printf("Getting VMI info for %s...\n", t.Vm)
	node, err := kubectl.GetVMINode(dc.KubeContext, t.Vm, t.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMI node: %w", err)
	}
	if node == "" {
		return nil, nil
	}
	return []string{node}, nil
}

func investigate(t target, cfg *config.Config, incidentTime time.Time, osPassword string) error {
	dc, ok := cfg.Datacenters[t.DC]
	if !ok {
		return fmt.Errorf("unknown datacenter: %s", t.DC)
	}

	since := incidentTime.Add(-cli.Window).Format(time.RFC3339)
	until := incidentTime.Add(cli.Window).Format(time.RFC3339)

	// Connect OpenSearch early so we can use it for harvester-node discovery
	// before any collection runs. Reused for the queries phase below.
	var osClient *opensearch.Client
	osAddresses := dc.OpenSearchAddresses
	if len(osAddresses) == 0 {
		osAddresses = cfg.OpenSearch.Addresses
	}
	if !cli.SkipOpenSearch && len(osAddresses) > 0 {
		var err error
		osClient, err = opensearch.Connect(osAddresses, cfg.OpenSearch.User, osPassword, cfg.OpenSearch.CACert)
		if err != nil {
			return fmt.Errorf("opensearch connect: %w", err)
		}
	}

	harvesterNodes, err := resolveHarvesterNodes(t, dc, osClient, since, until)
	if err != nil {
		return err
	}
	if len(harvesterNodes) > 1 {
		fmt.Printf("VM lived on harvester nodes: %s\n", strings.Join(harvesterNodes, ", "))
	} else if len(harvesterNodes) == 1 {
		fmt.Printf("VM is on harvester node: %s\n", harvesterNodes[0])
	}

	var primaryNode string
	if len(harvesterNodes) > 0 {
		primaryNode = harvesterNodes[0]
	}

	vars := config.TemplateVars{
		IncidentTime:   incidentTime,
		Since:          since,
		Until:          until,
		PgVersion:      cli.PgVersion,
		Host:           t.Host,
		Vm:             t.Vm,
		Namespace:      t.Namespace,
		KubeContext:    dc.KubeContext,
		HarvesterNode:  primaryNode,
		HarvesterNodes: harvesterNodes,
	}

	outputDir, err := output.BuildOutputPath(cli.Output, cli.Investigation, t.Vm)
	if err != nil {
		return err
	}

	if !cli.SkipKubectl {
		cmds, err := cfg.KubectlCommands(vars)
		if err != nil {
			return fmt.Errorf("render kubectl commands: %w", err)
		}
		if len(cmds) > 0 {
			if err := collector.NewKubectl(cmds).Collect(outputDir); err != nil {
				return fmt.Errorf("kubectl collection: %w", err)
			}
		}
	}

	if !cli.SkipSSH {
		cmds, err := cfg.SSHCommands(vars)
		if err != nil {
			return fmt.Errorf("render ssh commands: %w", err)
		}
		files, err := cfg.SSHFiles(vars)
		if err != nil {
			return fmt.Errorf("render ssh files: %w", err)
		}
		client, err := ssh.Connect(t.Host, cfg.SSH.User, cli.Insecure)
		if err != nil {
			return fmt.Errorf("ssh connect: %w", err)
		}
		if err := collector.NewSSH(client, cmds, files).Collect(outputDir); err != nil {
			return fmt.Errorf("ssh collection: %w", err)
		}
	}

	if osClient != nil {
		queries, err := cfg.OpensearchQueries(vars)
		if err != nil {
			return fmt.Errorf("render opensearch queries: %w", err)
		}
		if err := collector.NewOpenSearch(osClient, queries).Collect(outputDir); err != nil {
			return fmt.Errorf("opensearch collection: %w", err)
		}
	}

	fmt.Println("Done:", outputDir)
	return nil
}

func fail(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
	os.Exit(1)
}
