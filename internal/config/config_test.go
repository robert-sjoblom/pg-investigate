package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	cfg, err := LoadFrom("testdata/config.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SSH.User != "root" {
		t.Errorf("expected root, got %s", cfg.SSH.User)
	}
}

func TestTemplateSubstitution(t *testing.T) {
	cfg, err := LoadFrom("testdata/config.yaml")
	if err != nil {
		t.Fatal(err)
	}

	incidentTime := time.Date(2026, 5, 2, 4, 35, 0, 0, time.Local)
	vars := TemplateVars{
		IncidentTime: incidentTime,
		Since:        "2026-05-02 03:35",
		Until:        "2026-05-02 05:35",
	}

	cmds, err := cfg.SSHCommands(vars)
	if err != nil {
		t.Fatal(err)
	}

	if len(cmds) == 0 {
		t.Fatal("expected commands")
	}
}

func TestSSHFiles(t *testing.T) {
	cfg, _ := LoadFrom("testdata/config.yaml")
	incidentTime := time.Date(2026, 5, 1, 4, 35, 0, 0, time.Local) // Friday
	vars := TemplateVars{IncidentTime: incidentTime, PgVersion: "17"}

	files, err := cfg.SSHFiles(vars)
	if err != nil {
		t.Fatal(err)
	}

	// Check that Weekday was substituted
	if files[0].Path != "/var/lib/pgsql/17/data/log/postgresql-Fri.log" {
		t.Errorf("unexpected path: %s", files[0].Path)
	}
}

func TestOpensearchQueries(t *testing.T) {
	cfg, err := LoadFrom("testdata/config.yaml")
	if err != nil {
		t.Fatal(err)
	}

	vars := TemplateVars{
		Since: "2026-05-02 03:35:00",
		Until: "2026-05-02 05:35:00",
	}

	queries, err := cfg.OpensearchQueries(vars)
	if err != nil {
		t.Fatal(err)
	}

	if len(queries) == 0 {
		t.Fatal("expected queries")
	}

	if queries[0].Name != "vm-logs.json" {
		t.Errorf("unexpected name: %s", queries[0].Name)
	}

	if queries[0].Index != "logs-*" {
		t.Errorf("unexpected index: %s", queries[0].Index)
	}

	// Check that Since/Until were substituted
	expected := `{"query": {"range": {"@timestamp": {"gte": "2026-05-02 03:35:00", "lte": "2026-05-02 05:35:00"}}}}`
	// Trim whitespace for comparison
	if queries[0].Query != expected+"\n" {
		t.Errorf("unexpected query:\ngot:  %q\nwant: %q", queries[0].Query, expected)
	}
}
