package opensearch

import (
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

func Connect(addresses []string, username, caCertPath string) (*Client, error) {
	fmt.Print("OpenSearch password: ")
	pw, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, err
	}
	fmt.Println()
	password := string(pw)

	var caCert []byte
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
