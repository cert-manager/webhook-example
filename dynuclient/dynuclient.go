package dynuclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const dynuAPI string = "https://api.dynu.com/v2"

var httpClient *http.Client

// CreateDNSRecord ... Create a DNS Record and return it's ID
//   POST https://api.dynu.com/v2/dns/{DNSID}/record
func (c *DynuClient) CreateDNSRecord(records DNSRecord) (int, error) {
	dnsURL := fmt.Sprintf("%s/dns/%d/record", dynuAPI, c.DNSID)
	body, err := json.Marshal(records)
	if err != nil {
		return -1, err
	}
	var resp *http.Response

	resp, err = c.MakeRequest(dnsURL, "POST", bytes.NewReader(body))
	if err != nil {
		return -1, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return -1, err
		}

		var dnsBody DNSResponse
		err = json.Unmarshal(bodyBytes, &dnsBody)
		if err != nil {
			return -1, err
		}
		return dnsBody.ID, nil
	}

	return -1, fmt.Errorf("%s received for %s", resp.Status, dnsURL)
}

// RemoveDNSRecord ... Removes a DNS record based on dnsRecordID
//   DELETE https://api.dynu.com/v2/dns/{DNSID}/record/{DNSRecordID}
func (c *DynuClient) RemoveDNSRecord(DNSRecordID int) error {
	dnsURL := fmt.Sprintf("%s/dns/%d/record/%d", dynuAPI, c.DNSID, DNSRecordID)
	var resp *http.Response

	resp, err := c.MakeRequest(dnsURL, "DELETE", nil)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf(resp.Status)
	}
	return nil
}

// MakeRequest ...
func (c *DynuClient) MakeRequest(URL string, method string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, URL, body)
	if err != nil {
		return nil, err
	}

	req.Header["accept"] = []string{"application/json"}
	req.Header["User-Agent"] = []string{"Mozilla/5.0 (X11; Linux x86_64; rv:82.0) Gecko/20100101 Firefox/82.0"} //c.UserAgent)
	req.Header["Content-Type"] = []string{"application/json"}
	req.Header["API-Key"] = []string{c.APISecret}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}

	c.getHTTPClient().Timeout = 30 * time.Second

	return c.getHTTPClient().Do(req)
}

// DecodeBytes ...
func (c *DynuClient) DecodeBytes(input []byte) (string, error) {

	buf := new(strings.Builder)
	_, err := io.Copy(buf, bytes.NewBuffer(input))
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (c *DynuClient) getHTTPClient() *http.Client {
	if httpClient != nil {
		return httpClient
	}
	if c.HTTPClient != nil {
		httpClient = c.HTTPClient
	} else {
		httpClient = &http.Client{}
	}
	return httpClient
}
