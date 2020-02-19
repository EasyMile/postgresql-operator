package postgresqldatabase

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
	"github.com/thoas/go-funk"
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

const (
	RequeueDelayErrorSeconds   = 5 * time.Second
	RequeueDelaySuccessSeconds = 10 * time.Second
	ControllerName             = "postgresqldatabase-controller"
	readerPrivs                = "SELECT"
	writerPrivs                = "SELECT,INSERT,DELETE,UPDATE"
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
	pgEngCfg, err := utils.FindPgEngineCfg(r.client, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Check that postgres engine configuration is ready before continue but only if it is the first time
	// If not, requeue event with a short delay (1 second)
	if instance.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase && !pgEngCfg.Status.Ready {
		reqLogger.Info("PostgresqlEngineConfiguration not ready, waiting for it")
		r.recorder.Event(instance, "Warning", "Processing", "Processing stopped because PostgresqlEngineConfiguration isn't ready. Waiting for it.")
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: time.Second,
		}, nil
	}

	// Get secret linked to PostgresqlEngineConfiguration CR
	secret, err := utils.FindSecretPgEngineCfg(r.client, pgEngCfg)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Add finalizer and owners
	err = r.updateInstance(instance, pgEngCfg)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Create PG instance
	pg := utils.CreatePgInstance(reqLogger, secret.Data, &pgEngCfg.Spec)

	owner := instance.Spec.MasterRole
	if owner == "" {
		owner = fmt.Sprintf("%s-owner", instance.Spec.Database)
	}
	// Create owner role
	err = r.manageOwnerRole(pg, owner, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, errors.NewInternalError(err))
	}

	// Create or update database
	err = r.manageDBCreationOrUpdate(pg, instance, owner)
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

func (r *ReconcilePostgresqlDatabase) manageDBCreationOrUpdate(pg postgres.PG, instance *postgresqlv1alpha1.PostgresqlDatabase, owner string) error {
	// Check if database was already created in the past
	if instance.Status.Database != "" {
		exists, err := pg.IsDatabaseExist(instance.Status.Database)
		if err != nil {
			return err
		}
		// Check if "old" already exists and need to be renamed
		// If needed, rename and let create db do his job
		if exists && instance.Spec.Database != instance.Status.Database {
			// Rename
			err = pg.RenameDatabase(instance.Status.Database, instance.Spec.Database)
			if err != nil {
				return err
			}
		}
	}

	// Create database
	err := pg.CreateDB(instance.Spec.Database, owner)
	if err != nil {
		return err
	}

	// Update status
	instance.Status.Database = instance.Spec.Database

	return nil
}

func (r *ReconcilePostgresqlDatabase) manageDropDatabase(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	// Try to find PostgresqlEngineConfiguration CR
	pgEngCfg, err := utils.FindPgEngineCfg(r.client, instance)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	// In case of not found => Can't delete => skip
	if errors.IsNotFound(err) {
		logger.Error(err, "can't delete database because PostgresEngineConfiguration didn't exists anymore")
		return nil
	}

	// Get secret linked to PostgresqlEngineConfiguration CR
	secret, err := utils.FindSecretPgEngineCfg(r.client, pgEngCfg)
	if err != nil {
		return err
	}

	// Create PG instance
	pg := utils.CreatePgInstance(logger, secret.Data, &pgEngCfg.Spec)

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
	// Check if wait linked resources deletion flag is enabled
	if instance.Spec.WaitLinkedResourcesDeletion {
		// Check if there are linked resource linked to this
		existingUser, err := r.getAnyUserLinked(instance)
		if err != nil {
			return false, err
		}
		if existingUser != nil {
			// Wait for children removal
			err := fmt.Errorf("cannot remove resource because found user %s in namespace %s linked to this resource and wait for deletion flag is enabled", existingUser.Name, existingUser.Namespace)
			return false, err
		}
	}

	// Check if drop on delete flag is enabled
	if instance.Spec.DropOnDelete {
		return true, nil
	}

	// Default case is no !
	return false, nil
}

func (r *ReconcilePostgresqlDatabase) getAnyUserLinked(instance *postgresqlv1alpha1.PostgresqlDatabase) (*postgresqlv1alpha1.PostgresqlUser, error) {
	// Initialize postgres user list
	userL := postgresqlv1alpha1.PostgresqlUserList{}
	// Requests for list of users
	err := r.client.List(context.TODO(), &userL)
	if err != nil {
		return nil, err
	}
	// Loop over the list
	for _, user := range userL.Items {
		// Check db is linked to pgdatabase
		if user.Spec.Database.Name == instance.Name && (user.Spec.Database.Namespace == instance.Namespace || user.Namespace == instance.Namespace) {
			return &user, nil
		}
	}
	return nil, nil
}

func (r *ReconcilePostgresqlDatabase) updateInstance(instance *postgresqlv1alpha1.PostgresqlDatabase, pgEngCfg *postgresqlv1alpha1.PostgresqlEngineConfiguration) error {
	// Deep copy
	copy := instance.DeepCopy()

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
	// Check if role was already created in the past
	if instance.Status.Roles.Reader != "" {
		exists, err := pg.IsRoleExist(instance.Status.Roles.Reader)
		if err != nil {
			return err
		}
		// Check if "old" already exists and need to be renamed
		// if needed rename and let create role do his job
		if exists && reader != instance.Status.Roles.Reader {
			// Rename
			err = pg.RenameRole(instance.Status.Roles.Reader, reader)
			if err != nil {
				return err
			}
		}
	}

	err := pg.CreateGroupRole(reader)
	if err != nil {
		return err
	}
	// Update status
	instance.Status.Roles.Reader = reader
	return nil
}

func (r *ReconcilePostgresqlDatabase) manageWriterRole(pg postgres.PG, writer string, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	// Check if role was already created in the past
	if instance.Status.Roles.Writer != "" {
		exists, err := pg.IsRoleExist(instance.Status.Roles.Writer)
		if err != nil {
			return err
		}
		// Check if "old" already exists and need to be renamed
		// if needed rename and let create role do his job
		if exists && writer != instance.Status.Roles.Writer {
			// Rename
			err = pg.RenameRole(instance.Status.Roles.Writer, writer)
			if err != nil {
				return err
			}
		}
	}

	err := pg.CreateGroupRole(writer)
	if err != nil {
		return err
	}
	// Update status
	instance.Status.Roles.Writer = writer
	return nil
}

func (r *ReconcilePostgresqlDatabase) manageOwnerRole(pg postgres.PG, owner string, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
	// Check if role was already created in the past
	if instance.Status.Roles.Owner != "" {
		exists, err := pg.IsRoleExist(instance.Status.Roles.Owner)
		if err != nil {
			return err
		}
		// Check if "old" already exists and need to be renamed
		// if needed rename and let create role do his job
		if exists && owner != instance.Status.Roles.Owner {
			// Rename
			err = pg.RenameRole(instance.Status.Roles.Owner, owner)
			if err != nil {
				return err
			}
		}
	}

	err := pg.CreateGroupRole(owner)
	if err != nil {
		return err
	}
	// Update status
	instance.Status.Roles.Owner = owner
	return nil
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
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: RequeueDelaySuccessSeconds,
	}, nil
}
