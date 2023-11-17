/*
Copyright 2023.

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

package finetune

import (
	"context"
	"fmt"
	"time"

	"github.com/DataTunerX/utility-server/logging"
	"k8s.io/apimachinery/pkg/types"

	"github.com/DataTunerX/finetune-experiment-controller/pkg/util/handlererr"
	finetunev1beta1 "github.com/DataTunerX/meta-server/api/finetune/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// FinetuneExperimentReconciler reconciles a FinetuneExperiment object
type FinetuneExperimentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logging.Logger
}

//+kubebuilder:rbac:groups=finetune.datatunerx.io,resources=finetuneexperiments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=finetune.datatunerx.io,resources=finetuneexperiments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=finetune.datatunerx.io,resources=finetuneexperiments/finalizers,verbs=update

func (r *FinetuneExperimentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Infof("Start reconcile finetuneExperiment: %s/%s,", req.Name, req.Namespace)
	finetuneExperiment := &finetunev1beta1.FinetuneExperiment{}
	if err := r.Get(ctx, req.NamespacedName, finetuneExperiment); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Errorf("FinetuneExperiment resource not found. Ignoring since object must be deleted.")
			return handlererr.HandlerErr(nil)
		}
		r.Log.Errorf("Failed get finetuneExperiment: %s/%s, Err: %v", req.Name, req.Namespace, err)
		return handlererr.HandlerErr(err)
	}

	if finetuneExperiment.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(finetuneExperiment, finetunev1beta1.FinetuneGroupFinalizer) {
			// todo cleaner
			controllerutil.RemoveFinalizer(finetuneExperiment, finetunev1beta1.FinetuneGroupFinalizer)
			if err := r.Update(ctx, finetuneExperiment); err != nil {
				r.Log.Errorf("Remove finalizer failed: %s/%s, Err: %v", req.Name, req.Namespace, err)
				return handlererr.HandlerErr(err)
			}
		}
		return handlererr.HandlerErr(nil)
	}
	if !controllerutil.ContainsFinalizer(finetuneExperiment, finetunev1beta1.FinetuneGroupFinalizer) {
		controllerutil.AddFinalizer(finetuneExperiment, finetunev1beta1.FinetuneGroupFinalizer)
		err := r.Update(ctx, finetuneExperiment)
		if err != nil {
			r.Log.Errorf("Add finalizer failed: %s/%s, %v", req.Name, req.Namespace, err)
			return handlererr.HandlerErr(err)
		}
	}

	if finetuneExperiment.Spec.Pending {
		finetuneExperiment.Status.State = finetunev1beta1.FinetuneExperimentPending
		if err := r.Client.Status().Update(ctx, finetuneExperiment); err != nil {
			r.Log.Errorf("Update fineExperiment %s/%s status failed", finetuneExperiment.Name, finetuneExperiment.Namespace)
			return handlererr.HandlerErr(err)
		}
		return handlererr.HandlerErr(nil)
	}

	for i := range finetuneExperiment.Spec.FinetuneJobs {
		finetuneJob := finetuneExperiment.Spec.FinetuneJobs[i]
		if finetuneJob.Name == nil {
			name := fmt.Sprintf("%s-%s", finetuneExperiment.Name, "finetunejob")
			finetuneJob.Name = &name
		}
		finetuneJobInstance := &finetunev1beta1.FinetuneJob{}
		finetuneJobInstance.Spec = finetuneJob.Spec
		finetuneJobInstance.Name = *finetuneJob.Name
		finetuneJobInstance.Namespace = finetuneExperiment.Namespace
		if err := ctrl.SetControllerReference(finetuneExperiment, finetuneJobInstance, r.Scheme); err != nil {
			r.Log.Errorf("SetControllerReference failed finetuneJob: %s/%s, owner finetuneExperiment: %s/%s, err: %v",
				finetuneJobInstance.Name, finetuneJobInstance.Namespace, finetuneExperiment.Name, finetuneExperiment.Namespace, err)
			return handlererr.HandlerErr(err)
		}
		if err := r.Client.Create(ctx, finetuneJobInstance); err != nil {
			if !errors.IsAlreadyExists(err) {
				r.Log.Errorf("Create finetuneJob %s/%s failed: %v", finetuneJobInstance.Name, finetuneJobInstance.Namespace, err)
				return handlererr.HandlerErr(err)
			}
		}
		existFinetuneJob := &finetunev1beta1.FinetuneJob{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Name:      *finetuneJob.Name,
			Namespace: finetuneExperiment.Namespace,
		}, existFinetuneJob); err != nil {
			r.Log.Errorf("Get finetuneJob failed: %v", err)
			return handlererr.HandlerErr(err)
		}
		alreadyExists := false

		// Iterate over the JobsStatus to check if existFinetuneJob.Name exists
		for _, jobStatus := range finetuneExperiment.Status.JobsStatus {
			if jobStatus.Name == existFinetuneJob.Name {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			finetuneExperiment.Status.JobsStatus = append(finetuneExperiment.Status.JobsStatus, finetunev1beta1.FinetuneJobStatusSetting{
				Name:              existFinetuneJob.Name,
				FinetuneJobStatus: existFinetuneJob.Status,
			})
		}

	}
	finetuneExperiment.Status.State = finetunev1beta1.FinetuneExperimentProcessing
	if err := r.Client.Status().Update(ctx, finetuneExperiment); err != nil {
		r.Log.Errorf("Update fineExperiment %s/%s status failed", finetuneExperiment.Name, finetuneExperiment.Namespace)
		return handlererr.HandlerErr(err)
	}
	return handlererr.HandlerErr(nil)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FinetuneExperimentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&finetunev1beta1.FinetuneExperiment{}).
		WithOptions(controller.Options{
			CacheSyncTimeout:        10 * time.Second,
			MaxConcurrentReconciles: 1}).
		Complete(r)
}
