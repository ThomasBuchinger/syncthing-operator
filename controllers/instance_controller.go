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
	syncthingclient "github.com/thomasbuchinger/syncthing-operator/pkg/syncthing-client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// InstanceReconciler reconciles a Instance object
type InstanceReconciler struct {
	client.Client
	StClient *syncthingclient.StClient
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=instances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=instances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=instances/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,namespace=default,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,namespace=default,resources=pods,verbs=get;list;
//+kubebuilder:rbac:groups=core,namespace=default,resources=services,verbs=get;list;
//+kubebuilder:rbac:groups=core,namespace=default,resources=secrets,verbs=get;list;

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
	logger := log.FromContext(ctx)

	// Get the CustomResource
	instanceCr := &syncthingv1alpha1.Instance{}
	err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, instanceCr)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Syncthing Resource '" + req.Name + "' deleted.")
			return ctrl.Result{}, nil

		}
		logger.Error(err, "Something went terrible wrong!")
		return ctrl.Result{}, err
	}
	fillDefaultValues(instanceCr)

	// =====================================
	// === Create necessary API opbjects ===
	// =====================================

	// === Look for the referenced TLS Secret ===
	secret := generateTlsSecret(instanceCr)
	ctrl.SetControllerReference(instanceCr, secret, r.Scheme)
	obj, createReturn, err := GetOrCreate(r, ctx, req, secret)
	if createReturn == Created {
		logger.Info("Created Secret: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if createReturn == CreateError || createReturn == GetError {
		logger.Error(err, "Failed to create Secret for TLS config")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
	}
	logger.V(1).Info("Using Secret: " + obj.GetName())
	foundSecret := obj.(*corev1.Secret)

	// === Ensure Deployment exists ===
	deployment := generateDeployment(instanceCr)
	ctrl.SetControllerReference(instanceCr, deployment, r.Scheme)
	obj, createReturn, err = GetOrCreate(r, ctx, req, deployment)
	if createReturn == Created {
		logger.Info("Created Deployment: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if createReturn == CreateError || createReturn == GetError {
		logger.Error(err, "Failed to create Deployment")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
	}
	logger.V(1).Info("Using Deployment: " + obj.GetName())
	foundDeployment := obj.(*appsv1.Deployment)

	// === Ensure Service exists ===
	nodeportService := generateNodeportService(instanceCr)
	ctrl.SetControllerReference(instanceCr, nodeportService, r.Scheme)
	obj, createReturn, err = GetOrCreate(r, ctx, req, nodeportService)
	if createReturn == Created {
		logger.Info("Created Nodeport Service: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if createReturn == CreateError || createReturn == GetError {
		logger.Error(err, "Failed to create Nodeport Service")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
	}
	clusterService := generateClusterService(instanceCr)
	ctrl.SetControllerReference(instanceCr, clusterService, r.Scheme)
	obj, createReturn, err = GetOrCreate(r, ctx, req, clusterService)
	if createReturn == Created {
		logger.Info("Created Cluster Service: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if createReturn == CreateError || createReturn == GetError {
		logger.Error(err, "Failed to create Cluster Service")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
	}
	logger.V(1).Info("Using Service: " + obj.GetName())

	// =============================================
	// === Apply configuration to the API object ===
	// =============================================
	changed := false
	container_index := getContainerIndexByName(foundDeployment.Spec.Template.Spec.Containers, instanceCr.Spec.ContainerName)
	if container_index == -1 {
		logger.Error(nil, "Unable to find syncthing Container in Deployment. Replacing Containers section...")
		foundDeployment.Spec.Template.Spec.Containers = deployment.Spec.Template.Spec.Containers
	}

	// === Ensure the correct image is used ===
	container_image := instanceCr.Spec.ImageName + ":" + instanceCr.Spec.Tag
	current_image := foundDeployment.Spec.Template.Spec.Containers[container_index].Image
	if current_image != container_image {
		logger.Info("Updating Image from " + current_image + " to " + container_image)
		foundDeployment.Spec.Template.Spec.Containers[container_index].Image = container_image
		changed = true
	}

	// === Create Volume from TlsSecret ===
	if getVolumeIndexByName(foundDeployment.Spec.Template.Spec.Volumes, instanceCr.Spec.TlsConfigName) == -1 ||
		getVolumeMountIndexByName(foundDeployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, instanceCr.Spec.TlsConfigName) == -1 {
		logger.Info("Adding TLS configuration to deployment...")
		foundDeployment = generateTlsVolumeAndMount(instanceCr, foundSecret, foundDeployment)
	}

	// === Ensure all Volumes are present ===
	persistentVolumes := []corev1.Volume{instanceCr.Spec.ConfigVolume}
	persistentVolumes = append(persistentVolumes, instanceCr.Spec.DataVolumes...)
	mountConfigs := generateVolumeMountConfigs(instanceCr, persistentVolumes)
	for _, volume := range persistentVolumes {
		if -1 == getVolumeIndexByName(foundDeployment.Spec.Template.Spec.Volumes, volume.Name) {
			logger.Info("Adding Volume: " + volume.Name)
			foundDeployment.Spec.Template.Spec.Volumes = append(foundDeployment.Spec.Template.Spec.Volumes, volume)
			if getVolumeMountIndexByName(foundDeployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, volume.Name) == -1 {
				foundDeployment.Spec.Template.Spec.Containers[container_index].VolumeMounts = append(foundDeployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, mountConfigs[volume.Name])
			}
			changed = true
		}
	}

	// === Update Deployment

	if changed {
		logger.Info("Updating Deployment...", "deployment", foundDeployment)
		err = r.Update(ctx, foundDeployment)
		if err != nil {
			logger.Error(err, "Failed to update Deployment: "+foundDeployment.Name)
			return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// TODO: Update InstanceCr Status correctly

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
	logger.V(1).Info("Syncthing Container deployed!")
	return r.ReconcileApplication(ctx, req, instanceCr)
}

func (r *InstanceReconciler) ReconcileApplication(ctx context.Context, req ctrl.Request, syncthing_cr *syncthingv1alpha1.Instance) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	r.StClient.ApiKey = syncthing_cr.Spec.ApiKey

	for i := 0; i < 10; i++ {
		alive, msg := r.StClient.Ping()
		if !alive {
			logger.Info("Syncthing instance not reachable (yet?): " + msg)
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		}
	}

	// === Fetch current Syncthing config ===
	config, err := r.StClient.GetConfig()
	if err != nil {
		logger.Error(err, "cannot fetch config")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
	}

	// === Disable UsegeStatistics to make the popup go away ===
	if config.Options.UrAccepted != syncthingclient.DenyUsageReport {
		logger.Info("Configure UsageReport...")
		err := r.StClient.SendUsageStatistics(syncthingclient.AllowUsageReport)
		if err != nil {
			logger.Error(err, "Error setting UsageReporting")
			return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
		}
	}

	// === Set a admin password ===
	// This password is set once, changes in the UI are not reset
	// To reset the password to the default, change the admin username, this will trigger the reset
	username := "syncthing"
	if config.Gui.User != username {
		logger.Info("Configure authentication")
		err = r.StClient.SetAuth(username, syncthing_cr.Spec.ApiKey)
		if err != nil {
			logger.Error(err, "Error setting user authentication")
			return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
		}
	}

	logger.Info("Syncthing successfully configured")
	return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncthingv1alpha1.Instance{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
