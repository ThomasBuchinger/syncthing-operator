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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	syncthingv1alpha1 "github.com/thomasbuchinger/syncthing-operator/api/v1alpha1"
	syncthingclient "github.com/thomasbuchinger/syncthing-operator/pkg/syncthing-client"
)

// DeviceReconciler reconciles a Device object
type DeviceReconciler struct {
	client.Client
	StClient *syncthingclient.StClient
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=devices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=devices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=devices/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,namespace=default,resources=secrets,verbs=get;list;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Device object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// === Make sure we have a connection to Syncthing API ===
	apiKey, _ := findApikeyInNamespace(req.Namespace, r.Client, ctx)
	r.StClient.ApiKey = apiKey

	alive, msg := r.StClient.Ping()
	if !alive {
		logger.Info("Syncthing Instance not (yet) available: " + msg)
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
	}

	// === Fetch current Config ===
	config, err := r.StClient.GetConfig()
	if err != nil {
		logger.Error(err, "Cannot load Config")
		return ctrl.Result{}, err
	}

	// === Get the Device CR ===
	deviceCr := &syncthingv1alpha1.Device{}
	err = r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, deviceCr)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Device Resource deleted. Configuring Syncthing...")
			return r.ReconcileDeletion(req, config)

		}
		logger.Error(err, "Something went terrible wrong!")
		return ctrl.Result{}, err
	}

	// === check if device is present ===
	deviceIndex := r.StClient.GetDeviceIndexById(config.Devices, deviceCr.Spec.DeviceId)
	var device syncthingclient.DeviceElement
	changed := false
	if deviceIndex == -1 {
		device = generateStDeviceConfig(*deviceCr)
		changed = true
	} else {
		device = config.Devices[deviceIndex]
	}

	// === Check AutoAccept ===
	if device.Name != deviceCr.Name {
		logger.Info("Setting Name to " + deviceCr.Name)
		device.Name = deviceCr.Name
		changed = true
	}

	// === Check AutoAccept ===
	if device.AutoAcceptFolders != deviceCr.Spec.AutoAcceptFolders {
		logger.Info("Setting AutoAcceptFolders to " + fmt.Sprint(deviceCr.Spec.AutoAcceptFolders))
		device.AutoAcceptFolders = deviceCr.Spec.AutoAcceptFolders
		changed = true
	}

	// === Check MaxSendSpeed ===
	new_value := int(deviceCr.Spec.MaxSendSpeed.Value()) / 1000 * 8
	if device.MaxSendKbps != new_value {
		logger.Info(fmt.Sprintf("Setting Max Send Speed from %d kilobytes/s to %s-bytes/s", device.MaxSendKbps/8, deviceCr.Spec.MaxSendSpeed.String()))
		device.MaxSendKbps = new_value
		changed = true
	}
	// === Check MaxReceiveSpeed ===
	new_value = int(deviceCr.Spec.MaxReceiveSpeed.Value()) / 1000 * 8
	if device.MaxRecvKbps != new_value {
		logger.Info(fmt.Sprintf("Setting Max Receive Speed from %d kilobytes/s to %s-bytes/s", device.MaxRecvKbps/8, deviceCr.Spec.MaxReceiveSpeed.String()))
		device.MaxRecvKbps = new_value
		changed = true
	}

	if !changed {
		logger.Info("Device not changed: " + req.Name)
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Minute}, nil
	}

	logger.Info("Updating Device Configuration: " + deviceCr.Name)
	err = r.StClient.ReplaceDevice(device)
	if err != nil {
		logger.Error(err, "Error configuring device")
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
	} else {
		logger.Info("Device successfully configured: " + req.Name)
	}
	return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil

}

func findApikeyInNamespace(ns string, c interface{ client.Client }, ctx context.Context) (string, error) {
	secrets := &corev1.SecretList{}
	err := c.List(ctx, secrets, client.InNamespace(ns), client.HasLabels{"api.syncthing.buc.sh"})
	if err != nil {
		return "No Secret with 'api.syncthing.buc.sh' label", err
	}
	size := len(secrets.Items)
	if size != 1 {
		return "Ambigous. More than 1 secret matched" + fmt.Sprint(size), errors.NewBadRequest("API Secret")
	}
	apikey := string(secrets.Items[0].Data["apikey"])
	if apikey == "" {
		return "Invalid. Secret has no apikey", errors.NewBadRequest("API Secret")
	}
	return apikey, nil
}

func (r *DeviceReconciler) ReconcileDeletion(req ctrl.Request, config syncthingclient.StConfig) (ctrl.Result, error) {
	logger := ctrl.Log
	for _, dev := range config.Devices {
		if dev.Name == req.Name {
			err := r.StClient.DeleteDevice(dev.DeviceId)
			if err != nil {
				return ctrl.Result{}, err
			}

			logger.Info("Device deleted usccessfully: " + req.Name)
			return ctrl.Result{}, nil
		}
	}
	logger.Info("Deleted Resource '" + req.Name + "' was already deleted")
	return ctrl.Result{}, nil
}

func generateStDeviceConfig(deviceCr syncthingv1alpha1.Device) syncthingclient.DeviceElement {
	return syncthingclient.DeviceElement{
		DeviceId:                 deviceCr.Spec.DeviceId,
		Name:                     deviceCr.Name,
		Compression:              "metadata",
		Introducer:               false,
		Addresses:                []string{"dynamic"},
		SkipIntroductionRemovals: false,
		MaxSendKbps:              0,
		MaxRecvKbps:              0,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncthingv1alpha1.Device{}).
		Complete(r)
}
