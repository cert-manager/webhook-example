package dynuclient

import "net/http"

// DNSRecord ...
type DNSRecord struct {
	NodeName   string `json:"nodeName"`
	RecordType string `json:"recordType"`
	TextData   string `json:"textData"`
	TTL        string `json:"ttl"`
	DomainID   int    `json:"domainId"`
	State      bool   `json:"state"`
}

// DNSResponse ...
type DNSResponse struct {
	StatusCode int    `json:"statusCode"`
	ID         int    `json:"id"`
	DomainID   int    `json:"domainId"`
	DomainName string `json:"domainName"`
	NodeName   string `json:"nodeName"`
	Hostname   string `json:"hostname"`
	RecordType string `json:"recordType"`
	TTL        int16  `json:"ttl"`
	State      bool   `json:"state"`
	Content    string `json:"content"`
	UpdatedOn  string `json:"updatedOn"`
	TextData   string `json:"textData"`
}

// DynuClient ... options for DynuClient
type DynuClient struct {
	HTTPClient  *http.Client
	DNSID       string
	DNSRecordID int
	UserAgent   string
	APIKey      string
}

// DynuCreds - Details required to access API
type DynuCreds struct {
	APIKey string
	DNSID  string
}
