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
	RequeueDelayErrorSeconds   = 5 * time.Second
	RequeueDelaySuccessSeconds = 30 * time.Second
	ControllerName             = "postgresqluser-controller"
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

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner PostgresqlUser
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
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

	// Deletion case
	if !instance.GetDeletionTimestamp().IsZero() {
		// Deletion detected
		err = r.manageDeletion(reqLogger, instance)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
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

	// Find PG Database
	pgDb, err := utils.FindPgDatabase(r.client, instance)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}
	// TODO Check that pg db is ready before continue

	// Find PG Engine cfg
	pgEngineCfg, err := utils.FindPgEngineCfg(r.client, pgDb)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Find PG Engine secret
	pgEngineSecret, err := utils.FindSecretPgEngineCfg(r.client, pgEngineCfg)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Add finalizer and owners
	err = r.updateInstance(instance, pgDb)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Create pg instance
	pgInstance := utils.CreatePgInstance(reqLogger, pgEngineSecret.Data, &pgEngineCfg.Spec)

	role := instance.Status.PostgresRole
	login := instance.Status.PostgresLogin
	password := utils.GetRandomString(15)

	// Create user role if necessary
	if instance.Spec.RolePrefix != instance.Status.RolePrefix {
		role, login, err = r.manageCreateUserRole(reqLogger, pgInstance, instance, password)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
		// Update status
		instance.Status.PostgresRole = role
		instance.Status.RolePrefix = instance.Spec.RolePrefix
		instance.Status.PostgresLogin = login
	}
	// Check if user was already created and if it is still present in engine
	exists, err := pgInstance.IsRoleExist(role)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}
	// Check result
	if !exists {
		// Need to create a new user role
		role, login, err = r.manageCreateUserRole(reqLogger, pgInstance, instance, password)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
		// Update status with new role and login
		instance.Status.PostgresRole = role
		instance.Status.RolePrefix = instance.Spec.RolePrefix
		instance.Status.PostgresLogin = login
	}

	// Grant group role to user role
	var groupRole string
	switch instance.Spec.Privileges {
	case "READ":
		groupRole = pgDb.Status.Roles.Reader
	case "WRITE":
		groupRole = pgDb.Status.Roles.Writer
	default:
		groupRole = pgDb.Status.Roles.Owner
	}

	// Check if user was previously assign to another group
	if instance.Status.PostgresGroup != "" && instance.Status.PostgresGroup != groupRole {
		// Revoke old group from potentially old user role
		err = pgInstance.RevokeRole(instance.Status.PostgresGroup, instance.Status.PostgresRole)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
	}
	err = pgInstance.GrantRole(groupRole, role)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Alter default set role to group role
	// This is so that objects created by user gets owned by group role
	err = pgInstance.AlterDefaultLoginRole(role, groupRole)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Update status
	instance.Status.PostgresGroup = groupRole
	instance.Status.PostgresDatabaseName = pgDb.Spec.Database

	// Create new secret
	secret, err := r.newSecretForPGUser(instance, role, password, login, pgInstance, pgDb)
	if err != nil {
		return r.manageError(reqLogger, instance, err)
	}

	// Check if this Secret already exists
	secrFound := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, secrFound)
	// Check if error exists and not a not found error
	if err != nil && !errors.IsNotFound(err) {
		return r.manageError(reqLogger, instance, err)
	}

	// Get role in secret
	secretRole := string(secrFound.Data["ROLE"])
	// Check if error exists and if it a not found error
	if err != nil && errors.IsNotFound(err) {
		// Secret wasn't already present

		// Update password in pg
		err = pgInstance.UpdatePassword(role, password)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
		reqLogger.Info("Creating secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		r.recorder.Event(instance, "Normal", "Processing", fmt.Sprintf("Creating secret %s for Postgresql User", secret.Name))
		err = r.client.Create(context.TODO(), secret)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
	} else if secretRole != instance.Status.PostgresRole { // Check if secret must be updated
		// Need to update secret
		err = r.updatePGUserSecret(secrFound, secret)
		if err != nil {
			return r.manageError(reqLogger, instance, err)
		}
	}

	return r.manageSuccess(reqLogger, instance)
}

func (r *ReconcilePostgresqlUser) manageDeletion(reqLogger logr.Logger, instance *postgresqlv1alpha1.PostgresqlUser) error {
	// Find PG Database
	pgDb, err := utils.FindPgDatabase(r.client, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Can't do anything => log and stop
			reqLogger.Info("Can't delete user because linked PostgresqlDatabase can't be found")
			return nil
		}
		return err
	}

	// Find PG Engine cfg
	pgEngineCfg, err := utils.FindPgEngineCfg(r.client, pgDb)
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
	pgInstance := utils.CreatePgInstance(reqLogger, pgEngineSecret.Data, &pgEngineCfg.Spec)

	// Prepare database name
	databaseName := pgDb.Status.Database

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

func (r *ReconcilePostgresqlUser) updateInstance(instance *postgresqlv1alpha1.PostgresqlUser, pgDb *postgresqlv1alpha1.PostgresqlDatabase) error {
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
	suffix := utils.GetRandomString(6)
	role := fmt.Sprintf("%s-%s", instance.Spec.RolePrefix, suffix)
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
	return err
}

func (r *ReconcilePostgresqlUser) newSecretForPGUser(instance *postgresqlv1alpha1.PostgresqlUser, role, password, login string, pg postgres.PG, pgDb *postgresqlv1alpha1.PostgresqlDatabase) (*corev1.Secret, error) {
	pgUserUrl := fmt.Sprintf("postgresql://%s:%s@%s/%s", role, password, pg.GetHost(), instance.Status.PostgresDatabaseName)
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
			"POSTGRES_URL": []byte(pgUserUrl),
			"ROLE":         []byte(role),
			"PASSWORD":     []byte(password),
			"LOGIN":        []byte(login),
		},
	}

	// Set owner references
	err := controllerutil.SetControllerReference(instance, secret, r.scheme)
	if err != nil {
		return nil, err
	}
	return secret, err
}

func (r *ReconcilePostgresqlUser) manageError(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlUser, issue error) (reconcile.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = postgresqlv1alpha1.UserFailedPhase

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

func (r *ReconcilePostgresqlUser) manageSuccess(logger logr.Logger, instance *postgresqlv1alpha1.PostgresqlUser) (reconcile.Result, error) {
	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = postgresqlv1alpha1.UserCreatedPhase

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
