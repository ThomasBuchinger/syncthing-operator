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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InstanceSpec struct {

	// Enable deployment of Kubernetes Resources to run Syncthing in Kubernetes
	//+kubebuikder:default:=true
	EnableInstance bool `json:"enable_instance,omitempty"`
	// Name of the syncthing  docker-image
	//+kubebuilder:default:="docker.io/syncthing/syncthing"
	ImageName string `json:"image,omitempty"`
	// Tag of the syncthing docker image
	//+kubebuilder:default:="latest"
	Tag string `json:"tag,omitempty"`
	// Expose Syncthings 22000-Port as Nodeport-Service
	//+kubebuilder:default:=0
	SyncPort int `json:"sync_port,omitempty"`

	// Embed Syncthing API info into InstanceSpec.
	// This allows the operator to control an external Syncthing instance
	Clientconfig syncthingclient.StClientConfig `json:",inline"`

	// Hardcode Syncthing Certificate and private key
	// This certificate is used to authenticate syncthing to other syncthing-instances. Use HttpsCrt/HttpsKey to configure HTTPS for the webinterface
	// If you don't want the operator to manage the tls-secret, add a Kubernetes Secret with label "api.syncthing.buc.sh", "cert.syncthing.buc.sh", "key.syncthing.buc.sh" respectively to manage those secrets yourself
	// If no Secret is found and nothing is specified, the operator will generate one for you.
	//+kubebuilder:validation:Pattern=`(-----BEGIN CERTIFICATE-----(\n|\r|\r\n)([0-9a-zA-Z\+\/=]{64}(\n|\r|\r\n))*([0-9a-zA-Z\+\/=]{1,63}(\n|\r|\r\n))?-----END CERTIFICATE-----)`
	TlsCrt string `json:"tls_crt,omitempty"`
	//+kubebuilder:validation:Pattern=`(-----BEGIN [A-Z\s]+ KEY-----(\n|\r|\r\n)([0-9a-zA-Z\+\/=]{64}(\n|\r|\r\n))*([0-9a-zA-Z\+\/=]{1,63}(\n|\r|\r\n))?-----END [A-Z ]+ KEY-----)`
	TlsKey string `json:"tls_key,omitempty"`

	// Configure a Volume to store the Data
	// This volume is mounted on /var/syncthing, where sync'ed data will be stored
	DataRoot corev1.Volume `json:"data_volume,omitempty"`
	// Configure additional Volumes to be mounted on /var/syncthing/<volumename>
	// This can be used to store specific on different Kubernetes Volumes (and therefore different storage backends)
	AdditionalDataVolumes []corev1.Volume `json:"additional_data,omitempty"`
	// Use a persistent volume for syncthings config directory.
	// This will preserve any manual changes to the configuration as long as the volume is accessible
	// If not set, a EmptyDir Volume is configured and any manual changes will be lost when the Pod is restarted
	ConfigVolume corev1.Volume `json:"config_volume,omitempty"`

	// Enable the operator to manage global syncthing options
	//+kubebuilder:default:=true
	EnableConfiguration bool `json:"enable_config,omitempty"`
	// Allow syncthing to send anonymous usage data
	//+kubebuilder:default:=false
	AllowUsageReport bool `json:"allow_usage_report,omitempty"`
	// Limit Upload Speed.
	// Measured in bytes, rather than bits
	MaxSendSpeed resource.Quantity `json:"max_send_speed,omitempty"`
	// Limit Download Speed to X bytes per second
	MaxReceiveSpeed resource.Quantity `json:"max_receive_speed,omitempty"`
	// Configure Syncthing Webinterface to be insecure
	// Disables HTTPS and disables Basic-Auth
	//+kubebuilder:default:=false
	InsecureWeb bool `json:"insecure_web,omitempty"`
	// Set name of the syncthing admin user.
	// The users password is set to the Apikey
	//+kubebuilder:default:="syncthing"
	AdminUser string `json:"admin_user,omitempty"`
	// Hardcode Syncthing Certificate for HTTPS (not yet implemented)
	// If you don't want the operator to manage the https certificat, add a Kubernetes Secret with label "https-cert.syncthing.buc.sh", "https-key.syncthing.buc.sh" respectively to manage those secrets yourself
	// If no Secret is found and nothing is specified, HTTPS will not be enabled
	//+kubebuilder:validation:Pattern=`(-----BEGIN CERTIFICATE-----(\n|\r|\r\n)([0-9a-zA-Z\+\/=]{64}(\n|\r|\r\n))*([0-9a-zA-Z\+\/=]{1,63}(\n|\r|\r\n))?-----END CERTIFICATE-----)`
	HttpsCrt string `json:"https_crt,omitempty"`
	//+kubebuilder:validation:Pattern=`(-----BEGIN [A-Z\s]+ KEY-----(\n|\r|\r\n)([0-9a-zA-Z\+\/=]{64}(\n|\r|\r\n))*([0-9a-zA-Z\+\/=]{1,63}(\n|\r|\r\n))?-----END [A-Z ]+ KEY-----)`
	HttpsKey string `json:"https_key,omitempty"`

	// Expose Syncthings 22000-port and the webui via Nodeports
	//+kubebuilder:default:=false
	EnableNodeport bool `json:"enable_nodeport,omitempty"`

	//+kubebuilder:default:="/var/syncthing"
	DataPath string `json:"-"`
	//+kubebuider:default:="/etc/syncthing"
	ConfigPath string `json:"-"`
	//+kubebuilder:default:="syncthing"
	ContainerName string `json:"-"`
	//+kubebuilder:default:="tls-config"
	TlsConfigName string `json:"-"`

	MaxSendSpeedValue    int64 `json:"-"`
	MaxReceiveSpeedValue int64 `json:"-"`

	// TrueNAS Settings not propperly integrated
	//+kubebuilder:default:=false
	TrueNas bool `json:"truenas,omitempty"`
	//+kubebuilder:default:=32027
	DiscoveryPort int `json:"discovery_port,omitempty"` //syncthing default=21027
}

// InstanceStatus defines the observed state of Instance
type InstanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

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
