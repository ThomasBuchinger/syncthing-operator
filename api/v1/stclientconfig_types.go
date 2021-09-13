package v1

import (
	corev1 "k8s.io/api/core/v1"
)

// Optional in CustomResource definition of synyncthing instance. Shared by all CRDs
type StClientConfig struct {
	//+kubebuilder:default:=""
	ApiUrl string `json:"url,omitempty"`
	//+kubebuilder:default:=""
	ApiKey string `json:"apikey,omitempty"`

	// If set: Use this secret, do not search NS
	ConfigSecret *corev1.Secret `json:"-"`
}
