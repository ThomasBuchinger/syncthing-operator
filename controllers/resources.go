package controllers

import (
	syncthingv1alpha1 "github.com/thomasbuchinger/syncthing-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func fillDefaultValues(i *syncthingv1alpha1.Instance) error {
	if i.Spec.ImageName == "" {
		i.Spec.ImageName = "docker.io/syncthing/syncthing"
	}
	if i.Spec.Tag == "" {
		i.Spec.Tag = "latest"
	}

	return nil
}
func labelsForSyncthing(name string) map[string]string {
	return map[string]string{"k8s-app": "syncthing", "syncthing_cr": name}
}

func getContainerIndexByName(list []corev1.Container, name string) int {
	for i, v := range list {
		if v.Name == name {
			return i
		}
	}
	return -1
}

func getTlsVolumeIndexByName(list []corev1.Volume, name string) int {
	for i, v := range list {
		if v.Name == name {
			return i
		}
	}
	return -1
}

// TLS config for Syncthing
func tlsSecretForSyncthing(syncthing_cr *syncthingv1alpha1.Instance) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      syncthing_cr.Name + "-id",
			Namespace: syncthing_cr.Namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"apikey":  []byte(syncthing_cr.Spec.ApiKey),
			"tls.key": []byte(syncthing_cr.Spec.TlsKey),
			"tls.crt": []byte(syncthing_cr.Spec.TlsCrt),
		},
	}

	// Set Memcached instance as the owner and controller
	// ctrl.SetControllerReference(syncthing_cr, secret, r.Scheme)
	return secret
}

// deploymentForMemcached returns a memcached Deployment object
func deploymentForSyncthing(syncthing_cr *syncthingv1alpha1.Instance) *appsv1.Deployment {
	labels := labelsForSyncthing(syncthing_cr.Name)
	image := syncthing_cr.Spec.ImageName
	tag := syncthing_cr.Spec.Tag
	var replicas int32 = 1

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      syncthing_cr.Name,
			Namespace: syncthing_cr.Namespace,
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
						Image: image + ":" + tag,
						Name:  "syncthing",
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 8384,
								Name:          "http",
							},
							{
								ContainerPort: 22000,
								HostPort:      22000,
								Name:          "sync",
							},
						},
					}},
				},
			},
		},
	}
	return dep
}
