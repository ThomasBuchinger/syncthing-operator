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
	syncthingclient "github.com/thomasbuchinger/syncthing-operator/pkg/syncthing-client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FolderSpec defines the desired state of Folder
type FolderSpec struct {

	// Embed Syncthing API info into FolderSpec.
	// This allows the operator to control an external Syncthing instance
	Clientconfig syncthingclient.StClientConfig `json:",inline"`

	Label             string   `json:"label"`
	SharedDeviceNames []string `json:"shared_devices"`
	Path              string   `json:"path,omitempty"`

	SharedDeviceIds []string `json:"shared_ids,omitempty"`
	IgnorePattern   []string `json:"ignore_pattern,omitempty"`
	Type            string   `json:"type,omitempty"`
	FilesystemType  string   `json:"filesystem_type,omitempty"`
	Order           string   `json:"order,omitempty"`
	IgnorePerms     bool     `json:"ignore_permissions,omitempty"`
	IgnoreDelete    bool     `json:"ignore_delete,omitempty"`
	Paused          bool     `json:"paused,omitempty"`

	//+kubebuilder:default:=-1
	RescanInterval int `json:"rescan_interval,omitempty"`
}

// FolderStatus defines the observed state of Folder
type FolderStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// Folder is the Schema for the folders API
type Folder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FolderSpec   `json:"spec,omitempty"`
	Status FolderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FolderList contains a list of Folder
type FolderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Folder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Folder{}, &FolderList{})
}
