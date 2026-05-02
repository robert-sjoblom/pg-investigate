# pg-investigate

CLI tool to gather diagnostic data for PostgreSQL failover investigations on Harvester VMs with SolidFire storage.

## Usage

```bash
pg-investigate --vm <vm-name> --namespace <ns> --time "2026-05-02 04:35:00" --output ./investigation/
```

## What it collects

### From the VM (via SSH)
- `/var/lib/pgsql/{version}/data/log/postgresql-*.log`
- `/var/log/repmgr/repmgrd.log`
- `dmesg -T`
- `journalctl -k --since <time-5m> --until <time+5m>`

### From Kubernetes
- VMI details (node placement)
- PVC info (volume names, storage classes)
- StorageClass config
- TridentBackend config
- Trident node pod logs

### From Harvester node (via kubectl debug)
- `journalctl -k`
- `journalctl -u iscsid`
- `journalctl -u multipathd`
- `multipath -ll`

### From Prometheus (optional)
- SolidFire burst credits
- SolidFire throttle time
- Node disk I/O metrics

## Project structure

```
pg-investigate/
├── cmd/pg-investigate/main.go   # CLI entry point
├── internal/
│   ├── collector/               # Data collection implementations
│   │   ├── collector.go         # Collector interface
│   │   ├── vm.go                # SSH/SCP to VM
│   │   ├── kubernetes.go        # kubectl operations
│   │   ├── harvester.go         # kubectl debug to node
│   │   └── prometheus.go        # PromQL queries
│   └── output/
│       └── output.go            # Write results to files
├── go.mod
└── README.md
```

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `golang.org/x/crypto/ssh` - SSH connections
- `k8s.io/client-go` - Kubernetes API client

## Design notes

1. Each collector implements a common interface
2. Collectors run concurrently where possible
3. Output goes to timestamped directory with raw files + summary.md
4. Errors are collected but don't stop other collectors
