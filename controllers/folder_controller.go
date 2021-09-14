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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	syncthingv1 "github.com/thomasbuchinger/syncthing-operator/api/v1"
	syncthingclient "github.com/thomasbuchinger/syncthing-operator/pkg/syncthing-client"
)

type FolderReconciler struct {
	client.Client
	StClient *syncthingclient.StClient
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=folders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=folders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=folders/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,namespace=default,resources=secrets,verbs=get;list;
func (r *FolderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	deleteFolderFromSyncthing := false

	// === Get the Device CR ===
	folderCr := &syncthingv1.Folder{}
	err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, folderCr)
	if err != nil {
		if errors.IsNotFound(err) {
			// The CustomResource is deleted, but we need to make sure it is deleted in syncthing as well
			// Defer calling ReconcileDeletion() until we established a connection to syncthing
			deleteFolderFromSyncthing = true
		} else {
			logger.Error(err, "Something went terrible wrong!")
			return ctrl.Result{}, err
		}
	}

	// === Make sure we have a connection to Syncthing API ===
	r.StClient, err = syncthingclient.FromCr(folderCr.Spec.Clientconfig, req.Namespace, r.Client, ctx)
	if err != nil {
		logger.Error(err, "Error initializing Syncthing Client")
		return ctrl.Result{}, err
	}

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

	if deleteFolderFromSyncthing {
		logger.Info("Folder Resource deleted. Configuring Syncthing...")
		r.StClient.DeleteFolder(req.Name)
		return ctrl.Result{}, nil
	}

	// === check if device is present ===
	folderIndex := r.StClient.GetFolderIndexById(config.Folders, folderCr.Name)
	var folder syncthingclient.FolderElement
	fillFolderDefaults(folderCr)
	changed := false
	if folderIndex == -1 {
		folder = generateStFolderConfig(*folderCr)
		changed = true
	} else {
		folder = config.Folders[folderIndex]
	}

	// #########################
	// # Start Reconcile Logic #
	// #########################

	// === Check Folder Label ===
	if folder.Label != folderCr.Spec.Label {
		logger.Info("Setting Folder Label: " + folderCr.Spec.Label)
		folder.Label = folderCr.Spec.Label
		changed = true
	}

	// === Check Path ===
	if folder.Path != folderCr.Spec.Path {
		logger.Info("Setting Path: " + folderCr.Spec.Path)
		folder.Path = folderCr.Spec.Path
		changed = true
	}

	// === Set FolderID (if UseNameAsId) ===
	if folderCr.Spec.UseNameAsId && folder.Id != folderCr.Name {
		logger.Info("Setting folder ID to: " + folderCr.Name)
		folder.Id = folderCr.Name
		changed = true
	}

	// === Check Type ===
	if folder.Type != string(folderCr.Spec.Type) {
		logger.Info("Setting Type: " + string(folderCr.Spec.Type))
		folder.Type = string(folderCr.Spec.Type)
		changed = true
	}

	// === Check Pull Order ===
	if folder.Order != string(folderCr.Spec.Order) {
		logger.Info("Setting Pull Order: " + string(folderCr.Spec.Order))
		folder.Order = string(folderCr.Spec.Order)
		changed = true
	}

	// === Check IgnorePermisions Flag ===
	if folder.IgnorePerms != folderCr.Spec.IgnorePerms {
		logger.Info("Setting IgnorePermissions Flag: " + fmt.Sprint(folderCr.Spec.IgnorePerms))
		folder.IgnorePerms = folderCr.Spec.IgnorePerms
		changed = true
	}

	// === Check Pause Flag ===
	if folder.Paused != folderCr.Spec.Paused {
		logger.Info("Setting Pause Flag: " + fmt.Sprint(folderCr.Spec.Paused))
		folder.Paused = folderCr.Spec.Paused
		changed = true
	}

	// === Check Rescan Interval ===
	if folder.RescanIntervalS != folderCr.Spec.RescanInterval {
		logger.Info("Setting RescanInterval: " + fmt.Sprint(folderCr.Spec.RescanInterval))
		folder.RescanIntervalS = folderCr.Spec.RescanInterval
		changed = true
	}

	// === Chack Marker Settings ===
	if folderCr.Spec.StMarker != ".stfolder" && folder.MarkerName != folderCr.Spec.StMarker {
		logger.Info("Setting StMarker to: " + folderCr.Spec.StMarker)
		folder.MarkerName = folderCr.Spec.StMarker
		changed = true
	}

	// === Update Folder Configuration ===
	if !changed {
		logger.V(1).Info("Folder not changed: " + req.Name)
	} else {
		logger.Info("Updating Folder Configuration: " + folderCr.Name)
		err = r.StClient.ReplaceFolder(folder)
		if err != nil {
			logger.Error(err, "Error configuring folder")
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		} else {
			logger.Info("Folder successfully configured: " + req.Name)
		}

	}

	// #########################
	// # Update shared devices #
	// #########################

	// === Add Devices by ID ===
	changed, new, retry := false, false, false
	for _, dev := range folderCr.Spec.SharedDeviceIds {
		new = folder.AddDeviceById(dev)
		if new {
			logger.Info(fmt.Sprintf("Add Device %s to Folder %s", dev, req.Name))
		}
		changed = changed || new
	}

	// === Add Devices by name ===
	for _, name := range folderCr.Spec.SharedDeviceNames {
		dev := resolveDeviceNameToId(config.Devices, name)
		if dev == "" {
			logger.Info(fmt.Sprintf("Unknown Device '%s'. Retrying...", name))
			retry = true
		} else {
			// Update Devices with ID
			new = folder.AddDeviceById(dev)
			if new {
				logger.Info(fmt.Sprintf("Add Device %s with ID %s to Folder %s", name, dev, req.Name))
			}
			changed = changed || new
		}
	}

	// === Update Folder Configuration ===
	if !changed {
		logger.V(1).Info("Shared devices not changed for Folder: " + req.Name)
	} else {
		logger.Info("Updating shared Device Configuration for Folder: " + folderCr.Name)
		err = r.StClient.ReplaceFolder(folder)
		if err != nil {
			logger.Error(err, "Error configuring shared Devices for Folder")
			return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
		} else {
			logger.Info("Shared Devices successfully configured for Folder: " + req.Name)
		}

	}

	if retry {
		return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Minute}, nil
	} else {
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Minute}, nil
	}
}

func resolveDeviceNameToId(list []syncthingclient.DeviceElement, name string) string {
	for _, dev := range list {
		if dev.Name == name {
			return dev.DeviceId
		}
	}
	return ""
}

func generateStFolderConfig(folderCr syncthingv1.Folder) syncthingclient.FolderElement {
	// Defaults for settings not supported by the operator
	return syncthingclient.FolderElement{
		Id:    folderCr.Name,
		Label: folderCr.Spec.Label,
		Path:  "/var/syncthing/" + folderCr.Name,

		FilesystemType:  "basic",
		Type:            "sendreceive",
		Order:           "random",
		IgnorePerms:     false,
		IgnoreDelete:    false,
		Paused:          false,
		RescanIntervalS: 3600,
		Devices:         []syncthingclient.DeviceReference{},
		MarkerName:      ".stfolder",
	}
}

func fillFolderDefaults(folderCr *syncthingv1.Folder) {
	if folderCr.Spec.Path == "" {
		folderCr.Spec.Path = "/var/syncthing/" + folderCr.Name
	}
	if folderCr.Spec.Type == "" {
		folderCr.Spec.Type = "sendreceive"
	}
	if folderCr.Spec.Order == "" {
		folderCr.Spec.Order = "random"
	}
	if folderCr.Spec.RescanInterval == -1 {
		folderCr.Spec.RescanInterval = 3600
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *FolderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&syncthingv1.Folder{}).
		Complete(r)
}
