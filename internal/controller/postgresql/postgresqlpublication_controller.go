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
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/internal/controller/config"
	"github.com/easymile/postgresql-operator/internal/controller/postgresql/postgres"
	"github.com/easymile/postgresql-operator/internal/controller/utils"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
)

const DefaultReplicationSlotPlugin = "pgoutput"

// PostgresqlPublicationReconciler reconciles a PostgresqlPublication object.
type PostgresqlPublicationReconciler struct {
	Recorder record.EventRecorder
	client.Client
	Scheme                              *runtime.Scheme
	ControllerRuntimeDetailedErrorTotal *prometheus.CounterVec
	Log                                 logr.Logger
	ControllerName                      string
	ReconcileTimeout                    time.Duration
}

//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlpublications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlpublications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqlpublications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Reconcile function to compare the state specified by
// the PostgresqlPublication object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *PostgresqlPublicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:wsl // it is like that
	// Issue with this logger: controller and controllerKind are incorrect
	// Build another logger from upper to fix this.
	// reqLogger := log.FromContext(ctx)

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	reqLogger.Info("Reconciling PostgresqlPublication")

	// Fetch the PostgresqlPublication instance
	instance := &v1alpha1.PostgresqlPublication{}
	err := r.Get(ctx, req.NamespacedName, instance)

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

func (r *PostgresqlPublicationReconciler) mainReconcile(
	ctx context.Context,
	reqLogger logr.Logger,
	instance *v1alpha1.PostgresqlPublication,
	originalPatch client.Patch,
) (ctrl.Result, error) {
	// Deletion case
	if !instance.GetDeletionTimestamp().IsZero() { //nolint:wsl
		// Deletion detected

		// Check if drop on delete is enabled
		if instance.Spec.DropOnDelete {
			// Delete publication
			err := r.manageDropPublication(ctx, reqLogger, instance)
			if err != nil {
				return r.manageError(ctx, reqLogger, instance, originalPatch, err)
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(instance, config.Finalizer)

		// Update CR
		err := r.Update(ctx, instance)
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}

		reqLogger.Info("Successfully deleted")
		// Stop reconcile
		return reconcile.Result{}, nil
	}

	// Creation / Update case

	// Validate
	err := r.validate(instance)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Try to find pg db CR
	pgDB, err := utils.FindPgDatabaseFromLink(ctx, r.Client, instance.Spec.Database, instance.Namespace)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Check that postgres database is ready before continue but only if it is the first time
	// If not, requeue event
	if instance.Status.Phase == v1alpha1.PublicationNoPhase && !pgDB.Status.Ready {
		reqLogger.Info("PostgresqlDatabase not ready, waiting for it")
		r.Recorder.Event(instance, "Warning", "Processing", "Processing stopped because PostgresqlDatabase isn't ready. Waiting for it.")

		return ctrl.Result{}, nil
	}

	// Try to find PostgresqlEngineConfiguration CR
	pgEngCfg, err := utils.FindPgEngineCfg(ctx, r.Client, pgDB)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Check that postgres engine configuration is ready before continue but only if it is the first time
	// If not, requeue event
	if instance.Status.Phase == v1alpha1.PublicationNoPhase && !pgEngCfg.Status.Ready {
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

	// Calculate hash for status (this time is to update it in status)
	hash, err := utils.CalculateHash(instance.Spec)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewInternalError(err))
	}

	// Create PG instance
	pg := utils.CreatePgInstance(reqLogger, secret.Data, pgEngCfg)

	// Compute name to search
	nameToSearch := instance.Status.Name
	// Check
	if nameToSearch == "" {
		// ? This is done to recover the first creation with an existing publication with the same name
		nameToSearch = instance.Spec.Name
	}

	// Get publication
	pubRes, err := pg.GetPublication(ctx, pgDB.Status.Database, nameToSearch)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Check if publication haven't been found
	if pubRes == nil {
		// Create case
		reqLogger.Info("Publication creation case detected")

		err = r.manageCreate(ctx, instance, pg, pgDB)
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
	} else {
		// Update case
		// Need to check if status hash is the same or not to force renew or not
		if hash != instance.Status.Hash {
			reqLogger.Info("Specs are different, update need to be done")

			err = r.manageUpdate(ctx, instance, pg, pgDB, pubRes, nameToSearch)
			// Check error
			if err != nil {
				return r.manageError(ctx, reqLogger, instance, originalPatch, err)
			}
		}
	}

	// Get replication slot
	replicationSlotResult, err := pg.GetReplicationSlot(ctx, instance.Spec.ReplicationSlotName)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Check if replication slot hasn't been found in database
	if replicationSlotResult == nil {
		// Create it
		err = pg.CreateReplicationSlot(ctx, pgDB.Status.Database, instance.Spec.ReplicationSlotName, instance.Spec.ReplicationSlotPlugin)
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
	} else { //nolint:wsl
		// Update isn't possible in PG
		// Here we decide to check and fail if already exists and it isn't for the same database or with the same plugin
		//

		// Other database case
		if replicationSlotResult.Database != pgDB.Status.Database {
			return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewBadRequest("replication slot with the same name already exists for another database"))
		}

		// Other plugin case
		if replicationSlotResult.Plugin != instance.Spec.ReplicationSlotPlugin {
			return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewBadRequest("replication slot with the same name already exists with another plugin"))
		}
	}

	// Save name
	instance.Status.Name = instance.Spec.Name
	// Save hash in status
	instance.Status.Hash = hash
	// Save for all tables
	instance.Status.AllTables = &instance.Spec.AllTables
	// Save replication data
	instance.Status.ReplicationSlotName = instance.Spec.ReplicationSlotName
	instance.Status.ReplicationSlotPlugin = instance.Spec.ReplicationSlotPlugin

	return r.manageSuccess(ctx, reqLogger, instance, originalPatch)
}

func (*PostgresqlPublicationReconciler) manageUpdate(
	ctx context.Context,
	instance *v1alpha1.PostgresqlPublication,
	pg postgres.PG,
	pgDB *v1alpha1.PostgresqlDatabase,
	pubRes *postgres.PublicationResult,
	currentPublicationName string,
) error {
	// Check that publication in database and spec are aligned on "for all tables" as this cannot be changed
	if pubRes.AllTables != instance.Spec.AllTables {
		// Not aligned => Problem
		return errors.NewBadRequest("publication in database and spec are out of sync for 'for all tables' and values must be aligned to continue")
	}

	// Create builder
	builder := postgres.NewUpdatePublicationBuilder()

	// Check if publication has to be renamed
	if instance.Spec.Name != currentPublicationName {
		builder = builder.RenameTo(instance.Spec.Name)
	}

	// Add  tables schema
	builder = builder.SetTablesInSchema(instance.Spec.TablesInSchema)

	// Loop over tables
	for _, t := range instance.Spec.Tables {
		builder = builder.AddSetTable(t.TableName, t.Columns, t.AdditionalWhere)
	}

	// Check if there are with options
	if instance.Spec.WithParameters != nil {
		// Change with
		builder = builder.SetWith(instance.Spec.WithParameters.Publish, instance.Spec.WithParameters.PublishViaPartitionRoot)
	}

	// Perform update
	// ? Note: this will do an alter even if it is unnecessary
	// ? Detecting real diff will be long and painful, perform an alter with what is asked will ensure that nothing can be changed
	err := pg.UpdatePublication(ctx, pgDB.Status.Database, currentPublicationName, builder)
	// Check error
	if err != nil {
		return err
	}

	// Default
	return nil
}

func (*PostgresqlPublicationReconciler) manageCreate(
	ctx context.Context,
	instance *v1alpha1.PostgresqlPublication,
	pg postgres.PG,
	pgDB *v1alpha1.PostgresqlDatabase,
) error {
	// Save spec for easy use
	spec := instance.Spec

	// Create builder
	builder := postgres.NewCreatePublicationBuilder()

	// Add name & tables in schema
	builder = builder.SetName(spec.Name).SetTablesInSchema(spec.TablesInSchema)

	// Check if all tables is enabled
	if spec.AllTables {
		builder = builder.SetForAllTables()
	}

	// Check if with is set
	if spec.WithParameters != nil {
		// Manage with
		builder = builder.SetWith(spec.WithParameters.Publish, spec.WithParameters.PublishViaPartitionRoot)
	}

	// Manage tables
	lo.ForEach(spec.Tables, func(table *v1alpha1.PostgresqlPublicationTable, _ int) {
		builder = builder.AddTable(table.TableName, table.Columns, table.AdditionalWhere)
	})

	// Create publication
	err := pg.CreatePublication(ctx, pgDB.Status.Database, builder)
	// Check error
	if err != nil {
		return err
	}

	// Default
	return nil
}

func (*PostgresqlPublicationReconciler) validate(
	instance *v1alpha1.PostgresqlPublication,
) error {
	// Save spec for easy use
	spec := instance.Spec
	// Save status for easy use
	status := instance.Status

	// Check name
	if spec.Name == "" {
		return errors.NewBadRequest("name must have a value")
	}

	// Init some vars
	tablesInSchemaLength := len(spec.TablesInSchema)
	tablesLength := len(spec.Tables)

	// check that something have been asked
	if !spec.AllTables && tablesInSchemaLength == 0 && tablesLength == 0 {
		return errors.NewBadRequest("nothing is selected for publication (no all tables, no tables in schema, no tables)")
	}

	// Check all tables vs other case
	if spec.AllTables && (tablesInSchemaLength != 0 || tablesLength != 0) {
		return errors.NewBadRequest("all tables cannot be set with tables in schema or tables")
	}

	// Check status and spec "for all tables"
	if status.AllTables != nil && *status.AllTables != spec.AllTables {
		return errors.NewBadRequest("cannot change all tables flag on an upgrade")
	}

	// Check Tables in schema
	_, found := lo.Find(spec.TablesInSchema, func(it string) bool { return it == "" })
	// Check
	if found {
		return errors.NewBadRequest("tables in schema cannot have empty schema listed")
	}

	// Check tables
	_, found = lo.Find(spec.Tables, func(it *v1alpha1.PostgresqlPublicationTable) bool {
		// Check table name
		if it.TableName == "" {
			return true
		}

		// Check columns
		if it.Columns != nil {
			// Check if there is an empty column
			_, f := lo.Find(*it.Columns, func(it string) bool { return it == "" })
			// Check
			if f {
				return true
			}

			// Check if it have a columns list and a schema list
			if len(*it.Columns) != 0 && tablesInSchemaLength != 0 {
				return true
			}
		}

		// Check additional where
		if it.AdditionalWhere != nil && *it.AdditionalWhere == "" {
			return true
		}

		return false
	})
	// Check
	if found {
		return errors.NewBadRequest("tables cannot have a columns list with an empty name or have a columns list with a table schema list enabled or an empty additional where")
	}

	// Default
	return nil
}

func (r *PostgresqlPublicationReconciler) updateInstance(
	ctx context.Context,
	instance *v1alpha1.PostgresqlPublication,
) (bool, error) {
	// Deep copy
	oCopy := instance.DeepCopy()

	// Add finalizer
	controllerutil.AddFinalizer(instance, config.Finalizer)

	// Check if replication slot name isn't set
	if instance.Spec.ReplicationSlotName == "" {
		// Set to publication name
		instance.Spec.ReplicationSlotName = instance.Spec.Name
	}

	// Check if replication slot plugin isn't set
	if instance.Spec.ReplicationSlotPlugin == "" {
		// Set to default
		instance.Spec.ReplicationSlotPlugin = DefaultReplicationSlotPlugin
	}

	// Check if update is needed
	if !reflect.DeepEqual(oCopy.ObjectMeta, instance.ObjectMeta) {
		return true, r.Update(ctx, instance)
	}

	return false, nil
}

func (r *PostgresqlPublicationReconciler) manageDropPublication(
	ctx context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlPublication,
) error {
	// Get pg db
	pgDB, err := utils.FindPgDatabaseFromLink(ctx, r.Client, instance.Spec.Database, instance.Namespace)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	// In case of not found => Can't delete => skip
	if errors.IsNotFound(err) {
		logger.Error(err, "can't delete publication because PostgresDatabase didn't exists anymore")

		return nil
	}

	// Try to find PostgresqlEngineConfiguration CR
	pgEngCfg, err := utils.FindPgEngineCfg(ctx, r.Client, pgDB)
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

	// Get publication
	pub, err := pg.GetPublication(ctx, pgDB.Status.Database, instance.Spec.Name)
	if err != nil {
		return err
	}

	// Check if publication is still present to delete it
	if pub != nil {
		// Drop publication
		err = pg.DropPublication(ctx, pgDB.Status.Database, instance.Spec.Name)
		// Check error
		if err != nil {
			return err
		}
	}

	// Check if replication slot is defined
	if instance.Spec.ReplicationSlotName != "" {
		// Get replication slot
		rep, err := pg.GetReplicationSlot(ctx, instance.Spec.ReplicationSlotName)
		if err != nil {
			return err
		}

		// Check if replication slot is still present to delete it
		if rep != nil {
			// Drop replication slot
			err = pg.DropReplicationSlot(ctx, instance.Spec.ReplicationSlotName)
			// Check error
			if err != nil {
				return err
			}
		}
	}

	// Default
	return nil
}

func (r *PostgresqlPublicationReconciler) manageError(
	ctx context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlPublication,
	originalPatch client.Patch,
	issue error,
) (reconcile.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.Recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = v1alpha1.PublicationFailedPhase

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

func (r *PostgresqlPublicationReconciler) manageSuccess(
	ctx context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlPublication,
	originalPatch client.Patch,
) (reconcile.Result, error) {
	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = v1alpha1.PublicationCreatedPhase

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

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlPublicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PostgresqlPublication{}).
		Complete(r)
}
