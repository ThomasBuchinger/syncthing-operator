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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
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
	ctx      context.Context
	req      ctrl.Request
	logger   logr.Logger
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
	r.logger = log.FromContext(ctx)
	r.ctx = ctx
	r.req = req

	// Get the CustomResource
	instanceCr := &syncthingv1alpha1.Instance{}
	err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, instanceCr)
	if err != nil {
		if errors.IsNotFound(err) {
			r.logger.Info("Syncthing Resource '" + req.Name + "' deleted.")
			return ctrl.Result{}, nil

		}
		r.logger.Error(err, "Something went terrible wrong!")
		return ctrl.Result{}, err
	}
	fillDefaultValues(instanceCr)

	if instanceCr.Spec.EnableInstance {
		result, err := r.ReconcileKubernetes(instanceCr)
		if err != nil || result.Requeue {
			return result, err
		}
		r.logger.V(1).Info("Syncthing Container deployed!")
	}
	if instanceCr.Spec.EnableInstance && instanceCr.Spec.EnableNodeport {
		result, err := r.ReconcileNodeportservice(instanceCr)
		if err != nil || result.Requeue {
			return result, err
		}
		r.logger.V(1).Info("Syncthing Nodeports configured!")
	}
	if instanceCr.Spec.EnableConfiguration {
		result, err := r.ReconcileApplication(instanceCr)
		if err != nil || result.Requeue {
			return result, err
		}
		r.logger.V(1).Info("Syncthing Application configured!")
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
	return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, nil
}

func (r *InstanceReconciler) ReconcileKubernetes(instanceCr *syncthingv1alpha1.Instance) (ctrl.Result, error) {
	// =====================================
	// === Create necessary API opbjects ===
	// =====================================

	// === Look for the referenced TLS Secret ===
	secret := generateTlsSecret(instanceCr)
	ctrl.SetControllerReference(instanceCr, secret, r.Scheme)
	obj, createReturn, err := GetOrCreate(r, instanceCr.GetNamespace(), secret)
	if createReturn == Created {
		r.logger.Info("Created Secret: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if createReturn == CreateError || createReturn == GetError {
		r.logger.Error(err, "Failed to create Secret for TLS config")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
	}
	r.logger.V(1).Info("Using Secret: " + obj.GetName())
	foundSecret := obj.(*corev1.Secret)

	// === Ensure Deployment exists ===
	deployment := generateDeployment(instanceCr)
	ctrl.SetControllerReference(instanceCr, deployment, r.Scheme)
	obj, createReturn, err = GetOrCreate(r, instanceCr.GetNamespace(), deployment)
	if createReturn == Created {
		r.logger.Info("Created Deployment: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if createReturn == CreateError || createReturn == GetError {
		r.logger.Error(err, "Failed to create Deployment")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
	}
	r.logger.V(1).Info("Using Deployment: " + obj.GetName())
	foundDeployment := obj.(*appsv1.Deployment)

	// === Ensure Service exists ===
	clusterService := generateClusterService(instanceCr)
	ctrl.SetControllerReference(instanceCr, clusterService, r.Scheme)
	obj, createReturn, err = GetOrCreate(r, r.req.Namespace, clusterService)
	if createReturn == Created {
		r.logger.Info("Created Cluster Service: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if createReturn == CreateError || createReturn == GetError {
		r.logger.Error(err, "Failed to create Cluster Service")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
	}
	r.logger.V(1).Info("Using Service: " + obj.GetName())

	// =============================================
	// === Apply configuration to the API object ===
	// =============================================
	changed := false
	container_index := getContainerIndexByName(foundDeployment.Spec.Template.Spec.Containers, instanceCr.Spec.ContainerName)
	if container_index == -1 {
		r.logger.Error(nil, "Unable to find syncthing Container in Deployment. Replacing Containers section...")
		foundDeployment.Spec.Template.Spec.Containers = deployment.Spec.Template.Spec.Containers
	}

	// === Ensure the correct image is used ===
	container_image := instanceCr.Spec.ImageName + ":" + instanceCr.Spec.Tag
	current_image := foundDeployment.Spec.Template.Spec.Containers[container_index].Image
	if current_image != container_image {
		r.logger.Info("Updating Image from " + current_image + " to " + container_image)
		foundDeployment.Spec.Template.Spec.Containers[container_index].Image = container_image
		changed = true
	}

	// === Create Volume from TlsSecret ===
	if getVolumeIndexByName(foundDeployment.Spec.Template.Spec.Volumes, instanceCr.Spec.TlsConfigName) == -1 ||
		getVolumeMountIndexByName(foundDeployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, instanceCr.Spec.TlsConfigName) == -1 {
		r.logger.Info("Adding TLS configuration to deployment...")
		foundDeployment = generateTlsVolumeAndMount(instanceCr, foundSecret, foundDeployment)
	}

	// === Ensure all Volumes are present ===
	persistentVolumes := []corev1.Volume{instanceCr.Spec.ConfigVolume, instanceCr.Spec.DataRoot}
	persistentVolumes = append(persistentVolumes, instanceCr.Spec.AdditionalDataVolumes...)
	mountConfigs := generateVolumeMountConfigs(instanceCr, persistentVolumes)
	for _, volume := range persistentVolumes {
		if -1 == getVolumeIndexByName(foundDeployment.Spec.Template.Spec.Volumes, volume.Name) {
			r.logger.Info("Adding Volume: " + volume.Name)
			foundDeployment.Spec.Template.Spec.Volumes = append(foundDeployment.Spec.Template.Spec.Volumes, volume)
			if getVolumeMountIndexByName(foundDeployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, volume.Name) == -1 {
				foundDeployment.Spec.Template.Spec.Containers[container_index].VolumeMounts = append(foundDeployment.Spec.Template.Spec.Containers[container_index].VolumeMounts, mountConfigs[volume.Name])
			}
			changed = true
		}
	}

	// === Update Deployment

	if changed {
		r.logger.Info("Updating Deployment...", "deployment", foundDeployment)
		err = r.Update(r.ctx, foundDeployment)
		if err != nil {
			r.logger.Error(err, "Failed to update Deployment: "+foundDeployment.Name)
			return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) ReconcileNodeportservice(instanceCr *syncthingv1alpha1.Instance) (ctrl.Result, error) {

	// === Ensure Service exists ===
	nodeportService := generateNodeportService(instanceCr)
	ctrl.SetControllerReference(instanceCr, nodeportService, r.Scheme)
	obj, createReturn, err := GetOrCreate(r, instanceCr.GetNamespace(), nodeportService)
	if createReturn == Created {
		r.logger.Info("Created Nodeport Service: " + obj.GetName())
		return ctrl.Result{Requeue: true}, nil
	}
	if createReturn == CreateError || createReturn == GetError {
		r.logger.Error(err, "Failed to create Nodeport Service")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
	}
	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) ReconcileApplication(syncthing_cr *syncthingv1alpha1.Instance) (ctrl.Result, error) {
	var err error
	r.StClient, err = syncthingclient.FromCr(syncthing_cr.Spec.Clientconfig, syncthing_cr.Namespace, r.Client, r.ctx)
	if err != nil {
		r.logger.Error(err, "Error initializing Syncthing Client")
		return ctrl.Result{}, err
	}

	alive, msg := r.StClient.Ping()
	if !alive {
		r.logger.Info("Syncthing instance not reachable (yet?): " + msg)
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
	}

	// === Fetch current Syncthing config ===
	config, err := r.StClient.GetConfig()
	if err != nil {
		r.logger.Error(err, "cannot fetch config")
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
	}

	// === Disable UsegeStatistics to make the popup go away ===
	if config.Options.UrAccepted != syncthingclient.DenyUsageReport {
		r.logger.Info("Configure UsageReport...")
		err := r.StClient.SendUsageStatistics(syncthingclient.AllowUsageReport)
		if err != nil {
			r.logger.Error(err, "Error setting UsageReporting")
			return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, err
		}
	}

	// === Set a admin password ===
	// This password is set once, changes in the UI are not reset
	// To reset the password to the default, change the admin username, this will trigger the reset
	if syncthing_cr.Spec.InsecureWeb {
		if config.Gui.InsecureAdminAccess {
			r.logger.Info("Disable Authentication")
			err = r.StClient.SetAuth(false, "", "")
			if err != nil {
				r.logger.Error(err, "Error setting user authentication")
				return ctrl.Result{Requeue: true}, err
			}
		}
	} else {
		username := syncthing_cr.Spec.AdminUser
		if config.Gui.User != username {
			r.logger.Info("Set Password for " + username)
			err = r.StClient.SetAuth(true, username, syncthing_cr.Spec.Clientconfig.ApiKey)
			if err != nil {
				r.logger.Error(err, "Error setting user authentication")
				return ctrl.Result{Requeue: true}, err
			}
		}
	}

	// === Set TLS config ===
	if syncthing_cr.Spec.HttpsCrt != "" || syncthing_cr.Spec.HttpsKey != "" {
		r.logger.Error(nil, "HTTPs not implemented yet")
	}

	// === Set Max Sync Speed ===
	send := syncthing_cr.Spec.MaxSendSpeedValue
	recv := syncthing_cr.Spec.MaxReceiveSpeedValue
	if config.Options.MaxSendKbps != int(send) && config.Options.MaxRecvKbps != int(recv) {
		r.logger.Info(fmt.Sprintf(
			"Set Bandwidth Limit: Send: %d MB/s Recv: %d MB/s",
			resource.NewQuantity(send, resource.DecimalSI).ScaledValue(resource.Mega),
			resource.NewQuantity(recv, resource.DecimalSI).ScaledValue(resource.Mega),
		))
		err = r.StClient.SetSpeed(send, recv)
		if err != nil {
			r.logger.Error(err, "Error setting Bandwidth Limit")
			return ctrl.Result{Requeue: true}, err
		}
	}

	r.logger.Info("Syncthing successfully configured")
	return ctrl.Result{}, nil
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
