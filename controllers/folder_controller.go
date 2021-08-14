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

// FolderReconciler reconciles a Folder object
type FolderReconciler struct {
	client.Client
	StClient *syncthingclient.StClient
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=folders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=folders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=syncthing.buc.sh,namespace=default,resources=folders/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,namespace=default,resources=secrets,verbs=get;list;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Folder object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *FolderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	deleteFolderFromSyncthing := false

	// === Get the Device CR ===
	folderCr := &syncthingv1.Folder{}
	err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, folderCr)
	if err != nil {
		if errors.IsNotFound(err) {
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
		logger.Info("Device Resource deleted. Configuring Syncthing...")
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

	// === Check Type ===
	if folder.Type != folderCr.Spec.Type {
		logger.Info("Setting Type: " + folderCr.Spec.Type)
		folder.Type = folderCr.Spec.Type
		changed = true
	}

	// === Check Pull Order ===
	if folder.Order != folderCr.Spec.Order {
		logger.Info("Setting Pull Order: " + folderCr.Spec.Order)
		folder.Order = folderCr.Spec.Order
		changed = true
	}

	// === Check FS Type ===
	if folder.FilesystemType != folderCr.Spec.FilesystemType {
		logger.Info("Setting FS Type: " + folderCr.Spec.FilesystemType)
		folder.FilesystemType = folderCr.Spec.FilesystemType
		changed = true
	}

	// === Check IgnoreDelete Flag ===
	if folder.IgnoreDelete != folderCr.Spec.IgnoreDelete {
		logger.Info("Setting IgnoreDelete Flag: " + fmt.Sprint(folderCr.Spec.IgnoreDelete))
		folder.IgnoreDelete = folderCr.Spec.IgnoreDelete
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
	if folder.RescanInterval != folderCr.Spec.RescanInterval {
		logger.Info("Setting RescanInterval: " + fmt.Sprint(folderCr.Spec.RescanInterval))
		folder.RescanInterval = folderCr.Spec.RescanInterval
		changed = true
	}

	// === Update Folder Configuration ===
	if !changed {
		logger.Info("Folder not changed: " + req.Name)
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Minute}, nil
	}

	logger.Info("Updating Folder Configuration: " + folderCr.Name)
	err = r.StClient.ReplaceFolder(folder)
	if err != nil {
		logger.Error(err, "Error configuring folder")
		return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
	} else {
		logger.Info("Device successfully configured: " + req.Name)
	}
	return ctrl.Result{Requeue: true, RequeueAfter: 10 * time.Second}, nil
}

func generateStFolderConfig(folderCr syncthingv1.Folder) syncthingclient.FolderElement {
	return syncthingclient.FolderElement{
		Id:    folderCr.Name,
		Label: folderCr.Spec.Label,
		Path:  "/var/syncthing/" + folderCr.Name,

		FilesystemType: "basic",
		Type:           "sendreceive",
		Order:          "random",
		IgnorePerms:    false,
		IgnoreDelete:   false,
		Paused:         false,
		RescanInterval: 3600,
		Devices:        []syncthingclient.DeviceReference{},
	}
}

func fillFolderDefaults(folderCr *syncthingv1.Folder) {
	if folderCr.Spec.Path == "" {
		folderCr.Spec.Path = "/var/syncthing/" + folderCr.Name
	}
	if folderCr.Spec.FilesystemType == "" {
		folderCr.Spec.FilesystemType = "basic"
	}
	if folderCr.Spec.Type == "" {
		folderCr.Spec.Type = "send-receive"
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
