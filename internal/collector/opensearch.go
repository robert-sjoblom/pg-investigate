package collector

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fortnox/pg-investigate/internal/config"
	"github.com/fortnox/pg-investigate/internal/opensearch"
)

type OpenSearchCollector struct {
	client  *opensearch.Client
	queries []config.QueryItem
}

func NewOpenSearch(client *opensearch.Client, queries []config.QueryItem) *OpenSearchCollector {
	return &OpenSearchCollector{
		client:  client,
		queries: queries,
	}
}

func (c *OpenSearchCollector) Collect(outputDir string) error {
	for _, q := range c.queries {
		fmt.Printf("Running OpenSearch query: %s\n", q.Name)
		dest := filepath.Join(outputDir, q.Name)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		err = c.client.Search(q.Index, q.Query, f)
		f.Close()
		if err != nil {
			fmt.Printf("FAILED: %s\nError: %s\n", q.Name, err)
			return err
		}
		fmt.Printf("  -> %s\n", dest)
	}
	return nil
}
