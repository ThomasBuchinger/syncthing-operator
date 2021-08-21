package controllers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CreateReturnValues int

const (
	CreateError CreateReturnValues = iota
	GetError
	Created
	Existed
)

// Helper method to check if the CreateReturnValue is one of the given values
func (n CreateReturnValues) IsOneOf(list ...CreateReturnValues) bool {
	for _, value := range list {
		if n == value {
			return true
		}
	}
	return false
}

// Update a Resource, with logging and return a reconcile-result
func UpdateObject(r *InstanceReconciler, obj client.Object) (ctrl.Result, error) {
	r.logger.Info(fmt.Sprintf("Updating %s/%s...", obj.GetObjectKind(), obj.GetName()))
	err := r.Update(r.ctx, obj)
	if err != nil {
		r.logger.Error(err, fmt.Sprintf("Failed to update %s/%s", obj.GetObjectKind(), obj.GetName()))
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

// Try to find an object with obj.Name.
// This method does not compare the object in any way. Just matching Kind/Namespace/Name
// If the object does not exist, Try to create it.
// Return: StatusCode, the found/created object, a (suggested) reconcile.Result and any encountered errors
func GetOrCreateObject(r *InstanceReconciler, ns string, obj client.Object) (CreateReturnValues, client.Object, ctrl.Result, error) {
	err := r.Get(r.ctx, types.NamespacedName{Namespace: ns, Name: obj.GetName()}, obj)
	if err != nil && errors.IsNotFound(err) { // Object does not exist

		err2 := r.Create(r.ctx, obj) // Try to create the object
		if err2 != nil {
			r.logger.Error(err2, fmt.Sprintf("Failed to create %s/%s", obj.GetObjectKind(), obj.GetName()))
			return CreateError, nil, ctrl.Result{}, err2
		}

		// Suggest to Requeue the request after creating the object
		// By doing one thing at a time, we avoid branches in the code and there isn't a good reason not to
		r.logger.Info(fmt.Sprintf("Successfully created %s/%s", obj.GetObjectKind(), obj.GetName()))
		return Created, obj, ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		r.logger.Error(err, fmt.Sprintf("Failed to GET %s/%s", obj.GetObjectKind(), obj.GetName()))
		return GetError, nil, ctrl.Result{}, err
	}

	r.logger.V(1).Info(fmt.Sprintf("Found %s/%s", obj.GetObjectKind(), obj.GetName()))
	return Existed, obj, ctrl.Result{}, nil
}
