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

package v1

import (
	syncthingclient "github.com/thomasbuchinger/syncthing-operator/pkg/syncthing-client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CRD Spec forSyncthing Device: /api/system/config/devices
type DeviceSpec struct {
	// Embed Syncthing API info into DeviceSpec.
	// This allows the operator to control an external Syncthing instance
	Clientconfig syncthingclient.StClientConfig `json:",inline"`

	// Syncthing DeviceID
	//+kubebuilder:validation:Pattern=`[A-Z][7]([A-Z\-]{7}){7}`
	DeviceId string `json:"id"`

	// Automatically Accept all Folders shared from this Device
	//+kubebuilder:default:=true
	AutoAcceptFolders bool `json:"auto_accept,omitempty"`
	// Pause syncronization with this Device
	Paused bool `json:"paused,omitempty"`
	// Set an untrusted password.
	// Data is encrypted with this password, so the receiver cannot read it
	Untrusted string `json:"untrusted,omitempty"`

	IgnoredFolders []string `json:"ignored_folders,omitempty"`
}

type DeviceStatus struct {
	// TODO: Add information from/rst/system/connections
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceSpec   `json:"spec,omitempty"`
	Status DeviceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}
