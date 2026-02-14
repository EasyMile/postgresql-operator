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
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/internal/controller/config"
	"github.com/easymile/postgresql-operator/internal/controller/utils"
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

const (
	pgDumpEnvHost     = "PGHOST"
	pgDumpEnvPort     = "PGPORT"
	pgDumpEnvUser     = "PGUSER"
	pgDumpEnvPassword = "PGPASSWORD"
	pgDumpEnvDatabase = "PGDATABASE"
	pgDumpEnvSSLMode  = "PGSSLMODE"
	maxNameLength     = 63
)

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

	// Get related database
	database, err := utils.FindPgDatabaseFromLink(ctx, r.Client, instance.Spec.Database, instance.Namespace)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}
	// Check that postgres database is ready before continue but only if it is the first time
	// If not, requeue event
	if !database.Status.Ready {
		reqLogger.Info("PostgresqlDatabase not ready, waiting for it")
		r.Recorder.Event(instance, "Warning", "Processing", "Processing stopped because PostgresqlDatabase isn't ready. Waiting for it.")

		return ctrl.Result{}, nil
	}

	// Find pgec
	pgec, err := utils.FindPgEngineCfg(ctx, r.Client, database)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}
	if !pgec.Status.Ready {
		reqLogger.Info("PostgresqlEngineConfiguration not ready, waiting for it")
		r.Recorder.Event(instance, "Warning", "Processing", "Processing stopped because PostgresqlEngineConfiguration isn't ready. Waiting for it.")

		return ctrl.Result{}, nil
	}

	// Find pgec secret
	pgecSecret, err := utils.FindSecretPgEngineCfg(ctx, r.Client, pgec)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Get related provider
	backupProvider, err := utils.FindPgBackupProviderFromLink(ctx, r.Client, instance.Spec.BackupProvider, instance.Namespace)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Manage secret
	_, err = r.manageSecret(ctx, instance, database, pgec, pgecSecret, backupProvider)
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Manage cronjob part

	// Success
	return r.manageSuccess(ctx, reqLogger, instance, originalPatch)
}

func (r *PostgresqlBackupReconciler) generateResourceName(
	instance *postgresqlv1alpha1.PostgresqlBackup,
	provider *postgresqlv1alpha1.PostgresqlBackupProvider,
) string {
	// Start build name
	resourceName := provider.Spec.GeneratedNamePrefix + instance.Name + instance.Namespace
	// Check if it is greater than max authorized
	if len(resourceName) > maxNameLength {
		resourceName = resourceName[:maxNameLength]
	}

	return resourceName
}

func (r *PostgresqlBackupReconciler) manageSecret(
	ctx context.Context,
	instance *postgresqlv1alpha1.PostgresqlBackup,
	database *postgresqlv1alpha1.PostgresqlDatabase,
	pgec *postgresqlv1alpha1.PostgresqlEngineConfiguration,
	pgecSecret *corev1.Secret,
	backupProvider *postgresqlv1alpha1.PostgresqlBackupProvider,
) (string, error) {
	// Generate name
	secretName := r.generateResourceName(instance, backupProvider)
	if secretName == "" {
		return "", errors.NewBadRequest("backup provider generated secret name is empty")
	}

	user := string(pgecSecret.Data[pgecSecretUserKey])
	password := string(pgecSecret.Data[pgecSecretPassKey])
	if user == "" || password == "" {
		return "", errors.NewBadRequest("engine configuration secret must contain \"user\" and \"password\" values")
	}

	databaseName := database.Status.Database
	if databaseName == "" {
		return "", errors.NewBadRequest("database name is empty")
	}

	host := pgec.Spec.Host
	port := pgec.Spec.Port
	uriArgs := pgec.Spec.URIArgs

	data := map[string][]byte{
		pgDumpEnvHost:     []byte(host),
		pgDumpEnvPort:     []byte(strconv.Itoa(port)),
		pgDumpEnvUser:     []byte(user),
		pgDumpEnvPassword: []byte(password),
		pgDumpEnvDatabase: []byte(databaseName),
	}

	if uriArgs != "" {
		for part := range strings.SplitSeq(uriArgs, "&") {
			if part == "" {
				continue
			}
			keyValue := strings.SplitN(part, "=", 2) //nolint:mnd
			if len(keyValue) == 2 && keyValue[0] == "sslmode" && keyValue[1] != "" {
				data[pgDumpEnvSSLMode] = []byte(keyValue[1])

				break
			}
		}
	}

	// Create secret structure
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   instance.Namespace,
			Labels:      backupProvider.Spec.Labels,
			Annotations: backupProvider.Spec.Annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	// Add controller reference
	err := controllerutil.SetControllerReference(instance, secret, r.Scheme)
	if err != nil {
		return "", err
	}

	// Check if previous generated name was the same or not
	// If not, delete previous secret
	if instance.Status.GeneratedName != "" && instance.Status.GeneratedName != secretName {
		err = r.Client.Delete(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: instance.Namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{},
		})
		// Check error
		if err != nil {
			return "", err
		}
	}

	// Try to find it in kubernetes
	found := &corev1.Secret{}
	err = r.Get(
		ctx,
		types.NamespacedName{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		found,
	)
	// Check if error is present and it isn't a not found error
	if err != nil && !errors.IsNotFound(err) {
		return "", err
	}

	// Check if error is present and if it is a not found error
	if err != nil && errors.IsNotFound(err) {
		// Create secret
		return secretName, r.Create(ctx, secret)
	}

	// Update case

	// Check if update is needed
	if !reflect.DeepEqual(found.Data, secret.Data) ||
		!reflect.DeepEqual(found.Labels, secret.Labels) ||
		!reflect.DeepEqual(found.Annotations, secret.Annotations) {
		found.Data = secret.Data
		found.Labels = secret.Labels
		found.Annotations = secret.Annotations

		// Update
		return secretName, r.Update(ctx, found)
	}

	// Nothing to update or patch
	return secretName, nil
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
