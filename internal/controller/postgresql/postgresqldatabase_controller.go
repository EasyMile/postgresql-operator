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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/internal/controller/config"
	"github.com/easymile/postgresql-operator/internal/controller/postgresql/postgres"
	"github.com/easymile/postgresql-operator/internal/controller/utils"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thoas/go-funk"
)

const (
	readerPrivs               = "SELECT"
	writerPrivs               = "SELECT,INSERT,DELETE,UPDATE"
	defaultPGPublicSchemaName = "public"
)

// PostgresqlDatabaseReconciler reconciles a PostgresqlDatabase object.
type PostgresqlDatabaseReconciler struct {
	Recorder record.EventRecorder
	client.Client
	Scheme                              *runtime.Scheme
	ControllerRuntimeDetailedErrorTotal *prometheus.CounterVec
	Log                                 logr.Logger
	ControllerName                      string
}

//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqldatabases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqldatabases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqldatabases/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the PostgresqlDatabase object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *PostgresqlDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:wsl // it is like that
	// Issue with this logger: controller and controllerKind are incorrect
	// Build another logger from upper to fix this.
	// reqLogger := log.FromContext(ctx)

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling PostgresqlDatabase")

	// Fetch the PostgresqlDatabase instance
	instance := &postgresqlv1alpha1.PostgresqlDatabase{}

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
		// Deletion in progress detected
		// Close saved pools
		// This is done twice in the sequence, but function is idempotent => not a problem and should be kept otherwise a pool can survive
		err = utils.CloseDatabaseSavedPoolsForName(instance, instance.Spec.Database)
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}

		// Test should delete database
		shouldDelete, err := r.shouldDropDatabase(ctx, instance) //nolint:govet // Shadow err
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
		// Check if should delete database is flagged
		if shouldDelete {
			// Drop database
			err = r.manageDropDatabase(ctx, reqLogger, instance)
			if err != nil {
				return r.manageError(ctx, reqLogger, instance, originalPatch, err)
			}
		}
		// Remove finalizer
		controllerutil.RemoveFinalizer(instance, config.Finalizer)
		// Update CR
		err = r.Update(ctx, instance)
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
		// Stop reconcile
		return ctrl.Result{}, nil
	}

	// Creation case

	// Try to find PostgresqlEngineConfiguration CR
	pgEngCfg, err := utils.FindPgEngineCfg(ctx, r.Client, instance)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Check that postgres engine configuration is ready before continue but only if it is the first time
	// If not, requeue event with a short delay (1 second)
	if instance.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase && !pgEngCfg.Status.Ready {
		reqLogger.Info("PostgresqlEngineConfiguration not ready, waiting for it")
		r.Recorder.Event(instance, "Warning", "Processing", "Processing stopped because PostgresqlEngineConfiguration isn't ready. Waiting for it.")

		return ctrl.Result{}, nil
	}

	// Get secret linked to PostgresqlEngineConfiguration CR
	secret, err := utils.FindSecretPgEngineCfg(ctx, r.Client, pgEngCfg)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

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

	// Create PG instance
	pg := utils.CreatePgInstance(reqLogger, secret.Data, pgEngCfg)

	// Create all identifiers now to check length
	owner := instance.Spec.MasterRole
	if owner == "" {
		owner = fmt.Sprintf("%s-owner", instance.Spec.Database)
	}

	reader := fmt.Sprintf("%s-reader", instance.Spec.Database)
	writer := fmt.Sprintf("%s-writer", instance.Spec.Database)

	// Check identifier length
	if len(owner) > postgres.MaxIdentifierLength {
		errStr := fmt.Sprintf("identifier too long, must be <= 63, %s is %d character, must reduce master role or database name length", owner, len(owner))

		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewBadRequest(errStr))
	}

	if len(reader) > postgres.MaxIdentifierLength {
		errStr := fmt.Sprintf("identifier too long, must be <= 63, %s is %d character, must reduce database name length", reader, len(reader))

		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewBadRequest(errStr))
	}

	if len(writer) > postgres.MaxIdentifierLength {
		errStr := fmt.Sprintf("identifier too long, must be <= 63, %s is %d character, must reduce database name length", writer, len(writer))

		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewBadRequest(errStr))
	}

	// Create owner role
	err = r.manageOwnerRole(pg, owner, instance, pgEngCfg.Spec.AllowGrantAdminOption)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}

	// Create or update database
	err = r.manageDBCreationOrUpdate(pg, instance, owner)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}

	// Create reader role
	err = r.manageReaderRole(pg, reader, instance, pgEngCfg.Spec.AllowGrantAdminOption)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}

	// Create writer role
	err = r.manageWriterRole(pg, writer, instance, pgEngCfg.Spec.AllowGrantAdminOption)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}

	// Manage extensions
	err = r.manageExtensions(pg, instance)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}

	// Manage schema
	err = r.manageSchemas(pg, instance)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}

	return r.manageSuccess(ctx, reqLogger, instance, originalPatch)
}

func (*PostgresqlDatabaseReconciler) manageDBCreationOrUpdate(pg postgres.PG, instance *postgresqlv1alpha1.PostgresqlDatabase, owner string) error {
	// Check if database was already created in the past
	if instance.Status.Database != "" {
		// Check if database already exists
		exists, err := pg.IsDatabaseExist(instance.Status.Database)
		// Check error
		if err != nil {
			return err
		}
		// Check if "old" already exists and need to be renamed
		// If needed, rename and let create db do his job
		if exists && instance.Spec.Database != instance.Status.Database {
			// Close old saved pools
			err = utils.CloseDatabaseSavedPoolsForName(instance, instance.Status.Database)
			if err != nil {
				return err
			}
			// Rename
			err = pg.RenameDatabase(instance.Status.Database, instance.Spec.Database)
			if err != nil {
				return err
			}
		}
	}

	// Check if database already exists
	exists, err := pg.IsDatabaseExist(instance.Spec.Database)
	// Check error
	if err != nil {
		return err
	}
	// Check if exists
	if !exists {
		// Create database
		err := pg.CreateDB(instance.Spec.Database, owner)
		if err != nil {
			return err
		}
	} else {
		// Ensure owner is correct
		err := pg.ChangeDBOwner(instance.Spec.Database, owner)
		if err != nil {
			return err
		}
	}

	// Update status
	instance.Status.Database = instance.Spec.Database

	return nil
}

func (r *PostgresqlDatabaseReconciler) manageDropDatabase(
	ctx context.Context,
	logger logr.Logger,
	instance *postgresqlv1alpha1.PostgresqlDatabase,
) error {
	// Try to find PostgresqlEngineConfiguration CR
	pgEngCfg, err := utils.FindPgEngineCfg(ctx, r.Client, instance)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	// In case of not found => Can't delete => skip
	if errors.IsNotFound(err) {
		logger.Error(err, "can't delete database because PostgresEngineConfiguration didn't exists anymore")

		return nil
	}

	// Get secret linked to PostgresqlEngineConfiguration CR
	secret, err := utils.FindSecretPgEngineCfg(ctx, r.Client, pgEngCfg)
	if err != nil {
		return err
	}

	// Create PG instance
	pg := utils.CreatePgInstance(logger, secret.Data, pgEngCfg)

	// Drop roles first

	// Drop owner
	if instance.Status.Roles.Owner != "" {
		err = pg.DropRoleAndDropAndChangeOwnedBy(instance.Status.Roles.Owner, pg.GetUser(), instance.Spec.Database)
		if err != nil {
			return err
		}
		// Clear status
		instance.Status.Roles.Owner = ""
	}
	// Drop writer
	if instance.Status.Roles.Writer != "" {
		err = pg.DropRoleAndDropAndChangeOwnedBy(instance.Status.Roles.Writer, pg.GetUser(), instance.Spec.Database)
		if err != nil {
			return err
		}
		// Clear status
		instance.Status.Roles.Writer = ""
	}
	// Drop reader
	if instance.Status.Roles.Reader != "" {
		err = pg.DropRoleAndDropAndChangeOwnedBy(instance.Status.Roles.Reader, pg.GetUser(), instance.Spec.Database)
		if err != nil {
			return err
		}
		// Clear status
		instance.Status.Roles.Reader = ""
	}

	// Close saved pools for this database
	// This is done twice in the sequence, but function is idempotent => not a problem and should be kept otherwise a pool can survive
	err = utils.CloseDatabaseSavedPoolsForName(instance, instance.Spec.Database)
	if err != nil {
		return err
	}

	// Drop database
	return pg.DropDatabase(instance.Spec.Database)
}

func (r *PostgresqlDatabaseReconciler) shouldDropDatabase(
	ctx context.Context,
	instance *postgresqlv1alpha1.PostgresqlDatabase,
) (bool, error) {
	// Check if wait linked resources deletion flag is enabled
	if instance.Spec.WaitLinkedResourcesDeletion {
		// Check if there are user role linked resource linked to this
		existingUserRole, err := r.getAnyUserRoleLinked(ctx, instance)
		if err != nil {
			return false, err
		}

		if existingUserRole != nil {
			// Wait for children removal
			err = fmt.Errorf("cannot remove resource because found user role %s in namespace %s linked to this resource and wait for deletion flag is enabled", existingUserRole.Name, existingUserRole.Namespace)

			return false, err
		}

		// Check if there are user role linked resource linked to this
		existingPublication, err := r.getAnyPublicationLinked(ctx, instance)
		if err != nil {
			return false, err
		}

		if existingPublication != nil {
			// Wait for children removal
			err = fmt.Errorf("cannot remove resource because found publication %s in namespace %s linked to this resource and wait for deletion flag is enabled", existingPublication.Name, existingPublication.Namespace)

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

func (r *PostgresqlDatabaseReconciler) getAnyPublicationLinked(
	ctx context.Context,
	instance *postgresqlv1alpha1.PostgresqlDatabase,
) (*postgresqlv1alpha1.PostgresqlPublication, error) {
	// Initialize postgres user list
	list := postgresqlv1alpha1.PostgresqlPublicationList{}
	// Requests for list of users
	err := r.List(ctx, &list)
	if err != nil {
		return nil, err
	}
	// Loop over the list
	for _, item := range list.Items {
		// Check if db is linked to pgdatabase
		if item.Spec.Database.Name == instance.Name && (item.Spec.Database.Namespace == instance.Namespace || item.Namespace == instance.Namespace) {
			return &item, nil
		}
	}

	return nil, nil
}

func (r *PostgresqlDatabaseReconciler) getAnyUserRoleLinked(
	ctx context.Context,
	instance *postgresqlv1alpha1.PostgresqlDatabase,
) (*postgresqlv1alpha1.PostgresqlUserRole, error) {
	// Initialize postgres user list
	userL := postgresqlv1alpha1.PostgresqlUserRoleList{}
	// Requests for list of users
	err := r.List(ctx, &userL)
	if err != nil {
		return nil, err
	}
	// Loop over the list
	for _, user := range userL.Items {
		// Check if db is linked to pgdatabase
		for _, priv := range user.Spec.Privileges {
			if priv.Database.Name == instance.Name && (priv.Database.Namespace == instance.Namespace || user.Namespace == instance.Namespace) {
				return &user, nil
			}
		}
	}

	return nil, nil
}

func (r *PostgresqlDatabaseReconciler) updateInstance(
	ctx context.Context,
	instance *postgresqlv1alpha1.PostgresqlDatabase,
) (bool, error) {
	// Deep copy
	oCopy := instance.DeepCopy()

	// Add finalizer
	controllerutil.AddFinalizer(instance, config.Finalizer)

	// Check if schema list is set or not
	if len(instance.Spec.Schemas.List) == 0 {
		// Add "public" schema as it is the default for PG
		instance.Spec.Schemas.List = append(instance.Spec.Schemas.List, defaultPGPublicSchemaName)
	}

	// Check if update is needed
	if !reflect.DeepEqual(oCopy.ObjectMeta, instance.ObjectMeta) {
		return true, r.Update(ctx, instance)
	}

	return false, nil
}

func (*PostgresqlDatabaseReconciler) manageSchemas(pg postgres.PG, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
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

		// Get list of tables inside schema
		tableNames, err := pg.GetTablesInSchema(instance.Spec.Database, schema)
		if err != nil {
			return err
		}

		// Loop over all tables to force owner
		for _, tableName := range tableNames {
			// Force table owner
			err = pg.ChangeTableOwner(instance.Spec.Database, tableName, owner)
			if err != nil {
				return err
			}
		}

		// Get list of types inside schema
		types, err := pg.GetTypesInSchema(instance.Spec.Database, schema)
		if err != nil {
			return err
		}

		// Loop over all types to force owner
		for _, typeName := range types {
			// Force table owner
			err = pg.ChangeTypeOwnerInSchema(instance.Spec.Database, schema, typeName, owner)
			if err != nil {
				return err
			}
		}

		// Check if schema was created. Skip if already added
		if !funk.ContainsString(instance.Status.Schemas, schema) {
			instance.Status.Schemas = append(instance.Status.Schemas, schema)
		}
	}

	return nil
}

func (*PostgresqlDatabaseReconciler) manageExtensions(pg postgres.PG, instance *postgresqlv1alpha1.PostgresqlDatabase) error {
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
		// Execute create extension SQL statement
		err := pg.CreateExtension(instance.Spec.Database, extension)
		if err != nil {
			return err
		}
		// Check if extension was added. Skip if already added
		if !funk.ContainsString(instance.Status.Extensions, extension) {
			instance.Status.Extensions = append(instance.Status.Extensions, extension)
		}
	}

	return nil
}

func (*PostgresqlDatabaseReconciler) manageReaderRole(pg postgres.PG, reader string, instance *postgresqlv1alpha1.PostgresqlDatabase, allowGrantAdminOption bool) error {
	// Check if role was already created in the past
	if instance.Status.Roles.Reader != "" {
		// Check if role doesn't already exists
		exists, err := pg.IsRoleExist(instance.Status.Roles.Reader)
		// Check error
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

	// Check if role doesn't already exists
	exists, err := pg.IsRoleExist(reader)
	// Check error
	if err != nil {
		return err
	}
	// Check if exists
	if !exists {
		// Create it
		err = pg.CreateGroupRole(reader)
		// Check error
		if err != nil {
			return err
		}
	}

	// Grant role to current role
	err = pg.GrantRole(reader, pg.GetUser(), allowGrantAdminOption)
	// Check error
	if err != nil {
		return err
	}

	// Update status
	instance.Status.Roles.Reader = reader

	return nil
}

func (*PostgresqlDatabaseReconciler) manageWriterRole(pg postgres.PG, writer string, instance *postgresqlv1alpha1.PostgresqlDatabase, allowGrantAdminOption bool) error {
	// Check if role was already created in the past
	if instance.Status.Roles.Writer != "" {
		// Check if role doesn't already exists
		exists, err := pg.IsRoleExist(instance.Status.Roles.Writer)
		// Check error
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

	// Check if role doesn't already exists
	exists, err := pg.IsRoleExist(writer)
	// Check error
	if err != nil {
		return err
	}
	// Check if exists
	if !exists {
		// Create it
		err = pg.CreateGroupRole(writer)
		// Check error
		if err != nil {
			return err
		}
	}

	// Grant role to current role
	err = pg.GrantRole(writer, pg.GetUser(), allowGrantAdminOption)
	// Check error
	if err != nil {
		return err
	}

	// Update status
	instance.Status.Roles.Writer = writer

	return nil
}

func (*PostgresqlDatabaseReconciler) manageOwnerRole(pg postgres.PG, owner string, instance *postgresqlv1alpha1.PostgresqlDatabase, allowGrantAdminOption bool) error {
	// Check if role was already created in the past
	if instance.Status.Roles.Owner != "" {
		// Check if role doesn't already exists
		exists, err := pg.IsRoleExist(instance.Status.Roles.Owner)
		// Check error
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

	// Check if role doesn't already exists
	exists, err := pg.IsRoleExist(owner)
	// Check error
	if err != nil {
		return err
	}
	// Check if exists
	if !exists {
		// Create it
		err = pg.CreateGroupRole(owner)
		// Check error
		if err != nil {
			return err
		}
	}

	// Grant role to current role
	err = pg.GrantRole(owner, pg.GetUser(), allowGrantAdminOption)
	// Check error
	if err != nil {
		return err
	}

	// Update status
	instance.Status.Roles.Owner = owner

	return nil
}

func (r *PostgresqlDatabaseReconciler) manageError(
	ctx context.Context,
	logger logr.Logger,
	instance *postgresqlv1alpha1.PostgresqlDatabase,
	originalPatch client.Patch,
	issue error,
) (ctrl.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.Recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = postgresqlv1alpha1.DatabaseFailedPhase

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

func (r *PostgresqlDatabaseReconciler) manageSuccess(
	ctx context.Context,
	logger logr.Logger,
	instance *postgresqlv1alpha1.PostgresqlDatabase,
	originalPatch client.Patch,
) (ctrl.Result, error) {
	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = postgresqlv1alpha1.DatabaseCreatedPhase

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
func (r *PostgresqlDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&postgresqlv1alpha1.PostgresqlDatabase{}).
		Complete(r)
}
