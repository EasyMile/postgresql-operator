package postgresqldatabase

import (
	"context"
	"fmt"
	"reflect"
	"time"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/pkg/apis/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/pkg/config"
	"github.com/easymile/postgresql-operator/pkg/postgres"
	"github.com/go-logr/logr"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

const (
	RequeueDelayErrorSeconds = 5 * time.Second
	ControllerName           = "postgresqldatabase-controller"
	readerPrivs              = "SELECT"
	writerPrivs              = "SELECT,INSERT,DELETE,UPDATE"
)

var log = logf.Log.WithName("controller_postgresqldatabase")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PostgresqlDatabase Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePostgresqlDatabase{
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

	// Watch for changes to primary resource PostgresqlDatabase
	err = c.Watch(&source.Kind{Type: &postgresqlv1alpha1.PostgresqlDatabase{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePostgresqlDatabase implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePostgresqlDatabase{}

// ReconcilePostgresqlDatabase reconciles a PostgresqlDatabase object
type ReconcilePostgresqlDatabase struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a PostgresqlDatabase object and makes changes based on the state read
// and what is in the PostgresqlDatabase.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePostgresqlDatabase) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PostgresqlDatabase")

	// Fetch the PostgresqlDatabase instance
	instance := &postgresqlv1alpha1.PostgresqlDatabase{}
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
		// Deletion in progress detected
		// Test should delete database
		shouldDelete, err := r.shouldDropDatabase(instance)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
		if shouldDelete {
			// Drop database
			err := r.manageDropDatabase(reqLogger, instance)
			if err != nil {
				return r.manageError(reqLogger, instance, err)
			}
		}
		// Remove finalizer
		controllerutil.RemoveFinalizer(instance, config.Finalizer)
		// Update CR
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
		// Stop reconcile
		return reconcile.Result{}, nil
	}

	// Creation case

	// Try to find PostgresqlEngineConfiguration CR
	pgEngCfg, err := r.findPgEngineCfg(instance)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Get secret linked to PostgresqlEngineConfiguration CR
	secret, err := r.findSecretPgEngineCfg(pgEngCfg)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Add finalizer and owners
	err = r.updateInstance(instance, pgEngCfg)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Create PG instance
	pg := r.createPgInstance(reqLogger, secret.Data, &pgEngCfg.Spec)

	owner := instance.Spec.MasterRole
	if owner == "" {
		owner = fmt.Sprintf("%s-group", instance.Spec.Database)
	}
	// Create owner role
	err = r.manageOwnerRole(pg, owner, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, errors.NewInternalError(err))
	}

	// Create database
	// TODO Need to manage spec change
	// Because if spec has changed, "old" database won't be removed
	err = pg.CreateDB(instance.Spec.Database, owner)
	if err != nil {
		return r.manageError(reqLogger, instance, errors.NewInternalError(err))
	}

	// Create reader role
	reader := fmt.Sprintf("%s-reader", instance.Spec.Database)
	err = r.manageReaderRole(pg, reader, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, errors.NewInternalError(err))
	}

	// Create writer role
	writer := fmt.Sprintf("%s-writer", instance.Spec.Database)
	err = r.manageWriterRole(pg, writer, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, errors.NewInternalError(err))
	}

	// Manage extensions
	err = r.manageExtensions(pg, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, errors.NewInternalError(err))
	}

	// Manage schema
	err = r.manageSchemas(pg, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, errors.NewInternalError(err))
	}

	return r.manageSuccess(reqLogger, instance)
}

func (r *ReconcilePostgresqlDatabase) manageDropDatabase(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	// Try to find PostgresqlEngineConfiguration CR
	pgEngCfg, err := r.findPgEngineCfg(instance)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	// In case of not found => Can't delete => skip
	if errors.IsNotFound(err) {
		logger.Error(err, "can't delete database because PostgresEngineConfiguration didn't exists anymore")
		return nil
	}

	// Get secret linked to PostgresqlEngineConfiguration CR
	secret, err := r.findSecretPgEngineCfg(pgEngCfg)
	if err != nil {
		return err
	}

	// Create PG instance
	pg := r.createPgInstance(logger, secret.Data, &pgEngCfg.Spec)

	// Drop roles first

	// Drop owner
	if instance.Status.Roles.Owner != "" {
		err = pg.DropRole(instance.Status.Roles.Owner, pg.GetUser(), instance.Spec.Database)
		if err != nil {
			return err
		}
		// Clear status
		instance.Status.Roles.Owner = ""
	}
	// Drop writer
	if instance.Status.Roles.Writer != "" {
		err = pg.DropRole(instance.Status.Roles.Writer, pg.GetUser(), instance.Spec.Database)
		if err != nil {
			return err
		}
		// Clear status
		instance.Status.Roles.Writer = ""
	}
	// Drop reader
	if instance.Status.Roles.Reader != "" {
		err = pg.DropRole(instance.Status.Roles.Reader, pg.GetUser(), instance.Spec.Database)
		if err != nil {
			return err
		}
		// Clear status
		instance.Status.Roles.Reader = ""
	}

	// Drop database
	err = pg.DropDatabase(instance.Spec.Database)
	return err
}

func (r *ReconcilePostgresqlDatabase) shouldDropDatabase(instance *postgresqlv1alpha1.PostgresqlDatabase) (bool, error) {
	// Check if drop on delete flag is enabled
	if instance.Spec.DropOnDelete {
		return true, nil
	}

	// Check if other postgresql database CR ask for the same database
	crList := postgresqlv1alpha1.PostgresqlDatabaseList{}
	err := r.client.List(context.TODO(), &crList)
	// Check if error exists
	if err != nil {
		return false, err
	}
	// Check
	for _, cr := range crList.Items {
		// Check if cr is equal to actual instance
		// If yes, skip it
		if cr.Name == instance.Name && cr.Namespace == instance.Namespace {
			continue
		}
		// Check if database is the same
		// If yes, stop
		if cr.Spec.Database == instance.Spec.Database {
			return false, nil
		}
	}

	// Default case is no !
	return false, nil
}

func (r *ReconcilePostgresqlDatabase) updateInstance(instance *postgresqlv1alpha1.PostgresqlDatabase, pgEngCfg *postgresqlv1alpha1.PostgresqlEngineConfiguration) error {
	// Deep copy
	copy := instance.DeepCopy()

	// Add owner
	err := controllerutil.SetControllerReference(pgEngCfg, instance, r.scheme)
	if err != nil {
		return err
	}

	// Add finalizer
	controllerutil.AddFinalizer(instance, config.Finalizer)

	// Check if update is needed
	if !reflect.DeepEqual(copy.ObjectMeta, instance.ObjectMeta) {
		return r.client.Update(context.TODO(), instance)
	}

	return nil
}

func (r *ReconcilePostgresqlDatabase) manageSchemas(pg postgres.PG, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	// Check if were deleted from list and asked to be deleted
	if instance.Status.Schemas != nil && instance.Spec.Schemas.DropOnOnDelete {
		newStatusSchemas := make([]string, 0)
		// Look in status schemas list if there are differences
		for _, schemaExt := range instance.Status.Schemas {
			if funk.ContainsString(instance.Spec.Schemas.List, schemaExt) {
				// Still present in schemas list
				// Keep it
				newStatusSchemas = append(newStatusSchemas, schemaExt)
				continue
			}
			// Not present anymore
			// Need to delete it
			err := pg.DropSchema(instance.Spec.Database, schemaExt, instance.Spec.Schemas.DeleteWithCascade)
			if err != nil {
				return err
			}
		}
		// Save new list
		instance.Status.Schemas = newStatusSchemas
	}

	// Manage schemas creation
	var (
		owner  = instance.Status.Roles.Owner
		reader = instance.Status.Roles.Reader
		writer = instance.Status.Roles.Writer
	)
	for _, schema := range instance.Spec.Schemas.List {
		// Check if schema was created. Skip if already added
		if funk.ContainsString(instance.Status.Schemas, schema) {
			continue
		}

		// Create schema
		err := pg.CreateSchema(instance.Spec.Database, owner, schema)
		if err != nil {
			return err
		}

		// Set privileges on schema
		err = pg.SetSchemaPrivileges(instance.Spec.Database, owner, reader, schema, readerPrivs)
		if err != nil {
			return err
		}
		err = pg.SetSchemaPrivileges(instance.Spec.Database, owner, writer, schema, writerPrivs)
		if err != nil {
			return err
		}

		instance.Status.Schemas = append(instance.Status.Schemas, schema)
	}
	return nil
}

func (r *ReconcilePostgresqlDatabase) manageExtensions(pg postgres.PG, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	// Check if were deleted from list and asked to be deleted
	if instance.Status.Extensions != nil && instance.Spec.Extensions.DropOnOnDelete {
		newStatusExtensions := make([]string, 0)
		// Look in status extensions list if there are differences
		for _, statusExt := range instance.Status.Extensions {
			if funk.ContainsString(instance.Spec.Extensions.List, statusExt) {
				// Still present in extensions list
				// Keep it
				newStatusExtensions = append(newStatusExtensions, statusExt)
				continue
			}
			// Not present anymore
			// Need to delete it
			err := pg.DropExtension(instance.Spec.Database, statusExt, instance.Spec.Extensions.DeleteWithCascade)
			if err != nil {
				return err
			}
		}
		// Save new list
		instance.Status.Extensions = newStatusExtensions
	}

	// Manage extensions creation
	for _, extension := range instance.Spec.Extensions.List {
		// Check if extension was added. Skip if already added
		if funk.ContainsString(instance.Status.Extensions, extension) {
			continue
		}
		// Execute create extension SQL statement
		err := pg.CreateExtension(instance.Spec.Database, extension)
		if err != nil {
			return err
		}
		instance.Status.Extensions = append(instance.Status.Extensions, extension)
	}

	return nil
}

func (r *ReconcilePostgresqlDatabase) manageReaderRole(pg postgres.PG, reader string, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	err := pg.CreateGroupRole(reader)
	if err != nil {
		return err
	}
	// Update status
	instance.Status.Roles.Reader = reader
	return nil
}

func (r *ReconcilePostgresqlDatabase) manageWriterRole(pg postgres.PG, writer string, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	err := pg.CreateGroupRole(writer)
	if err != nil {
		return err
	}
	// Update status
	instance.Status.Roles.Writer = writer
	return nil
}

func (r *ReconcilePostgresqlDatabase) manageOwnerRole(pg postgres.PG, owner string, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	err := pg.CreateGroupRole(owner)
	if err != nil {
		return err
	}
	// Check if previous owner was the same
	if instance.Status.Roles.Owner != "" && instance.Status.Roles.Owner != owner {
		// Drop old owner
		err = pg.DropRole(instance.Status.Roles.Owner, owner, instance.Spec.Database)
		if err != nil {
			return err
		}
	}
	// Update status
	instance.Status.Roles.Owner = owner
	return nil
}

// TODO put this into utils and rework controllers
func (r *ReconcilePostgresqlDatabase) createPgInstance(reqLogger logr.Logger, secretData map[string][]byte, spec *postgresqlv1alpha1.PostgresqlEngineConfigurationSpec) postgres.PG {
	user := string(secretData["user"])
	password := string(secretData["password"])
	return postgres.NewPG(
		spec.Host,
		user,
		password,
		spec.UriArgs,
		spec.DefaultDatabase,
		spec.Provider,
		reqLogger,
	)
}

// TODO put this into utils and rework controllers
func (r *ReconcilePostgresqlDatabase) findSecretPgEngineCfg(instance *postgresqlv1alpha1.PostgresqlEngineConfiguration) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.SecretName, Namespace: instance.Namespace}, secret)
	return secret, err
}

// TODO put this into utils and rework controllers
func (r *ReconcilePostgresqlDatabase) findPgEngineCfg(instance *postgresqlv1alpha1.PostgresqlDatabase) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, error) {
	// Try to get namespace from spec
	namespace := instance.Spec.EngineConfiguration.Namespace
	if namespace == "" {
		// Namespace not found, take it from instance namespace
		namespace = instance.Namespace
	}

	pgEngineCfg := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
	err := r.client.Get(context.TODO(), client.ObjectKey{
		Name:      instance.Spec.EngineConfiguration.Name,
		Namespace: namespace,
	}, pgEngineCfg)

	return pgEngineCfg, err
}

func (r *ReconcilePostgresqlDatabase) manageError(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlDatabase, issue error) (reconcile.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = postgresqlv1alpha1.DatabaseFailedPhase

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

func (r *ReconcilePostgresqlDatabase) manageSuccess(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlDatabase) (reconcile.Result, error) {
	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = postgresqlv1alpha1.DatabaseCreatedPhase

	// Update object
	err := r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		logger.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: RequeueDelayErrorSeconds,
			Requeue:      true,
		}, nil
	}

	logger.Info("Reconcile done")
	return reconcile.Result{}, nil
}
