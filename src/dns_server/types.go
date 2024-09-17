package dns_server

import (
	"go.temporal.io/sdk/client"
)

type JSONDNSRecord struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type DNSRequest struct {
	SessionID  string
	DomainName string
	IPs        []string
}

type DNSResponse struct {
	SessionID  string
	DomainName string
	IPs        []string
}

type DNSHandler struct {
	TemporalClient client.Client
}
