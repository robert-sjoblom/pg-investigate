package opensearch

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/opensearch-project/opensearch-go/v2"
	"golang.org/x/term"
)

type Client struct {
	client *opensearch.Client
}

// PromptPassword reads an OpenSearch password from the terminal. Call once at
// startup and pass the result to each Connect() to avoid prompting per-target.
func PromptPassword() (string, error) {
	fmt.Print("OpenSearch password: ")
	pw, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(pw), nil
}

func Connect(addresses []string, username, password, caCertPath string) (*Client, error) {
	var caCert []byte
	var err error
	if caCertPath != "" {
		caCert, err = os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
	}

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: addresses,
		Username:  username,
		Password:  password,
		CACert:    caCert,
	})
	if err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

// DiscoverHosts returns distinct kubernetes.host values where any pod whose
// name starts with podPrefix logged anything in the time window. Used to
// find every harvester node a VM lived on, including before failover/migration.
func (c *Client) DiscoverHosts(index, podPrefix, since, until string) ([]string, error) {
	query := fmt.Sprintf(`{
  "size": 0,
  "query": {
    "bool": {
      "must": [
        {"prefix": {"kubernetes.pod_name.keyword": %q}},
        {"range": {"@timestamp": {"gte": %q, "lte": %q}}}
      ]
    }
  },
  "aggs": {
    "hosts": {"terms": {"field": "kubernetes.host.keyword", "size": 20}}
  }
}`, podPrefix, since, until)

	res, err := c.client.Search(
		c.client.Search.WithIndex(index),
		c.client.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Aggregations struct {
			Hosts struct {
				Buckets []struct {
					Key string `json:"key"`
				} `json:"buckets"`
			} `json:"hosts"`
		} `json:"aggregations"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse discovery response: %w", err)
	}

	var hosts []string
	for _, b := range parsed.Aggregations.Hosts.Buckets {
		hosts = append(hosts, b.Key)
	}
	return hosts, nil
}

func (c *Client) Search(index, query string, w io.Writer) error {
	res, err := c.client.Search(
		c.client.Search.WithIndex(index),
		c.client.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, err = io.Copy(w, res.Body)
	return err
}
