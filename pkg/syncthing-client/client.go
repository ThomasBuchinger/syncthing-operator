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

type StClientConfig struct {
	//+kubebuilder:default:="http://syncthing.svc.cluster.local:8384"
	ApiUrl string `json:"url"`
	//+kubebuilder:default:=""
	ApiKey string `json:"apikey"`
}
type StClient struct {
	BaseUrl    url.URL
	ApiKey     string
	HttpClient http.Client
	logger     logr.Logger
}

const StClientConfigLabel string = "config.syncthing.buc.sh"

// func SetupSyncthingClient() *StClient {
// 	c := new(StClient)
// 	c.BaseUrl = GetSyncthingDefaultUrl()
// 	c.ApiKey = ""
// 	c.HttpClient = *http.DefaultClient
// 	return c
// }

func FromCr(cr StClientConfig, ns string, client interface{ client.Client }, ctx context.Context) (*StClient, error) {
	url_string, key := "http://syncthing.svc.cluster.local:8384", ""
	logger := log.FromContext(ctx)

	// Look for a Config in the Namespace
	ns_url, ns_key, err := searchNamespace(ns, client, ctx)
	if err != nil {
		logger.V(1).Info("Error quering Secret: %s", err.Error())
	} else {
		// If both values are empty, either no secret was found or the secret did not have matching keys
		if ns_url != "" {
			url_string = ns_url
		}
		if ns_key != "" {
			key = ns_key
		}
	}

	// CustomResource Config has highest precedence
	if cr.ApiUrl != "" {
		url_string = cr.ApiUrl
	}
	if cr.ApiKey != "" {
		key = cr.ApiKey
	}

	// Build StClient
	apiurl, err := url.Parse(url_string)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %s", err.Error())
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("no API key found")
	}

	c := new(StClient)
	c.HttpClient = http.Client{}
	c.logger = logger
	c.BaseUrl = *apiurl
	c.ApiKey = key
	return c, nil
}

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
