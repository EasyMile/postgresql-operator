package postgresqlengineconfiguration

import (
	"context"
	"fmt"
	"time"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/pkg/apis/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/pkg/config"
	"github.com/easymile/postgresql-operator/pkg/postgres"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_postgresqlengineconfiguration")

const (
	RequeueDelayErrorSeconds = 5 * time.Second
	ControllerName           = "postgresqlengineconfiguration-controller"
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

	// Deletion case
	if !instance.GetDeletionTimestamp().IsZero() {
		// Need to delete
		// TODO Need to check if linked subresources exist
		// Clean finalizer
		instance.SetFinalizers(nil)
		// Update CR
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
		return reconcile.Result{}, nil
	}

	// Creation or update case

	// Check if the reconcile loop wasn't recall just because of update status
	if instance.Status.Phase == postgresqlv1alpha1.ValidatedPhase && instance.Status.LastValidatedTime != "" {
		dur, err := time.ParseDuration(instance.Spec.CheckInterval)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}

		now := time.Now()
		lastValidatedTime, err := time.Parse(time.RFC3339, instance.Status.LastValidatedTime)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
		if now.Sub(lastValidatedTime) < dur {
			newWaitDuration := now.Add(dur).Sub(now)
			return reconcile.Result{Requeue: true, RequeueAfter: newWaitDuration}, err
		}
	}

	// Add default values and/or finalizer if needed
	err = r.updateInstance(instance)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Change status
	r.recorder.Event(instance, "Normal", "Validating", "Validating engine connection")

	// Get secret for user/password
	secret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.SecretName, Namespace: instance.Namespace}, secret)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Got secret
	// Check that secret is valid
	user := string(secret.Data["user"])
	password := string(secret.Data["password"])
	if user == "" || password == "" {
		return r.manageError(
			reqLogger,
			instance,
			fmt.Errorf("secret %s must contain \"user\" and \"password\" values", instance.Spec.SecretName),
		)
	}

	// Create PG object
	pg := postgres.NewPG(
		instance.Spec.Host,
		user,
		password,
		instance.Spec.UriArgs,
		instance.Spec.DefaultDatabase,
		instance.Spec.Provider,
		reqLogger,
	)

	// Try to connect
	err = pg.Ping()
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	return r.manageSuccess(reqLogger, instance)
}

func (r *ReconcilePostgresqlEngineConfiguration) updateInstance(instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) error {
	// Add default values
	needUpdateDefaultValues := r.addDefaultValues(instance)

	// Add finalizer
	needUpdateFinalizer := r.addFinalizer(instance)

	// Check if update is needed
	if needUpdateDefaultValues || needUpdateFinalizer {
		return r.client.Update(context.TODO(), instance)
	}

	return nil
}

func (r *ReconcilePostgresqlEngineConfiguration) addFinalizer(instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) bool {
	if len(instance.GetFinalizers()) < 1 && instance.GetDeletionTimestamp() == nil {
		instance.SetFinalizers([]string{config.Finalizer})
		return true
	}
	return false
}

// Add default values here to be saved in reconcile loop in order to help people to debug
func (r *ReconcilePostgresqlEngineConfiguration) addDefaultValues(instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) bool {
	needUpdate := false
	// Check port
	if instance.Spec.Port == 0 {
		needUpdate = true
		instance.Spec.Port = 5432
	}
	// Check default database
	if instance.Spec.DefaultDatabase == "" {
		needUpdate = true
		// In classic pg, postgres is a default database
		instance.Spec.DefaultDatabase = "postgres"
	}
	// Check "check interval"
	if instance.Spec.CheckInterval == "" {
		needUpdate = true
		instance.Spec.CheckInterval = "30s"
	}

	return needUpdate
}

func (r *ReconcilePostgresqlEngineConfiguration) manageError(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlEngineConfiguration, issue error) (reconcile.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = postgresqlv1alpha1.FailedPhase

	// Update object
	err := r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		logger.Error(err, "unable to update status")
	}

	// Requeue
	return reconcile.Result{
		RequeueAfter: RequeueDelayErrorSeconds,
		Requeue:      true,
	}, nil
}

func (r *ReconcilePostgresqlEngineConfiguration) manageSuccess(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) (reconcile.Result, error) {
	// Try to parse duration
	dur, err := time.ParseDuration(instance.Spec.CheckInterval)
	if err != nil {
		return r.manageError(logger, instance, err)
	}

	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = postgresqlv1alpha1.ValidatedPhase
	instance.Status.LastValidatedTime = time.Now().UTC().Format(time.RFC3339)

	// Update object
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		logger.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: RequeueDelayErrorSeconds,
			Requeue:      true,
		}, nil
	}

	logger.Info("Reconcile done")
	return reconcile.Result{RequeueAfter: dur, Requeue: true}, nil
}
