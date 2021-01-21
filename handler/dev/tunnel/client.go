package tunnel

import (
	"context"
	"fmt"
	"github.com/inlets/inlets/pkg/transport"
	"github.com/rancher/remotedialer"
	"github.com/twinj/uuid"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// Client for inlets
type Client struct {
	// Remote site for websocket address
	Remote string

	// Map of upstream servers dns.entry=http://ip:port
	UpstreamMap map[string]string

	// Token for authentication
	Token string

	// StrictForwarding
	StrictForwarding bool
}

func makeAllowsAllFilter() func(network, address string) bool {
	return func(network, address string) bool {
		return true
	}
}

func makeFilter(upstreamMap map[string]string) func(network, address string) bool {

	trimmedMap := map[string]bool{}

	for _, v := range upstreamMap {
		u, err := url.Parse(v)
		if err != nil {
			log.Printf("Error parsing: %s, skipping.\n", v)
			continue
		}

		trimmedMap[u.Host] = true
	}

	return func(network, address string) bool {
		if network != "tcp" {
			log.Printf("network not allowed: %q\n", network)

			return false
		}

		if ok, v := trimmedMap[address]; ok && v {
			return true
		}

		return false
	}
}

// Connect connect and serve traffic through websocket
func (c *Client) Connect() error {
	headers := http.Header{}
	headers.Set(transport.InletsHeader, uuid.Formatter(uuid.NewV4(), uuid.FormatHex))
	for k, v := range c.UpstreamMap {
		headers.Add(transport.UpstreamHeader, fmt.Sprintf("%s=%s", k, v))
	}
	if c.Token != "" {
		headers.Add("Authorization", "Bearer "+c.Token)
	}

	url := c.Remote
	if !strings.HasPrefix(url, "ws") {
		url = "ws://" + url
	}
	var filter func(network, address string) bool

	if c.StrictForwarding {
		filter = makeFilter(c.UpstreamMap)
	} else {
		filter = makeAllowsAllFilter()
	}

	for {
		remotedialer.ClientConnect(context.Background(), url+"/tunnel", headers, nil, filter, nil)
	}
}
