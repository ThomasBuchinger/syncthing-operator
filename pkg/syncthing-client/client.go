package syncthingclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const StClientConfigLabel string = "syncthing.buc.sh/config"
const StClientSyncTlsLabel = "syncthing.buc.sh/sync-cert"
const StClientHttpsLabel = "syncthing.buc.sh/https-cert"
const StClientKeyUrl = "url"
const StClientKeyApiKey = "apikey"

// Optional in CustomResource definition of synyncthing instance. Shared by all CRDs
type StClientConfig struct {
	//+kubebuilder:default:="http://syncthing.svc.cluster.local:8384"
	ApiUrl string `json:"url,omitempty"`
	//+kubebuilder:default:=""
	ApiKey string `json:"apikey,omitempty"`
}

// Syncthing Client Object
type StClient struct {
	BaseUrl    url.URL
	ApiKey     string
	HttpClient http.Client
	logger     logr.Logger
}

// Create a Client from either the given CustomResource or search the given namespace
func FromCr(cr StClientConfig, ns string, client interface{ client.Client }, ctx context.Context) (*StClient, error) {
	url_string, key := "http://syncthing.svc.cluster.local:8384", ""
	logger := log.FromContext(ctx)
	config_valid := struct{ Url, Key bool }{Url: false, Key: false}

	// CustomResource Config has highest precedence
	if cr.ApiUrl != "" {
		url_string = cr.ApiUrl
		config_valid.Url = true
	}
	if cr.ApiKey != "" {
		key = cr.ApiKey
		config_valid.Key = true
	}

	// Next look for a Config in the Namespace
	if !(config_valid.Url && config_valid.Key) {
		logger.V(1).Info("Searching for connection secret in namespace: " + ns)
		ns_url, ns_key, err := searchNamespace(ns, client, ctx)
		if err != nil {
			logger.V(1).Info("Error quering Secret: %s", err.Error())
		} else {
			// If both values are empty, either no secret was found or the secret did not have matching keys
			if ns_url != "" && !config_valid.Url {
				url_string = ns_url
				config_valid.Url = true
			}
			if ns_key != "" && !config_valid.Key {
				key = ns_key
				config_valid.Key = true
			}
		}
	}

	// Use default value for URL
	if !config_valid.Url {
		url_string = "http://syncthing.svc.cluster.local:8384"
		config_valid.Url = true
	}

	// Build StClient
	if !(config_valid.Url && len(url_string) != 0) {
		return nil, fmt.Errorf("API URL is empty")
	}
	if !(config_valid.Key && len(key) == 0) {
		return nil, fmt.Errorf("no API key found")
	}
	apiurl, err := url.Parse(url_string)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %s", err.Error())
	}

	c := new(StClient)
	c.HttpClient = http.Client{}
	c.logger = logger
	c.BaseUrl = *apiurl
	c.ApiKey = key
	return c, nil
}

// search namespace for a secret containing an API endpoint
func searchNamespace(ns string, c interface{ client.Client }, ctx context.Context) (string, string, error) {
	secrets := &corev1.SecretList{}
	err := c.List(ctx, secrets, client.InNamespace(ns), client.HasLabels{StClientConfigLabel})
	if err != nil && errors.IsNotFound(err) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("error quering Secret: %s", err.Error())
	}
	if len(secrets.Items) != 1 {
		return "", "", fmt.Errorf("found %d secrets with '%s'-label", len(secrets.Items), StClientConfigLabel)
	}
	apiurl := secrets.Items[0].StringData["url"]
	apikey := secrets.Items[0].StringData["apikey"]

	return apiurl, apikey, nil
}

// helper method for http requests
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

// helper method for http requests
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
