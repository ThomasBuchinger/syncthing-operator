package controllers

import (
	"context"
	"fmt"
	"strings"

	syncthingv1 "github.com/thomasbuchinger/syncthing-operator/api/v1"
	syncthingclient "github.com/thomasbuchinger/syncthing-operator/pkg/syncthing-client"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func fillDefaultValues(i *syncthingv1.Instance) error {
	// Not configureable via CRD
	i.Spec.DataPath = "/var/syncthing/"
	i.Spec.ConfigPath = "/etc/syncthing/"
	i.Spec.ContainerName = "syncthing"
	i.Spec.TlsConfigName = "tls-config"

	// Set default values for CustomResource
	if i.Spec.ImageName == "" {
		i.Spec.ImageName = "docker.io/syncthing/syncthing"
	}
	if i.Spec.Tag == "" {
		i.Spec.Tag = "latest"
	}
	if i.Spec.SyncPort == 0 {
		i.Spec.SyncPort = 32000
	}
	default_volume := corev1.Volume{Name: "", VolumeSource: corev1.VolumeSource{}} // same as golangs default-initialized Volume
	if i.Spec.ConfigVolume == default_volume {
		i.Spec.ConfigVolume = corev1.Volume{
			Name: "config-root",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
	if i.Spec.DataRoot == default_volume {
		i.Spec.DataRoot = corev1.Volume{
			Name: "data-root",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
	default_quantity := resource.Quantity{}
	if i.Spec.MaxReceiveSpeed == default_quantity {
		i.Spec.MaxReceiveSpeedValue = 0
	} else {
		i.Spec.MaxReceiveSpeedValue = i.Spec.MaxReceiveSpeed.ScaledValue(resource.Kilo)
	}
	if i.Spec.MaxSendSpeed == default_quantity {
		i.Spec.MaxSendSpeedValue = 0
	} else {
		i.Spec.MaxSendSpeedValue = i.Spec.MaxSendSpeed.ScaledValue(resource.Kilo)
	}
	return nil
}

func commonSyncthingLabels(name string) map[string]string {
	return map[string]string{"app.kubernetes.io/app": "syncthing", "syncthing.buc.sh/cr": name}
}

func getContainerIndexByName(list []corev1.Container, name string) int {
	for i, v := range list {
		if v.Name == name {
			return i
		}
	}
	return -1
}

func getVolumeIndexByName(list []corev1.Volume, name string) int {
	for i, v := range list {
		if v.Name == name {
			return i
		}
	}
	return -1
}

func getVolumeMountIndexByName(list []corev1.VolumeMount, name string) int {
	for i, v := range list {
		if v.Name == name {
			return i
		}
	}
	return -1
}

func FindSecretByLabel(ns string, label string, c interface{ client.Client }, ctx context.Context) (*corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	err := c.List(ctx, secretList, client.InNamespace(ns), client.HasLabels{label})
	if err != nil && errors.IsNotFound(err) {
		// Finding nothing isn't a problem
		return nil, nil
	}
	if err != nil {
		// Return error
		return nil, err
	}
	if len(secretList.Items) != 1 {
		return nil, fmt.Errorf("found %d secrets with '%s'-label", len(secretList.Items), syncthingclient.StClientSyncTlsLabel)
	}
	return &secretList.Items[0], nil
}

// Set command-parameter for container
func setCommandParameter(paramName string, paramValue string, container *corev1.Container) bool {
	// Convert string-array to map
	cmdAsMap := make(map[string]string, len(container.Command)+1)
	var sep, name, value string
	for _, part := range container.Command {
		sep = " "
		if strings.Contains(part, "=") {
			sep = "="
		}
		splitted := strings.SplitN(part, sep, 2)
		name = splitted[0]
		if len(splitted) == 2 {
			value = splitted[1]
		} else {
			value = ""
		}
		cmdAsMap[name] = value
	}

	// return false if parameter is already set
	old_value, exists := cmdAsMap[paramName]
	if exists && old_value == paramValue {
		return false
	}

	// Update parameter && regenerate command array
	cmdAsMap[paramName] = paramValue
	new_command := make([]string, len(cmdAsMap))
	for name, val := range cmdAsMap {
		if val != "" {
			new_command = append(new_command, name+" "+val)
		} else {
			new_command = append(new_command, name)
		}
	}
	container.Command = new_command
	return true
}

// Mount the secret as a Volume in the deployment
func generateTlsVolumeAndMount(instanceCr *syncthingv1.Instance, secret *corev1.Secret, deployment *appsv1.Deployment) *appsv1.Deployment {
	tlsVolume := corev1.Volume{
		Name: instanceCr.Spec.TlsConfigName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secret.Name,
			},
		},
	}
	keyMount := corev1.VolumeMount{
		Name:      tlsVolume.Name,
		MountPath: instanceCr.Spec.ConfigPath + "key.pem",
		SubPath:   "tls.key",
	}
	certMount := corev1.VolumeMount{
		Name:      tlsVolume.Name,
		MountPath: instanceCr.Spec.ConfigPath + "cert.pem",
		SubPath:   "tls.crt",
	}

	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, tlsVolume)
	for index := range deployment.Spec.Template.Spec.Containers {
		if -1 == getVolumeMountIndexByName(deployment.Spec.Template.Spec.Containers[index].VolumeMounts, tlsVolume.Name) {
			deployment.Spec.Template.Spec.Containers[index].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[index].VolumeMounts, keyMount, certMount)
		}
	}
	for index := range deployment.Spec.Template.Spec.InitContainers {
		if -1 == getVolumeMountIndexByName(deployment.Spec.Template.Spec.InitContainers[index].VolumeMounts, tlsVolume.Name) {
			deployment.Spec.Template.Spec.InitContainers[index].VolumeMounts = append(deployment.Spec.Template.Spec.InitContainers[index].VolumeMounts, keyMount, certMount)
		}
	}
	return deployment
}

// MountConfigs for all persistent volumes
func generateVolumeMountConfigs(instanceCr *syncthingv1.Instance, volumes []corev1.Volume) map[string]corev1.VolumeMount {
	mounts := map[string]corev1.VolumeMount{}
	for _, volume := range volumes {
		templateMount := corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: instanceCr.Spec.DataPath + volume.Name + "/",
		}

		if volume.Name == "config-root" {
			templateMount.MountPath = instanceCr.Spec.ConfigPath
		}
		if volume.Name == "data-root" {
			templateMount.MountPath = instanceCr.Spec.DataPath
		}
		mounts[volume.Name] = templateMount
	}

	return mounts
}

// TLS certificates for sync protocol
func generateSyncSecret(instanceCr *syncthingv1.Instance) *corev1.Secret {
	secretLabels := commonSyncthingLabels(instanceCr.Name)
	secretLabels[syncthingclient.StClientSyncTlsLabel] = "pem"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceCr.Name + "-id",
			Namespace: instanceCr.Namespace,
			Labels:    secretLabels,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.key": []byte(instanceCr.Spec.TlsKey),
			"tls.crt": []byte(instanceCr.Spec.TlsCrt),
		},
	}

	// Combine Clientconfig with SyncSecret, if configured in CustomResource
	if instanceCr.Spec.Clientconfig.ApiKey != "" {
		secretLabels[syncthingclient.StClientConfigLabel] = "plain"
		secret.Data["url"] = []byte(instanceCr.Spec.Clientconfig.ApiUrl)
		secret.Data["apikey"] = []byte(instanceCr.Spec.Clientconfig.ApiKey)
	}

	return secret
}

// Base Deployment Object
func generateDeployment(instanceCr *syncthingv1.Instance) *appsv1.Deployment {
	labels := commonSyncthingLabels(instanceCr.Name)
	image := instanceCr.Spec.ImageName
	tag := instanceCr.Spec.Tag
	var replicas int32 = 1
	reqCpu, _ := resource.ParseQuantity("10m")
	reqMem, _ := resource.ParseQuantity("50M")
	limitCpu, _ := resource.ParseQuantity("2")
	limitMem, _ := resource.ParseQuantity("2G")

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceCr.Name,
			Namespace: instanceCr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: "Recreate",
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           image + ":" + tag,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Name:            instanceCr.Spec.ContainerName,
						Command: []string{
							"syncthing",
							"serve",
							"--config" + " " + instanceCr.Spec.ConfigPath,
							"--data" + " " + instanceCr.Spec.DataPath,
						},
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 8384,
								Name:          "http",
							}, {
								ContainerPort: 22000,
								Name:          "sync",
								Protocol:      corev1.ProtocolUDP,
							}, {
								ContainerPort: 22000,
								Name:          "sync-tcp",
								Protocol:      corev1.ProtocolTCP,
							}, {
								ContainerPort: 21027,
								Name:          "discovery",
							},
						},
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/rest/system/ping",
									Port: intstr.FromString("http"),
									HTTPHeaders: []corev1.HTTPHeader{
										{Name: "X-API-Key", Value: instanceCr.Spec.Clientconfig.ApiKey},
									},
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    reqCpu,
								corev1.ResourceMemory: reqMem,
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    limitCpu,
								corev1.ResourceMemory: limitMem,
							},
						},
					}},
				},
			},
		},
	}

}

// Generate a ClusterIP Service, but leave exposing syncthing to the user
func generateClusterService(instanceCr *syncthingv1.Instance) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceCr.Name + "-web",
			Namespace: instanceCr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: commonSyncthingLabels(instanceCr.Name),
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8384,
				TargetPort: intstr.FromString("http"),
				Protocol:   corev1.ProtocolTCP,
			}, {
				Name:       "sync",
				TargetPort: intstr.FromString("sync"),
				Port:       int32(instanceCr.Spec.SyncPort),
				Protocol:   corev1.ProtocolUDP,
			}},
		},
	}
}

// Optional: Generate a NodePort to expose syncthing to the outside world
func generateNodeportService(instanceCr *syncthingv1.Instance) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceCr.Name + "-nodeport",
			Namespace: instanceCr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: commonSyncthingLabels(instanceCr.Name),
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8384,
				NodePort:   32001,
				TargetPort: intstr.FromString("http"),
				Protocol:   corev1.ProtocolTCP,
			}, {
				Name:       "sync",
				TargetPort: intstr.FromString("sync"),
				Port:       int32(instanceCr.Spec.SyncPort),
				NodePort:   int32(instanceCr.Spec.SyncPort),
				Protocol:   corev1.ProtocolUDP,
			}, {
				Name:       "sync-tcp",
				TargetPort: intstr.FromString("sync"),
				Port:       int32(instanceCr.Spec.SyncPort),
				NodePort:   int32(instanceCr.Spec.SyncPort),
				Protocol:   corev1.ProtocolTCP,
			}, {
				Name:       "discovery",
				TargetPort: intstr.FromString("discovery"),
				Port:       int32(instanceCr.Spec.DiscoveryPort),
				NodePort:   int32(instanceCr.Spec.DiscoveryPort),
				Protocol:   corev1.ProtocolUDP,
			},
			},
		},
	}
}
