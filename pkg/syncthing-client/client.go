package syncthingclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	syncthingv1 "github.com/thomasbuchinger/syncthing-operator/api/v1"
)

const StClientConfigLabel string = "syncthing.buc.sh/config"
const StClientLabelSyncTls = "syncthing.buc.sh/sync-cert"
const StClientLabelHttps = "syncthing.buc.sh/https-cert"
const StClienLabeltUrlDiscovery = "syncthing.buc.sh/url-discovery"
const StClientKeyUrl = "url"
const StClientKeyApiKey = "apikey"

// Syncthing Client Object
type StClient struct {
	BaseUrl    url.URL
	ApiKey     string
	HttpClient http.Client
	logger     logr.Logger
}

// Create a Client from either the given CustomResource or search the given namespace
func FromCr(cr syncthingv1.StClientConfig, ns string, client interface{ client.Client }, ctx context.Context) (*StClient, error) {
	logger := log.FromContext(ctx)
	config_valid := struct{ Url, Key bool }{Url: false, Key: false}
	var url_string, key string

	// CustomResource Config has highest precedence
	if cr.ApiUrl != "" {
		url_string = cr.ApiUrl
		config_valid.Url = true
	}
	if cr.ApiKey != "" {
		key = cr.ApiKey
		config_valid.Key = true
	}

	// Check if configSecret is set explicitly
	if cr.ConfigSecret != nil {
		logger.V(1).Info("Using configuration in Secret: " + cr.ConfigSecret.Name)
		url_bytes, ok := cr.ConfigSecret.Data[StClientKeyUrl]
		if ok && !config_valid.Key {
			url_string = string(url_bytes)
			config_valid.Url = true
		}
		key_bytes, ok := cr.ConfigSecret.Data[StClientKeyApiKey]
		if ok && !config_valid.Key {
			key = string(key_bytes)
			config_valid.Key = true
		}
	}

	// Next look for a Config in the Namespace
	if !(config_valid.Url && config_valid.Key) {
		logger.V(1).Info("Searching for connection secret in namespace: " + ns)
		secret, err := FindSecretByLabel(ns, StClientConfigLabel, client, ctx)
		if err != nil {
			// Log error. This is not (yet) fatal
			logger.V(1).Info("Error quering Secret: %s", err.Error())
		} else if secret == nil {
			logger.V(1).Info("No Secret with label '%s' found in namespace '%s'", StClientConfigLabel, ns)
		} else {
			url_bytes, ok := secret.Data[StClientKeyUrl]
			if ok && !config_valid.Url {
				url_string = string(url_bytes)
				config_valid.Url = true
			}
			key_bytes, ok := secret.Data[StClientKeyApiKey]
			if ok && !config_valid.Key {
				key = string(key_bytes)
				config_valid.Key = true
			}
		}
	}

	// Check Cluster-DNS for API
	if !config_valid.Url {
		svc, err := FindServiceByLabel(ns, StClienLabeltUrlDiscovery, "cluster-service", client, ctx)
		if svc != nil && err == nil {
			url_string = fmt.Sprintf("http://%s:%d", svc.Name, 8384) // Resolve Service-Name using Cluster-DNS
			config_valid.Url = true
		}
	}

	// Build StClient
	if !config_valid.Url {
		return nil, fmt.Errorf("API-URL not found")
	}
	if len(url_string) == 0 {
		return nil, fmt.Errorf("API URL is empty")
	}
	if !config_valid.Key {
		return nil, fmt.Errorf("API-Key not found")
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("API-Key is empty")
	}
	apiurl, err := url.Parse(url_string)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %s", err.Error())
	}

	c := new(StClient)
	c.HttpClient = http.Client{}
	c.logger = logger
	c.BaseUrl = *apiurl
	c.ApiKey = strings.Trim(key, "\n \t")
	return c, nil
}

func FindSecretByLabel(ns string, label string, c interface{ client.Client }, ctx context.Context) (*corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	err := c.List(ctx, secretList, client.InNamespace(ns), client.HasLabels{label})
	if err != nil && errors.IsNotFound(err) {
		// Finding nothing isn't a problem
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(secretList.Items) != 1 {
		// TODO: odd number of arguments passed as key-value pairs for logging
		return nil, fmt.Errorf("found %d secrets with '%s'-label", len(secretList.Items), label)
	}
	return &secretList.Items[0], nil
}

func FindServiceByLabel(ns string, label string, value string, c interface{ client.Client }, ctx context.Context) (*corev1.Service, error) {
	serviceList := &corev1.ServiceList{}
	err := c.List(ctx, serviceList, client.InNamespace(ns), client.MatchingLabels{StClienLabeltUrlDiscovery: value})
	if err != nil && errors.IsNotFound(err) {
		// Finding nothing isn't a problem
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(serviceList.Items) != 1 {
		return nil, fmt.Errorf("found %d services with '%s=%s'-label", len(serviceList.Items), label, value)
	}
	return &serviceList.Items[0], nil

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
