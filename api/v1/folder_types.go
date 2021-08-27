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

//+kubebuilder:validation:Enum=sendonly;receiveonly;sendreceive
type FolderTypeEnum string

const (
	TypeSendOnly    FolderTypeEnum = "sendonly"
	TypeReceiveOnly FolderTypeEnum = "receiveonly"
	TypeSendReceive FolderTypeEnum = "sendreceive"
)

//+kubebuilder:validation:Enum=random;alphabetic;smallestFirst;largestFirst;newestFirst;oldestFirst
type FolderOrderEnum string

const (
	OrderRandom        FolderOrderEnum = "random"
	OrderAlphabetic    FolderOrderEnum = "alphabetic"
	OrderSmallestFirst FolderOrderEnum = "smallestFirst"
	OrderLargestFirst  FolderOrderEnum = "largestFirst"
	OrderNewestFirst   FolderOrderEnum = "newestFirst"
	OrderOldestFirst   FolderOrderEnum = "oldestFirst"
)

// Syncthing folder: /api/system/config/folder
type FolderSpec struct {

	// Embed Syncthing API info into FolderSpec.
	// This allows the operator to control an external Syncthing instance
	Clientconfig syncthingclient.StClientConfig `json:",inline"`
	// Human-readable name for the folder
	Label string `json:"label"`
	// Share this Folder with Devices. This matches the human-readable device names (not their unique ID)
	SharedDeviceNames []string `json:"shared_devices"`
	// Path to Folder in the container use. Defaults to /var/syncthing/<label>
	//+kubebuilder:validation:Pattern=`/var/syncting/.+`
	Path string `json:"path,omitempty"`
	// Share this folder with these IDs.
	SharedDeviceIds []string `json:"shared_ids,omitempty"`

	// Helper booleans

	// Use the CustomResource name as FolderId (set to true) or let syncthing generate an id (set to false)
	//TODO: implement
	// This way 2 operator controlled instances can start syncing folders, without manually accepting shares (or set a device to AutoAccept)
	//+kubebuilder:default:=false
	UseNameAsId bool `json:"use_name_as_id,omitempty"`

	// Exposed syncthing settings

	// Set allowed synchronization direction
	//+kubebuilder:default="sendreceive"
	Type FolderTypeEnum `json:"type,omitempty"`
	// Set the order in which to synchronize files
	//+kubebuilder:default:="random"
	Order FolderOrderEnum `json:"order,omitempty"`
	// Do not synchronize Permissions
	//+kubebuilder:default:=false
	IgnorePerms bool `json:"ignore_permissions,omitempty"`
	// Pause synchronization for this folder
	//+kubebuilder:default:=false
	Paused bool `json:"paused,omitempty"`
	// Set interval between full checks for changed files. This is only for files not picked up immediatly by fsWatcher
	// -1 Uses a default value. 0 disables rescans
	//+kubebuilder:default:=-1
	//+kubebuilder:validation:Minimum:=-1
	RescanInterval int `json:"rescan_interval,omitempty"`
	// Controlls location of .stfolder-marker. Changing this is only necessary, when you want to share a readonly filesystem
	//+kubebuilder:default:=".stfolder"
	StMarker string `json:"stMarker,omitempty"` // For experienced Users. handle MarkerName property yourself
}

type FolderStatus struct {
	// TODO: Add Folder status
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
