package controllers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

func FindObject(r *InstanceReconciler, ns string, obj client.Object) (client.Object, FindReturnValues, error) {
	err := r.Get(r.ctx, types.NamespacedName{Namespace: ns, Name: obj.GetName()}, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, Deleted, err
		}

		return nil, UnknownError, err
	}
	return obj, Found, nil
}

func GetOrCreate(r *InstanceReconciler, ns string, obj client.Object) (client.Object, CreateReturnValues, error) {
	err := r.Get(r.ctx, types.NamespacedName{Namespace: ns, Name: obj.GetName()}, obj)
	if err != nil && errors.IsNotFound(err) {
		err2 := r.Create(r.ctx, obj)

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
