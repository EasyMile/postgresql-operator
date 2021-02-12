package postgresqlengineconfiguration

import (
	"context"
	"fmt"
	"reflect"
	"time"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/pkg/apis/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/pkg/config"
	"github.com/easymile/postgresql-operator/pkg/controller/utils"
	"github.com/easymile/postgresql-operator/pkg/postgres"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_postgresqlengineconfiguration")

const (
	RequeueDelayErrorNumberSeconds = 5
	ControllerName                 = "postgresqlengineconfiguration-controller"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PostgresqlEngineConfiguration Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePostgresqlEngineConfiguration{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor(ControllerName),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(ControllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PostgresqlEngineConfiguration
	err = c.Watch(&source.Kind{Type: &postgresqlv1alpha1.PostgresqlEngineConfiguration{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePostgresqlEngineConfiguration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePostgresqlEngineConfiguration{}

// ReconcilePostgresqlEngineConfiguration reconciles a PostgresqlEngineConfiguration object
type ReconcilePostgresqlEngineConfiguration struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a PostgresqlEngineConfiguration object and makes changes based on the state read
// and what is in the PostgresqlEngineConfiguration.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePostgresqlEngineConfiguration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PostgresqlEngineConfiguration")

	// Fetch the PostgresqlEngineConfiguration instance
	instance := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Original patch
	originalPatch := client.MergeFrom(instance.DeepCopy())

	// Deletion case
	if !instance.GetDeletionTimestamp().IsZero() {
		// Need to delete
		// Check if wait linked resources deletion flag is enabled
		if instance.Spec.WaitLinkedResourcesDeletion {
			// Check if there are linked resource linked to this
			existingDb, err := r.getAnyDatabaseLinked(instance)
			if err != nil {
				return r.manageError(reqLogger, instance, originalPatch, err)
			}
			if existingDb != nil {
				// Wait for children removal
				err := fmt.Errorf("cannot remove resource because found database %s in namespace %s linked to this resource and wait for deletion flag is enabled", existingDb.Name, existingDb.Namespace)
				return r.manageError(reqLogger, instance, originalPatch, err)
			}
		}
		// Close all saved pools for that pgec
		err = postgres.CloseAllSavedPoolsForName(
			utils.CreateNameKeyForSavedPools(instance.Name, instance.Namespace),
		)
		// Check error
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
		// Clean finalizer
		controllerutil.RemoveFinalizer(instance, config.Finalizer)
		// Update CR
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
		return reconcile.Result{}, nil
	}

	// Creation or update case

	// Check if the reconcile loop wasn't recall just because of update status
	if instance.Status.Phase == postgresqlv1alpha1.EngineValidatedPhase && instance.Status.LastValidatedTime != "" {
		dur, err := time.ParseDuration(instance.Spec.CheckInterval)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, errors.NewInternalError(err))
		}

		now := time.Now()
		lastValidatedTime, err := time.Parse(time.RFC3339, instance.Status.LastValidatedTime)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, errors.NewInternalError(err))
		}

		// Check if reconcile was called before interval
		if now.Sub(lastValidatedTime) < dur {
			// Called before
			// Need to calculate hash to know if something has changed
			hash, err := utils.CalculateHash(instance.Spec)
			if err != nil {
				return r.manageError(reqLogger, instance, originalPatch, errors.NewInternalError(err))
			}

			// Compare hash to check if spec has changed before interval
			if instance.Status.Hash == hash {
				// Not changed => Requeue
				newWaitDuration := now.Add(dur).Sub(now)
				reqLogger.Info("Reconcile skipped because called before check interval and nothing has changed")
				return reconcile.Result{Requeue: true, RequeueAfter: newWaitDuration}, err
			}
		}
	}

	// Add default values and/or finalizer if needed
	err = r.updateInstance(instance)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Calculate hash for status (this time is to update it in status)
	hash, err := utils.CalculateHash(instance.Spec)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}
	// Need to check if status hash is the same or not to force renew or not
	if hash != instance.Status.Hash {
		err = postgres.CloseAllSavedPoolsForName(
			utils.CreateNameKeyForSavedPools(instance.Name, instance.Namespace),
		)
		// Check error
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
	}
	// Save new hash
	instance.Status.Hash = hash

	// Get secret for user/password
	secret, err := utils.FindSecretPgEngineCfg(r.client, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Got secret
	// Check that secret is valid
	user := string(secret.Data["user"])
	password := string(secret.Data["password"])
	if user == "" || password == "" {
		return r.manageError(
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
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	return r.manageSuccess(reqLogger, instance, originalPatch)
}

func (r *ReconcilePostgresqlEngineConfiguration) getAnyDatabaseLinked(instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) (*postgresqlv1alpha1.PostgresqlDatabase, error) {
	// Initialize postgres database list
	dbL := postgresqlv1alpha1.PostgresqlDatabaseList{}
	// Requests for list of databases
	err := r.client.List(context.TODO(), &dbL)
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

func (r *ReconcilePostgresqlEngineConfiguration) updateInstance(instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) error {
	// Deep copy
	copy := instance.DeepCopy()

	// Add default values
	r.addDefaultValues(instance)

	// Add finalizer
	controllerutil.AddFinalizer(instance, config.Finalizer)

	// Check if update is needed
	if !reflect.DeepEqual(instance, copy) {
		return r.client.Update(context.TODO(), instance)
	}

	return nil
}

// Add default values here to be saved in reconcile loop in order to help people to debug
func (r *ReconcilePostgresqlEngineConfiguration) addDefaultValues(instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) {
	// Check port
	if instance.Spec.Port == 0 {
		instance.Spec.Port = 5432
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
}

func (r *ReconcilePostgresqlEngineConfiguration) manageError(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlEngineConfiguration, originalPatch client.Patch, issue error) (reconcile.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = postgresqlv1alpha1.EngineFailedPhase

	// Patch status
	err := r.client.Status().Patch(context.TODO(), instance, originalPatch)
	if err != nil {
		logger.Error(err, "unable to update status")
	}

	// Requeue
	return reconcile.Result{
		RequeueAfter: RequeueDelayErrorNumberSeconds * time.Second,
		Requeue:      true,
	}, nil
}

func (r *ReconcilePostgresqlEngineConfiguration) manageSuccess(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlEngineConfiguration, originalPatch client.Patch) (reconcile.Result, error) {
	// Try to parse duration
	dur, err := time.ParseDuration(instance.Spec.CheckInterval)
	if err != nil {
		return r.manageError(logger, instance, originalPatch, errors.NewInternalError(err))
	}

	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = postgresqlv1alpha1.EngineValidatedPhase
	instance.Status.LastValidatedTime = time.Now().UTC().Format(time.RFC3339)

	// Patch status
	err = r.client.Status().Patch(context.TODO(), instance, originalPatch)
	if err != nil {
		logger.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: RequeueDelayErrorNumberSeconds * time.Second,
			Requeue:      true,
		}, nil
	}

	logger.Info("Reconcile done")
	return reconcile.Result{RequeueAfter: dur, Requeue: true}, nil
}
