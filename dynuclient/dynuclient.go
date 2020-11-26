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
	dnsURL := fmt.Sprintf("%s/dns/%s/record", dynuAPI, c.DNSID)
	body, err := json.Marshal(records)
	if err != nil {
		return -1, err
	}
	var resp *http.Response

	resp, err = c.makeRequest(dnsURL, "POST", bytes.NewReader(body))
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
	dnsURL := fmt.Sprintf("%s/dns/%s/record/%d", dynuAPI, c.DNSID, DNSRecordID)
	var resp *http.Response

	resp, err := c.makeRequest(dnsURL, "DELETE", nil)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf(resp.Status)
	}
	return nil
}

func (c *DynuClient) makeRequest(URL string, method string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, URL, body)
	if err != nil {
		return nil, err
	}

	req.Header["accept"] = []string{"application/json"}
	req.Header["User-Agent"] = []string{c.UserAgent}
	req.Header["Content-Type"] = []string{"application/json"}
	req.Header["API-Key"] = []string{c.APIKey}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}

	c.HTTPClient.Timeout = 30 * time.Second

	return c.HTTPClient.Do(req)
}

func (c *DynuClient) decodeBytes(input []byte) (string, error) {

	buf := new(strings.Builder)
	_, err := io.Copy(buf, bytes.NewBuffer(input))
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
