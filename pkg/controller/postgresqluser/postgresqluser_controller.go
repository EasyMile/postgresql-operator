package postgresqluser

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	RequeueDelayErrorNumberSeconds   = 5
	RequeueDelaySuccessNumberSeconds = 10
	ControllerName                   = "postgresqluser-controller"
	RoleSuffixSize                   = 6
	PasswordSize                     = 15
)

var log = logf.Log.WithName("controller_postgresqluser")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PostgresqlUser Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePostgresqlUser{
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

	// Watch for changes to primary resource PostgresqlUser
	err = c.Watch(&source.Kind{Type: &postgresqlv1alpha1.PostgresqlUser{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner PostgresqlUser
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &postgresqlv1alpha1.PostgresqlUser{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePostgresqlUser implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePostgresqlUser{}

// ReconcilePostgresqlUser reconciles a PostgresqlUser object
type ReconcilePostgresqlUser struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a PostgresqlUser object and makes changes based on the state read
// and what is in the PostgresqlUser.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePostgresqlUser) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PostgresqlUser")

	// Fetch the PostgresqlUser instance
	instance := &postgresqlv1alpha1.PostgresqlUser{}
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
		// Deletion detected
		err = r.manageDeletion(reqLogger, instance)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
		// Remove finalizer
		controllerutil.RemoveFinalizer(instance, config.Finalizer)
		// Update CR
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
		// Stop reconcile
		return reconcile.Result{}, nil
	}

	// Creation case

	// Find PG Database
	pgDB, err := utils.FindPgDatabase(r.client, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Check that postgres database is ready before continue but only if it is the first time
	// If not, requeue event with a short delay (1 second)
	if instance.Status.Phase == postgresqlv1alpha1.UserNoPhase && !pgDB.Status.Ready {
		reqLogger.Info("PostgresqlDatabase not ready, waiting for it")
		r.recorder.Event(instance, "Warning", "Processing", "Processing stopped because PostgresqlDatabase isn't ready. Waiting for it.")
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: time.Second,
		}, nil
	}

	// Find PG Engine cfg
	pgEngineCfg, err := utils.FindPgEngineCfg(r.client, pgDB)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Find PG Engine secret
	pgEngineSecret, err := utils.FindSecretPgEngineCfg(r.client, pgEngineCfg)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Add finalizer and owners
	updated, err := r.updateInstance(instance)
	// Check error
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}
	// Check if it has been updated in order to stop this reconcile loop here for the moment
	if updated {
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: time.Second,
		}, nil
	}

	// Create pg instance
	pgInstance := utils.CreatePgInstance(reqLogger, pgEngineSecret.Data, pgEngineCfg)

	role := instance.Status.PostgresRole
	login := instance.Status.PostgresLogin
	password := utils.GetRandomString(PasswordSize)

	// Create user role if necessary
	if instance.Spec.RolePrefix != instance.Status.RolePrefix {
		// Previous role prefix doesn't match new one => need to create new role
		role, login, err = r.manageCreateUserRole(reqLogger, pgInstance, instance, password)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
		// Update status
		instance.Status.PostgresRole = role
		instance.Status.RolePrefix = instance.Spec.RolePrefix
		instance.Status.PostgresLogin = login
	}
	// Check if user was already created and if it is still present in engine
	exists, err := pgInstance.IsRoleExist(role)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}
	// Check result
	if !exists {
		// Need to create a new user role
		role, login, err = r.manageCreateUserRole(reqLogger, pgInstance, instance, password)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
		// Update status with new role and login
		instance.Status.PostgresRole = role
		instance.Status.RolePrefix = instance.Spec.RolePrefix
		instance.Status.PostgresLogin = login
	}

	// Grant group role to user role
	var groupRole string
	switch instance.Spec.Privileges {
	case postgresqlv1alpha1.ReaderPrivilege:
		groupRole = pgDB.Status.Roles.Reader
	case postgresqlv1alpha1.WriterPrivilege:
		groupRole = pgDB.Status.Roles.Writer
	default:
		groupRole = pgDB.Status.Roles.Owner
	}

	// Check if user was previously assign to another group
	if instance.Status.PostgresGroup != "" && instance.Status.PostgresGroup != groupRole {
		// Revoke old group from potentially old user role
		err = pgInstance.RevokeRole(instance.Status.PostgresGroup, instance.Status.PostgresRole)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
	}
	err = pgInstance.GrantRole(groupRole, role)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Alter default set role to group role
	// This is so that objects created by user gets owned by group role
	err = pgInstance.AlterDefaultLoginRole(role, groupRole)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Update status
	instance.Status.PostgresGroup = groupRole
	instance.Status.PostgresDatabaseName = pgDB.Spec.Database

	// Create new secret
	generatedSecret, err := r.newSecretForPGUser(instance, role, password, login, pgInstance)
	if err != nil {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Check if this Secret already exists
	secrFound := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: generatedSecret.Name, Namespace: generatedSecret.Namespace}, secrFound)
	// Check if error exists and not a not found error
	if err != nil && !errors.IsNotFound(err) {
		return r.manageError(reqLogger, instance, originalPatch, err)
	}

	// Check if error exists and if it a not found error
	if err != nil && errors.IsNotFound(err) {
		// Secret wasn't already present

		// Update password in pg
		err = pgInstance.UpdatePassword(role, password)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}
		reqLogger.Info("Creating secret", "Secret.Namespace", generatedSecret.Namespace, "Secret.Name", generatedSecret.Name)
		r.recorder.Event(instance, "Normal", "Processing", fmt.Sprintf("Creating secret %s for Postgresql User", generatedSecret.Name))
		err = r.client.Create(context.TODO(), generatedSecret)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}

		// Update status
		instance.Status.LastPasswordChangedTime = time.Now().Format(time.RFC3339)
	} else if !r.isSecretValid(secrFound, generatedSecret) { // Check if secret must be updated because invalid
		// Update password in pg
		reqLogger.Info("Updating password in Postgresql Engine")
		err = pgInstance.UpdatePassword(role, password)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}

		// Need to update secret
		reqLogger.Info("Updating secret because secret has changed", "Secret.Namespace", generatedSecret.Namespace, "Secret.Name", generatedSecret.Name)
		err = r.updatePGUserSecret(secrFound, generatedSecret)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}

		// Update status
		instance.Status.LastPasswordChangedTime = time.Now().Format(time.RFC3339)
	} else if instance.Spec.UserPasswordRotationDuration != "" { // Check if password rotation is enabled
		// Try to parse duration
		dur, err := time.ParseDuration(instance.Spec.UserPasswordRotationDuration)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}

		// Check if is time to change
		now := time.Now()
		lastChange, err := time.Parse(time.RFC3339, instance.Status.LastPasswordChangedTime)
		if err != nil {
			return r.manageError(reqLogger, instance, originalPatch, err)
		}

		if now.Sub(lastChange) >= dur {
			// Need to change password

			// Update password in pg
			reqLogger.Info("Updating password in Postgresql Engine")
			err = pgInstance.UpdatePassword(role, password)
			if err != nil {
				return r.manageError(reqLogger, instance, originalPatch, err)
			}
			// Need to update secret
			reqLogger.Info("Updating secret", "Secret.Namespace", generatedSecret.Namespace, "Secret.Name", generatedSecret.Name)
			err = r.updatePGUserSecret(secrFound, generatedSecret)
			if err != nil {
				return r.manageError(reqLogger, instance, originalPatch, err)
			}
			// Update status
			instance.Status.LastPasswordChangedTime = now.Format(time.RFC3339)
		}
	}

	return r.manageSuccess(reqLogger, instance, originalPatch)
}

func (r *ReconcilePostgresqlUser) manageDeletion(reqLogger logr.Logger, instance *postgresqlv1alpha1.PostgresqlUser) error {
	// Check if previous resource was created
	if instance.Status.Phase != postgresqlv1alpha1.UserCreatedPhase {
		// Stop because was in error
		return nil
	}
	// Find PG Database
	pgDB, err := utils.FindPgDatabase(r.client, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Can't do anything => log and stop
			reqLogger.Info("Can't delete user because linked PostgresqlDatabase can't be found")
			return nil
		}
		return err
	}

	// Find PG Engine cfg
	pgEngineCfg, err := utils.FindPgEngineCfg(r.client, pgDB)
	if err != nil {
		if errors.IsNotFound(err) {
			// Can't do anything => log and stop
			reqLogger.Info("Can't delete user because linked PostgresqlEngineConfiguration can't be found")
			return nil
		}
		return err
	}

	// Find PG Engine secret
	pgEngineSecret, err := utils.FindSecretPgEngineCfg(r.client, pgEngineCfg)
	if err != nil {
		return err
	}

	// Create pg instance
	pgInstance := utils.CreatePgInstance(reqLogger, pgEngineSecret.Data, pgEngineCfg)

	// Prepare database name
	databaseName := pgDB.Status.Database

	// Delete role
	err = pgInstance.DropRole(
		instance.Status.PostgresRole,
		instance.Status.PostgresGroup,
		databaseName,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcilePostgresqlUser) updateInstance(instance *postgresqlv1alpha1.PostgresqlUser) (bool, error) {
	// Deep copy
	copy := instance.DeepCopy()

	// Add finalizer
	controllerutil.AddFinalizer(instance, config.Finalizer)

	// Check if update is needed
	if !reflect.DeepEqual(copy.ObjectMeta, instance.ObjectMeta) {
		return true, r.client.Update(context.TODO(), instance)
	}

	return false, nil
}

func (r *ReconcilePostgresqlUser) manageCreateUserRole(reqLogger logr.Logger, pgInstance postgres.PG, instance *postgresqlv1alpha1.PostgresqlUser, password string) (string, string, error) {
	// Delete old role if exists
	if instance.Status.RolePrefix != "" {
		// Drop old role
		err := pgInstance.DropRole(
			instance.Status.PostgresRole,
			instance.Status.PostgresGroup,
			instance.Status.PostgresDatabaseName,
		)
		if err != nil {
			return "", "", err
		}
	}
	// Create new role
	suffix := utils.GetRandomString(RoleSuffixSize)
	role := fmt.Sprintf("%s-%s", instance.Spec.RolePrefix, suffix)

	// Check role length
	if len(role) > postgres.MaxIdentifierLength {
		errStr := fmt.Sprintf("identifier too long, must be <= 63, %s is %d character, must reduce role prefix length", role, len(role))
		return "", "", errors.NewBadRequest(errStr)
	}

	login, err := pgInstance.CreateUserRole(role, password)
	if err != nil {
		return "", "", err
	}
	return role, login, nil
}

func (r *ReconcilePostgresqlUser) updatePGUserSecret(foundSecret, newSecret *corev1.Secret) error {
	// Update old secret data with new data
	foundSecret.Data = newSecret.Data

	// Save it
	err := r.client.Update(context.TODO(), foundSecret)
	if err != nil {
		return err
	}

	// Add event
	r.recorder.Event(foundSecret, "Normal", "Updated", "Secret updated by "+ControllerName)

	return nil
}

func (r *ReconcilePostgresqlUser) isSecretValid(foundSecret, newSecret *corev1.Secret) bool {
	// Get data
	foundData := foundSecret.Data
	newData := newSecret.Data

	// Check if POSTGRES_URL exists
	// As we don't know the password, just check exist
	if string(foundData["POSTGRES_URL"]) == "" {
		return false
	}

	// Check if POSTGRES_URL_ARGS exists
	// As we don't know the password, just check exist
	if string(foundData["POSTGRES_URL_ARGS"]) == "" {
		return false
	}

	// Check if PASSWORD exists
	// As we don't know the password, just check exist
	if string(foundData["PASSWORD"]) == "" {
		return false
	}

	// Must be equal cases
	cases := []string{"ROLE", "LOGIN", "DATABASE", "HOST", "PORT", "ARGS"}
	// Check
	for _, k := range cases {
		if string(foundData[k]) != string(newData[k]) {
			return false
		}
	}

	// Ok
	return true
}

func (r *ReconcilePostgresqlUser) newSecretForPGUser(instance *postgresqlv1alpha1.PostgresqlUser, role, password, login string, pg postgres.PG) (*corev1.Secret, error) {
	pgUserURL := postgres.TemplatePostgresqlURL(pg.GetHost(), role, password, instance.Status.PostgresDatabaseName, pg.GetPort())
	pgUserURLWArgs := postgres.TemplatePostgresqlURLWithArgs(pg.GetHost(), role, password, pg.GetArgs(), instance.Status.PostgresDatabaseName, pg.GetPort())
	labels := map[string]string{
		"app": instance.Name,
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", instance.Spec.GeneratedSecretNamePrefix, instance.Name),
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Data: map[string][]byte{
			"POSTGRES_URL":      []byte(pgUserURL),
			"POSTGRES_URL_ARGS": []byte(pgUserURLWArgs),
			"ROLE":              []byte(role),
			"PASSWORD":          []byte(password),
			"LOGIN":             []byte(login),
			"DATABASE":          []byte(instance.Status.PostgresDatabaseName),
			"HOST":              []byte(pg.GetHost()),
			"PORT":              []byte(fmt.Sprintf("%d", pg.GetPort())),
			"ARGS":              []byte(pg.GetArgs()),
		},
	}

	// Set owner references
	err := controllerutil.SetControllerReference(instance, secret, r.scheme)
	if err != nil {
		return nil, err
	}
	return secret, err
}

func (r *ReconcilePostgresqlUser) manageError(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlUser, originalPatch client.Patch, issue error) (reconcile.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = postgresqlv1alpha1.UserFailedPhase

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

func (r *ReconcilePostgresqlUser) manageSuccess(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlUser, originalPatch client.Patch) (reconcile.Result, error) {
	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = postgresqlv1alpha1.UserCreatedPhase

	// Patch status
	err := r.client.Status().Patch(context.TODO(), instance, originalPatch)
	if err != nil {
		logger.Error(err, "unable to update status")
		return reconcile.Result{
			RequeueAfter: RequeueDelayErrorNumberSeconds * time.Second,
			Requeue:      true,
		}, nil
	}

	logger.Info("Reconcile done")
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: RequeueDelaySuccessNumberSeconds * time.Second,
	}, nil
}
