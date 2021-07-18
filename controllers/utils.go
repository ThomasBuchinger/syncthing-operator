package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FindReturnValues int

const (
	Deleted FindReturnValues = iota
	Found
	UnknownError
)

type CreateReturnValues int

const (
	CreateError CreateReturnValues = iota
	GetError
	Created
	Existed
)

func FindObject(r *InstanceReconciler, ctx context.Context, req ctrl.Request, obj client.Object) (client.Object, FindReturnValues, error) {
	err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: obj.GetName()}, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, Deleted, err
		}

		return nil, UnknownError, err
	}
	return obj, Found, nil
}

func GetOrCreate(r *InstanceReconciler, ctx context.Context, req ctrl.Request, obj client.Object) (client.Object, CreateReturnValues, error) {
	err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: obj.GetName()}, obj)
	if err != nil && errors.IsNotFound(err) {
		err2 := r.Create(ctx, obj)

		if err2 != nil {
			fmt.Println("Failed to create:", obj.GetName())
			return nil, CreateError, err2
		}

		fmt.Println("Successfully created:", obj.GetName())
		return obj, Created, nil
	}
	if err != nil {
		fmt.Println("Failed to GET:", obj.GetName(), err)
		return nil, GetError, err
	}

	fmt.Println("Found :", obj.GetName())
	return obj, Existed, nil
}
