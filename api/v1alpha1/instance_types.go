/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InstanceSpec struct {
	// AutoUpdate bool   `json:"auto_update"`
	ImageName string `json:"image,omitempty"`
	Tag       string `json:"tag,omitempty"`
	SyncPort  int    `json:"sync_port,omitempty"`

	ApiKey string `json:"apikey"`
	TlsCrt string `json:"tls_crt,omitempty"`
	TlsKey string `json:"tls_key,omitempty"`
	// SyncthingTlsConfig corev1.SecretReference `json:"syncthing_tls,omitempty"`

	AllowUsageReport bool                   `json:"allow_usage_report,omitempty"`
	HttpsTlsConfig   corev1.SecretReference `json:"https_tls,omitempty"`

	ConfigVolume corev1.Volume   `json:"config_volume,omitempty"`
	DataVolumes  []corev1.Volume `json:"data_volumes,omitempty"`

	DataPath      string `json:"-"`
	ConfigPath    string `json:"-"`
	ContainerName string `json:"-"`
	TlsConfigName string `json:"-"`
}

// InstanceStatus defines the observed state of Instance
type InstanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Instance is the Schema for the instances API
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec,omitempty"`
	Status InstanceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InstanceList contains a list of Instance
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}
