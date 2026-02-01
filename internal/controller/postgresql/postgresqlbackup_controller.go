/*
Copyright 2026.

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

package postgresql

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ctrl "sigs.k8s.io/controller-runtime"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/internal/controller/config"
)

// PostgresqlBackupReconciler reconciles a PostgresqlBackup object.
type PostgresqlBackupReconciler struct {
	Recorder record.EventRecorder
	client.Client
	Scheme                              *runtime.Scheme
	ControllerRuntimeDetailedErrorTotal *prometheus.CounterVec
	Log                                 logr.Logger
	ControllerName                      string
	ReconcileTimeout                    time.Duration
}

// +kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlbackups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PostgresqlBackup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *PostgresqlBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Issue with this logger: controller and controllerKind are incorrect
	// Build another logger from upper to fix this.
	// reqLogger := log.FromContext(ctx)
	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling PostgresqlBackup")

	// Fetch the PostgresqlDatabase instance
	instance := &postgresqlv1alpha1.PostgresqlBackup{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Original patch
	originalPatch := client.MergeFrom(instance.DeepCopy())

	// Create timeout in ctx
	timeoutCtx, cancel := context.WithTimeout(ctx, r.ReconcileTimeout)
	// Defer cancel
	defer cancel()

	// Init result
	var res ctrl.Result

	errC := make(chan error, 1)

	// Create wrapping function
	cb := func() {
		a, err := r.mainReconcile(timeoutCtx, reqLogger, instance, originalPatch)
		// Save result
		res = a
		// Send error
		errC <- err
	}

	// Start wrapped function
	go cb()

	// Run or timeout
	select {
	case <-timeoutCtx.Done():
		// ? Note: Here use primary context otherwise update to set error will be aborted
		return r.manageError(ctx, reqLogger, instance, originalPatch, timeoutCtx.Err())
	case err := <-errC:
		return res, err
	}
}

func (r *PostgresqlBackupReconciler) mainReconcile(
	ctx context.Context,
	reqLogger logr.Logger,
	instance *postgresqlv1alpha1.PostgresqlBackup,
	originalPatch client.Patch,
) (ctrl.Result, error) {
	// Deletion case
	if !instance.GetDeletionTimestamp().IsZero() {
		// Deletion in progress detected
		// TODO wait for children

		// Remove finalizer
		controllerutil.RemoveFinalizer(instance, config.Finalizer)
		// Update CR
		err := r.Update(ctx, instance)
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
		// Stop reconcile
		return ctrl.Result{}, nil
	}

	// Creation case

	// Add finalizer, owners and default values
	updated, err := r.updateInstance(ctx, instance)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}
	// Check if it has been updated in order to stop this reconcile loop here for the moment
	if updated {
		return ctrl.Result{}, nil
	}

	// Compute spec hash

	// Success
	return r.manageSuccess(ctx, reqLogger, instance, originalPatch)
}

func (r *PostgresqlBackupReconciler) updateInstance(
	ctx context.Context,
	instance *postgresqlv1alpha1.PostgresqlBackup,
) (bool, error) {
	// Deep copy
	oCopy := instance.DeepCopy()

	// Add finalizer
	controllerutil.AddFinalizer(instance, config.Finalizer)

	// Check if update is needed
	if !reflect.DeepEqual(oCopy.ObjectMeta, instance.ObjectMeta) {
		return true, r.Update(ctx, instance)
	}

	return false, nil
}

func (r *PostgresqlBackupReconciler) manageError(
	ctx context.Context,
	logger logr.Logger,
	instance *postgresqlv1alpha1.PostgresqlBackup,
	originalPatch client.Patch,
	issue error,
) (ctrl.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.Recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = postgresqlv1alpha1.BackupErrorPhase

	// Increase fail counter
	r.ControllerRuntimeDetailedErrorTotal.WithLabelValues(r.ControllerName, instance.Namespace, instance.Name).Inc()

	// Patch status
	err := r.Status().Patch(ctx, instance, originalPatch)
	if err != nil {
		logger.Error(err, "unable to update status")
	}

	// Return error
	return ctrl.Result{}, issue
}

func (r *PostgresqlBackupReconciler) manageSuccess(
	ctx context.Context,
	logger logr.Logger,
	instance *postgresqlv1alpha1.PostgresqlBackup,
	originalPatch client.Patch,
) (ctrl.Result, error) {
	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = postgresqlv1alpha1.BackupValidPhase

	// Patch status
	err := r.Status().Patch(ctx, instance, originalPatch)
	if err != nil {
		// Increase fail counter
		r.ControllerRuntimeDetailedErrorTotal.WithLabelValues(r.ControllerName, instance.Namespace, instance.Name).Inc()

		logger.Error(err, "unable to update status")

		// Return error
		return ctrl.Result{}, err
	}

	logger.Info("Reconcile done")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&postgresqlv1alpha1.PostgresqlBackup{}).
		Named("postgresqlbackup").
		Complete(r)
}
