package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	SSH         SSHConfig           `yaml:"ssh"`
	OpenSearch  OpenSearchConfig    `yaml:"opensearch"`
	Kubectl     KubectlConfig       `yaml:"kubectl"`
	Datacenters map[string]DCConfig `yaml:"datacenters"`
}

type KubectlConfig struct {
	VMIQuery string        `yaml:"vmi_query"`
	Commands []CommandItem `yaml:"commands"`
}

type DCConfig struct {
	KubeContext         string   `yaml:"kubecontext"`
	OpenSearchAddresses []string `yaml:"opensearch_addresses"`
}

type SSHConfig struct {
	User     string        `yaml:"user"`
	Commands []CommandItem `yaml:"commands"`
	Files    []FileItem    `yaml:"files"`
}

type CommandItem struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
}

type FileItem struct {
	Name     string `yaml:"name"`
	Path     string `yaml:"path"`
	Gzip     bool   `yaml:"gzip"`
	Optional bool   `yaml:"optional"`
}

type OpenSearchConfig struct {
	Addresses []string    `yaml:"addresses"`
	User      string      `yaml:"user"`
	CACert    string      `yaml:"ca_cert"`
	Queries   []QueryItem `yaml:"queries"`
}

type QueryItem struct {
	Name  string `yaml:"name"`
	Index string `yaml:"index"`
	Query string `yaml:"query"`
}

func Load() (*Config, error) {
	dir, _ := os.UserConfigDir()
	path := filepath.Join(dir, "pg-investigate", "config.yaml")
	return LoadFrom(path)
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	return &cfg, err
}

type TemplateVars struct {
	IncidentTime  time.Time
	Since         string
	Until         string
	PgVersion     string
	Host          string
	Vm            string
	Namespace     string
	KubeContext   string
	HarvesterNode string
}

func (v TemplateVars) Weekday() string {
	return v.IncidentTime.Format("Mon")
}

// Date returns incident date + 1 day (YYYYMMDD format).
// Log rotation typically names files by the date they were rotated,
// not the date of the logs. Logs from 05-02 get rotated into
// repmgrd.log-20260503.gz on 05-03.
func (v TemplateVars) Date() string {
	return v.IncidentTime.AddDate(0, 0, 1).Format("20060102")
}

func (c *Config) SSHCommands(vars TemplateVars) ([]CommandItem, error) {
	var result []CommandItem
	for _, cmd := range c.SSH.Commands {
		rendered, err := render(cmd.Command, vars)
		if err != nil {
			return nil, err
		}
		result = append(result, CommandItem{Name: cmd.Name, Command: rendered})
	}
	return result, nil
}

type weekdayVars struct {
	TemplateVars
	overrideWeekday string
}

func (w weekdayVars) Weekday() string { return w.overrideWeekday }

func (c *Config) SSHFiles(vars TemplateVars) ([]FileItem, error) {
	var result []FileItem
	for _, f := range c.SSH.Files {
		// Files whose path templates against {{.Weekday}} are fanned out
		// across the incident weekday and (if different) today's weekday,
		// so investigations run a day after the incident still pull both
		// logs. Daily-rotated logs (e.g. postgresql-Mon.log) need this.
		if strings.Contains(f.Path, ".Weekday") {
			today := time.Now().Format("Mon")
			weekdays := []string{vars.Weekday()}
			if today != vars.Weekday() {
				weekdays = append(weekdays, today)
			}
			for i, wd := range weekdays {
				rendered, err := render(f.Path, weekdayVars{TemplateVars: vars, overrideWeekday: wd})
				if err != nil {
					return nil, err
				}
				optional := f.Optional
				if i > 0 {
					// Today's file may not exist yet on quiet systems.
					optional = true
				}
				result = append(result, FileItem{
					Name:     nameWithWeekday(f.Name, wd),
					Path:     rendered,
					Gzip:     f.Gzip,
					Optional: optional,
				})
			}
			continue
		}
		rendered, err := render(f.Path, vars)
		if err != nil {
			return nil, err
		}
		result = append(result, FileItem{
			Name:     f.Name,
			Path:     rendered,
			Gzip:     f.Gzip,
			Optional: f.Optional,
		})
	}
	return result, nil
}

// nameWithWeekday inserts the weekday before the file extension so the two
// rendered targets don't collide when written to disk:
// ("postgresql.log", "Mon") -> "postgresql-Mon.log".
func nameWithWeekday(name, wd string) string {
	if dot := strings.LastIndex(name, "."); dot > 0 {
		return name[:dot] + "-" + wd + name[dot:]
	}
	return name + "-" + wd
}

func (c *Config) KubectlCommands(vars TemplateVars) ([]CommandItem, error) {
	var result []CommandItem
	for _, cmd := range c.Kubectl.Commands {
		rendered, err := render(cmd.Command, vars)
		if err != nil {
			return nil, err
		}
		result = append(result, CommandItem{Name: cmd.Name, Command: rendered})
	}
	return result, nil
}

func (c *Config) KubectlVMIQuery(vars TemplateVars) (string, error) {
	return render(c.Kubectl.VMIQuery, vars)
}

func (c *Config) OpensearchQueries(vars TemplateVars) ([]QueryItem, error) {
	var result []QueryItem
	for _, f := range c.OpenSearch.Queries {
		rendered, err := render(f.Query, vars)
		if err != nil {
			return nil, err
		}
		result = append(result, QueryItem{
			Name:  f.Name,
			Index: f.Index,
			Query: rendered,
		})
	}
	return result, nil
}

func render(s string, v any) (string, error) {
	tmpl, err := template.New("cmd").Parse(s)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, v)
	return buf.String(), err
}
