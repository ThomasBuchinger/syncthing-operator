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

package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	syncthingv1alpha1 "github.com/thomasbuchinger/syncthing-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// InstanceReconciler reconciles a Instance object
type InstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=syncthing.buc.sh,resources=instances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syncthing.buc.sh,resources=instances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syncthing.buc.sh,resources=instances/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Instance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.Log.WithName(req.Namespace + "/" + req.Name)
	logger.Info("In Reconcile...")

	// Get the CustomResource
	syncthing_cr := &syncthingv1alpha1.Instance{}
	err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, syncthing_cr)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Syncthing Resource not found. Must be deleted")
			return ctrl.Result{}, nil

		}
		logger.Error(err, "Something went terrible wrong!")
		return ctrl.Result{}, err
	}
	fillDefaultValues(syncthing_cr)

	// =====================================
	// === Create necessary API opbjects ===
	// =====================================

	// === Look for the referenced TLS Secret ===
	secret := generateTlsSecret(syncthing_cr)
	ctrl.SetControllerReference(syncthing_cr, secret, r.Scheme)
	obj, create_return, err := GetOrCreate(r, ctx, req, secret)
	if create_return == Created {
		logger.Info("Created Secret: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if create_return == CreateError || create_return == GetError {
		logger.Error(err, "Failed to create Secret for TLS config")
		return ctrl.Result{}, nil
	}
	logger.Info("Using Secret: " + obj.GetName())
	found_secret := obj.(*corev1.Secret)

	// === Ensure Deployment exists ===
	deployment := generateDeployment(syncthing_cr)
	ctrl.SetControllerReference(syncthing_cr, deployment, r.Scheme)
	obj, create_return, err = GetOrCreate(r, ctx, req, deployment)
	if create_return == Created {
		logger.Info("Created Deployment: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if create_return == CreateError || create_return == GetError {
		logger.Error(err, "Failed to create Deployment")
		return ctrl.Result{}, nil
	}
	logger.Info("Using Deployment: " + obj.GetName())
	found_deployment := obj.(*appsv1.Deployment)

	// === Ensure Service exists ===
	service := generateService(syncthing_cr)
	ctrl.SetControllerReference(syncthing_cr, service, r.Scheme)
	obj, create_return, err = GetOrCreate(r, ctx, req, service)
	if create_return == Created {
		logger.Info("Created Service: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if create_return == CreateError || create_return == GetError {
		logger.Error(err, "Failed to create Service")
		return ctrl.Result{}, nil
	}
	logger.Info("Using Service: " + obj.GetName())
	found_service := obj.(*corev1.Service)
	logger.Info("Using ClusterIP: " + found_service.Spec.ClusterIP)

	// =============================================
	// === Apply configuration to the API object ===
	// =============================================
	changed := false
	container_index := getContainerIndexByName(found_deployment.Spec.Template.Spec.Containers, syncthing_cr.Spec.ContainerName)
	if container_index == -1 {
		logger.Error(nil, "Unable to find syncthing Container in Deployment. Replacing Containers section...")
		found_deployment.Spec.Template.Spec.Containers = deployment.Spec.Template.Spec.Containers
	}

	// === Ensure the correct image is used ===
	container_image := syncthing_cr.Spec.ImageName + ":" + syncthing_cr.Spec.Tag
	current_image := found_deployment.Spec.Template.Spec.Containers[container_index].Image
	if current_image != container_image {
		logger.Info("Updating Image from " + current_image + " to " + container_image)
		found_deployment.Spec.Template.Spec.Containers[container_index].Image = container_image
		changed = true
	}

	// === Create Volume from TlsSecret ===
	if getVolumeIndexByName(found_deployment.Spec.Template.Spec.Volumes, "tls-config") == -1 ||
		getVolumeMountIndexByName(found_deployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, "tls-config") == -1 {
		logger.Info("Adding TLS configuration to deployment...")
		found_deployment = generateTlsVolumeAndMount(syncthing_cr, found_secret, found_deployment)
		logger.Info("tls volume mounts", "mount", found_deployment.Spec.Template.Spec.Containers[container_index].VolumeMounts)
	}

	// === Ensure all Volumes are present ===
	persistentVolumes := []corev1.Volume{syncthing_cr.Spec.ConfigVolume}
	persistentVolumes = append(persistentVolumes, syncthing_cr.Spec.DataVolumes...)
	mountConfigs := generateVolumeMountConfigs(syncthing_cr, persistentVolumes)
	for _, volume := range persistentVolumes {
		if -1 == getVolumeIndexByName(found_deployment.Spec.Template.Spec.Volumes, volume.Name) {
			logger.Info("Adding Volume: " + volume.Name)
			found_deployment.Spec.Template.Spec.Volumes = append(found_deployment.Spec.Template.Spec.Volumes, volume)
			if getVolumeMountIndexByName(found_deployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, volume.Name) == -1 {
				found_deployment.Spec.Template.Spec.Containers[container_index].VolumeMounts = append(found_deployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, mountConfigs[volume.Name])
			}
			if getVolumeMountIndexByName(found_deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts, volume.Name) == -1 {
				found_deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts = append(found_deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts, mountConfigs[volume.Name])
			}
			changed = true
		}
	}

	// === Update Deployment

	if changed {
		logger.Info("Updating Deployment...", "deployment", found_deployment)
		err = r.Update(ctx, found_deployment)
		if err != nil {
			logger.Error(err, "Failed to update Deployment: "+found_deployment.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// // Update the Memcached status with the pod names
	// // List the pods for this memcached's deployment
	// podList := &corev1.PodList{}
	// listOpts := []client.ListOption{
	// 	client.InNamespace(memcached.Namespace),
	// 	client.MatchingLabels(labelsForMemcached(memcached.Name)),
	// }
	// if err = r.List(ctx, podList, listOpts...); err != nil {
	// 	log.Error(err, "Failed to list pods", "Memcached.Namespace", memcached.Namespace, "Memcached.Name", memcached.Name)
	// 	return ctrl.Result{}, err
	// }
	// podNames := getPodNames(podList.Items)

	// // Update status.Nodes if needed
	// if !reflect.DeepEqual(podNames, memcached.Status.Nodes) {
	// 	memcached.Status.Nodes = podNames
	// 	err := r.Status().Update(ctx, memcached)
	// 	if err != nil {
	// 		log.Error(err, "Failed to update Memcached status")
	// 		return ctrl.Result{}, err
	// 	}
	// }
	logger.Info("Syncthing successfully deployed!")
	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
	// return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncthingv1alpha1.Instance{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
