# pg-investigate

CLI tool to gather diagnostic data for PostgreSQL failover investigations on Harvester VMs with SolidFire storage.

## Usage

```bash
pg-investigate \
  -i <investigation-name> \
  -t "2026-05-02 04:35" \
  --host <ssh-target> \
  --vm <vm-name> \
  --ns <namespace> \
  --pg-version 17 \
  [--insecure]
```

Example:
```bash
pg-investigate -i dev-pg-app013 -t "2026-05-02 04:35" \
  --host dev-pg-app013-db001.sto1.fnox.se \
  --vm dev-pg-app013-db001 \
  --ns db-dev001 \
  --pg-version 17 \
  --insecure
```

Output goes to: `investigation/<name>/<date>/<vm>/`

## Configuration

Create `~/.config/pg-investigate/config.yaml`:

```yaml
ssh:
  user: your_username
  commands:
    - name: dmesg.txt
      command: "sudo dmesg -T | grep -iE 'error|fail|i/o|ext4|xfs' | tail -100"
    - name: journalctl-kernel.txt
      command: "sudo journalctl -k --since '{{.Since}}' --until '{{.Until}}' --no-pager"
    - name: journalctl-postgres.txt
      command: "sudo journalctl -u postgresql-{{.PgVersion}} --since '{{.Since}}' --until '{{.Until}}' --no-pager"
    - name: systemd-timers.txt
      command: "sudo systemctl list-timers --all"
    - name: systemd-status-postgres.txt
      command: "sudo systemctl status postgresql-{{.PgVersion}}"
  files:
    - name: postgresql.log
      path: "/var/lib/pgsql/{{.PgVersion}}/data/log/postgresql-{{.Weekday}}.log"
    - name: repmgrd.log
      path: "/var/log/repmgr/repmgrd.log"
      optional: true
    - name: repmgrd-rotated.log
      path: "/var/log/repmgr/repmgrd.log-{{.Date}}.gz"
      gzip: true
      optional: true

opensearch:
  addresses:
    - https://logs.example.com:9200
  index: "logs-*"
```

### Template variables

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Since}}` | Incident time - 1 hour | `2026-05-02 03:35:05` |
| `{{.Until}}` | Incident time + 1 hour | `2026-05-02 05:35:05` |
| `{{.Weekday}}` | Abbreviated weekday | `Sat` |
| `{{.Date}}` | Date + 1 day (YYYYMMDD) | `20260503` |
| `{{.PgVersion}}` | PostgreSQL version | `17` |

Note: `{{.Date}}` adds 1 day because log rotation names files by rotation date, not log date.

### File options

| Option | Description |
|--------|-------------|
| `gzip: true` | Use `zcat` instead of `cat` |
| `optional: true` | Skip without error if file doesn't exist |

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

- `github.com/alecthomas/kong` - CLI framework
- `golang.org/x/crypto/ssh` - SSH connections via ssh-agent
- `gopkg.in/yaml.v3` - Config parsing

## SSH Authentication

Uses ssh-agent via `SSH_AUTH_SOCK`. Make sure your key is loaded:

```bash
ssh-add -l
```

Use `--insecure` to skip host key verification.
