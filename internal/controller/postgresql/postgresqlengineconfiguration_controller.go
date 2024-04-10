/*
Copyright 2022.

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
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/internal/controller/config"
	"github.com/easymile/postgresql-operator/internal/controller/postgresql/postgres"
	"github.com/easymile/postgresql-operator/internal/controller/utils"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DefaultPGPort      = 5432
	DefaultBouncerPort = 6432
)

// PostgresqlEngineConfigurationReconciler reconciles a PostgresqlEngineConfiguration object.
type PostgresqlEngineConfigurationReconciler struct {
	Recorder record.EventRecorder
	client.Client
	Scheme                              *runtime.Scheme
	ControllerRuntimeDetailedErrorTotal *prometheus.CounterVec
	Log                                 logr.Logger
	ControllerName                      string
}

//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlengineconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlengineconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlengineconfigurations/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the PostgresqlEngineConfiguration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *PostgresqlEngineConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:wsl // it is like that
	// Issue with this logger: controller and controllerKind are incorrect
	// Build another logger from upper to fix this.
	// reqLogger := log.FromContext(ctx)

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling PostgresqlEngineConfiguration")

	// Fetch the PostgresqlEngineConfiguration instance
	instance := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}

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

	// Deletion case
	if !instance.GetDeletionTimestamp().IsZero() {
		// Need to delete
		// Check if wait linked resources deletion flag is enabled
		if instance.Spec.WaitLinkedResourcesDeletion {
			// Check if there are linked resource linked to this
			existingDB, err := r.getAnyDatabaseLinked(ctx, instance) //nolint:govet // Shadow err
			if err != nil {
				return r.manageError(ctx, reqLogger, instance, originalPatch, err)
			}

			if existingDB != nil {
				// Wait for children removal
				err = fmt.Errorf("cannot remove resource because found database %s in namespace %s linked to this resource and wait for deletion flag is enabled", existingDB.Name, existingDB.Namespace)

				return r.manageError(ctx, reqLogger, instance, originalPatch, err)
			}
		}
		// Close all saved pools for that pgec
		err = postgres.CloseAllSavedPoolsForName(
			utils.CreateNameKeyForSavedPools(instance.Name, instance.Namespace),
		)
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
		// Clean finalizer
		controllerutil.RemoveFinalizer(instance, config.Finalizer)
		// Update CR
		err = r.Update(ctx, instance)
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}

		return ctrl.Result{}, nil
	}

	// Creation or update case

	// Check if the reconcile loop wasn't recall just because of update status
	if instance.Status.Phase == postgresqlv1alpha1.EngineValidatedPhase && instance.Status.LastValidatedTime != "" {
		dur, err := time.ParseDuration(instance.Spec.CheckInterval) //nolint:govet // Shadow err
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
		}

		now := time.Now()

		lastValidatedTime, err := time.Parse(time.RFC3339, instance.Status.LastValidatedTime)
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
		}

		// Check if reconcile was called before interval
		if now.Sub(lastValidatedTime) < dur {
			// Called before
			// Need to calculate hash to know if something has changed
			hash, err := utils.CalculateHash(instance.Spec)
			if err != nil {
				return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
			}

			// Compare hash to check if spec has changed before interval
			if instance.Status.Hash == hash {
				// Not changed => Requeue
				newWaitDuration := now.Add(dur).Sub(now)

				reqLogger.Info("Reconcile skipped because called before check interval and nothing has changed")

				return ctrl.Result{Requeue: true, RequeueAfter: newWaitDuration}, err
			}
		}
	}

	// Add default values and/or finalizer if needed
	updated, err := r.updateInstance(ctx, instance)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}
	// Check if it has been updated in order to stop this reconcile loop here for the moment
	if updated {
		return ctrl.Result{}, nil
	}

	// Calculate hash for status (this time is to update it in status)
	hash, err := utils.CalculateHash(instance.Spec)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}
	// Need to check if status hash is the same or not to force renew or not
	if hash != instance.Status.Hash {
		err = postgres.CloseAllSavedPoolsForName(
			utils.CreateNameKeyForSavedPools(instance.Name, instance.Namespace),
		)
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
	}
	// Save new hash
	instance.Status.Hash = hash

	// Get secret for user/password
	secret, err := utils.FindSecretPgEngineCfg(ctx, r.Client, instance)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Got secret
	// Check that secret is valid
	user := string(secret.Data["user"])
	password := string(secret.Data["password"])

	if user == "" || password == "" {
		return r.manageError(
			ctx,
			reqLogger,
			instance,
			originalPatch,
			fmt.Errorf("secret %s must contain \"user\" and \"password\" values", instance.Spec.SecretName),
		)
	}

	// Create PG object
	pg := utils.CreatePgInstance(reqLogger, secret.Data, instance)

	// Try to connect
	err = pg.Ping()
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	return r.manageSuccess(ctx, reqLogger, instance, originalPatch)
}

func (r *PostgresqlEngineConfigurationReconciler) getAnyDatabaseLinked(
	ctx context.Context,
	instance *postgresqlv1alpha1.PostgresqlEngineConfiguration,
) (*postgresqlv1alpha1.PostgresqlDatabase, error) {
	// Initialize postgres database list
	dbL := postgresqlv1alpha1.PostgresqlDatabaseList{}
	// Requests for list of databases
	err := r.List(ctx, &dbL)
	if err != nil {
		return nil, err
	}
	// Loop over the list
	for _, db := range dbL.Items {
		// Check db is linked to pgengineconfig
		if db.Spec.EngineConfiguration.Name == instance.Name && (db.Spec.EngineConfiguration.Namespace == instance.Namespace || db.Namespace == instance.Namespace) {
			return &db, nil
		}
	}

	return nil, nil
}

func (r *PostgresqlEngineConfigurationReconciler) updateInstance(
	ctx context.Context,
	instance *postgresqlv1alpha1.PostgresqlEngineConfiguration,
) (bool, error) {
	// Deep copy
	oCopy := instance.DeepCopy()

	// Add default values
	r.addDefaultValues(instance)

	// Add finalizer
	controllerutil.AddFinalizer(instance, config.Finalizer)

	// Check if update is needed
	if !reflect.DeepEqual(instance, oCopy) {
		return true, r.Update(ctx, instance)
	}

	return false, nil
}

// Add default values here to be saved in reconcile loop in order to help people to debug.
func (*PostgresqlEngineConfigurationReconciler) addDefaultValues(instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) {
	// Check port
	if instance.Spec.Port == 0 {
		instance.Spec.Port = DefaultPGPort
	}
	// Check default database
	if instance.Spec.DefaultDatabase == "" {
		// In classic pg, postgres is a default database
		instance.Spec.DefaultDatabase = "postgres"
	}
	// Check "check interval"
	if instance.Spec.CheckInterval == "" {
		instance.Spec.CheckInterval = "30s"
	}

	// Check if user connections aren't set to init it
	if instance.Spec.UserConnections == nil {
		instance.Spec.UserConnections = &postgresqlv1alpha1.UserConnections{}
	}

	// Check if primary user connections aren't set to init it
	if instance.Spec.UserConnections.PrimaryConnection == nil {
		instance.Spec.UserConnections.PrimaryConnection = &postgresqlv1alpha1.GenericUserConnection{
			Host:    instance.Spec.Host,
			URIArgs: instance.Spec.URIArgs,
			Port:    instance.Spec.Port,
		}
	}

	// Check if primary user connections are set and fully valued
	if instance.Spec.UserConnections.PrimaryConnection != nil {
		// Check port
		if instance.Spec.UserConnections.PrimaryConnection.Port == 0 {
			instance.Spec.UserConnections.PrimaryConnection.Port = DefaultPGPort
		}
	}

	// Check if bouncer user connections are set and fully valued
	if instance.Spec.UserConnections.BouncerConnection != nil {
		// Check port
		if instance.Spec.UserConnections.BouncerConnection.Port == 0 {
			instance.Spec.UserConnections.BouncerConnection.Port = DefaultBouncerPort
		}
	}

	// Loop over replica connections
	for _, item := range instance.Spec.UserConnections.ReplicaConnections {
		// Check port
		if item.Port == 0 {
			item.Port = DefaultPGPort
		}
	}

	// Loop over replica bouncer connections
	for _, item := range instance.Spec.UserConnections.ReplicaBouncerConnections {
		// Check port
		if item.Port == 0 {
			item.Port = DefaultBouncerPort
		}
	}
}

func (r *PostgresqlEngineConfigurationReconciler) manageError(
	ctx context.Context,
	logger logr.Logger,
	instance *postgresqlv1alpha1.PostgresqlEngineConfiguration,
	originalPatch client.Patch,
	issue error,
) (ctrl.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.Recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = postgresqlv1alpha1.EngineFailedPhase

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

func (r *PostgresqlEngineConfigurationReconciler) manageSuccess(
	ctx context.Context,
	logger logr.Logger,
	instance *postgresqlv1alpha1.PostgresqlEngineConfiguration,
	originalPatch client.Patch,
) (ctrl.Result, error) {
	// Try to parse duration
	dur, err := time.ParseDuration(instance.Spec.CheckInterval)
	if err != nil {
		return r.manageError(ctx, logger, instance, originalPatch, errors.NewInternalError(err))
	}

	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = postgresqlv1alpha1.EngineValidatedPhase
	instance.Status.LastValidatedTime = time.Now().UTC().Format(time.RFC3339)

	// Patch status
	err = r.Status().Patch(ctx, instance, originalPatch)
	if err != nil {
		// Increase fail counter
		r.ControllerRuntimeDetailedErrorTotal.WithLabelValues(r.ControllerName, instance.Namespace, instance.Name).Inc()

		logger.Error(err, "unable to update status")

		// Return error
		return ctrl.Result{}, err
	}

	logger.Info("Reconcile done")

	return ctrl.Result{RequeueAfter: dur, Requeue: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlEngineConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&postgresqlv1alpha1.PostgresqlEngineConfiguration{}).
		Complete(r)
}
