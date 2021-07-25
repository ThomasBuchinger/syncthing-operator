package syncthingclient

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type StClient struct {
	BaseUrl    url.URL
	ApiKey     string
	HttpClient http.Client
}

func SetupSyncthingClient() *StClient {
	c := new(StClient)
	c.BaseUrl = GetSyncthingDefaultUrl()
	c.ApiKey = ""
	c.HttpClient = *http.DefaultClient
	return c
}
func GetSyncthingDefaultUrl() url.URL {
	return url.URL{Scheme: "http", Host: "syncthing.svc.cluster.local:8384"}
}

func (c *StClient) newRequestTemplate(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.BaseUrl.ResolveReference(rel)
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.ApiKey)
	return req, nil
}
func (c *StClient) do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
	}
	return resp, err
}
