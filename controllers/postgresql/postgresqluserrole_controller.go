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
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/easymile/postgresql-operator/apis/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/controllers/config"
	"github.com/easymile/postgresql-operator/controllers/postgresql/postgres"
	"github.com/easymile/postgresql-operator/controllers/utils"
	"github.com/go-logr/logr"
	"github.com/thoas/go-funk"
)

const (
	PGURRequeueDelayErrorNumberSeconds         = 5
	ListLimit                                  = 10
	Login0Suffix                               = "-0"
	Login1Suffix                               = "-1"
	DefaultWorkGeneratedSecretNamePrefix       = "pgcreds-work-" //nolint: gosec // Ignore this false positive
	DefaultWorkGeneratedSecretNameRandomLength = 20
	UsernameSecretKey                          = "USERNAME"
	PasswordSecretKey                          = "PASSWORD"
	ManagedPasswordSize                        = 15
)

// PostgresqlUserRoleReconciler reconciles a PostgresqlUserRole object.
type PostgresqlUserRoleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Log      logr.Logger
}

type dbPrivilegeCache struct {
	DBInstance    *v1alpha1.PostgresqlDatabase
	UserPrivilege *v1alpha1.PostgresqlUserRolePrivilege
}

//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqluserroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqluserroles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=postgresql.easymile.com,resources=postgresqluserroles/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the PostgresqlUserRole object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *PostgresqlUserRoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { //nolint:wsl // it is like that
	// Issue with this logger: controller and controllerKind are incorrect
	// Build another logger from upper to fix this.
	// reqLogger := log.FromContext(ctx)

	reqLogger := r.Log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	reqLogger.Info("Reconciling PostgresqlUserRole")

	// Fetch the PostgresqlUser instance
	instance := &v1alpha1.PostgresqlUserRole{}
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

	// Deletion case
	if !instance.GetDeletionTimestamp().IsZero() { //nolint:wsl // it is like that
		// Deletion detected

		// Check status postgresrole and so if user have been created
		if instance.Status.PostgresRole != "" {
			// Consider the current user as another old one
			instance.Status.OldPostgresRoles = append(instance.Status.OldPostgresRoles, instance.Status.PostgresRole)
			// Unique them
			instance.Status.OldPostgresRoles = funk.UniqString(instance.Status.OldPostgresRoles)
		}

		// Get needed items

		// Find PG Database cache
		dbCache, pgecDBPrivilegeCache, err := r.getDatabaseInstances(ctx, instance, true) //nolint:govet // Allow err shadow
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
		// Create PG instances
		pgInstancesCache, err := r.getPGInstances(ctx, reqLogger, dbCache, true)
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}

		// Delete roles
		err = r.manageActiveSessionsAndDropOldRoles(ctx, reqLogger, instance, pgInstancesCache, pgecDBPrivilegeCache)
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
		// Check if there is still users
		if len(instance.Status.OldPostgresRoles) != 0 {
			return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewBadRequest("old postgres roles still present"))
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(instance, config.Finalizer)

		// Update CR
		err = r.Update(ctx, instance)
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}

		reqLogger.Info("Successfully deleted")
		// Stop reconcile
		return reconcile.Result{}, nil
	}

	// Creation case

	// Validate
	err = r.validateInstance(ctx, instance)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Find PG Database cache
	dbCache, pgecDBPrivilegeCache, err := r.getDatabaseInstances(ctx, instance, false)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Loop over db cache
	for _, pgDB := range dbCache {
		// Check that postgres database is ready before continue but only if it is the first time
		// If not, requeue event with a short delay (1 second)
		if instance.Status.Phase == v1alpha1.UserRoleNoPhase && !pgDB.Status.Ready {
			reqLogger.Info("PostgresqlDatabase not ready, waiting for it")
			r.Recorder.Event(instance, "Warning", "Processing", "Processing stopped because PostgresqlDatabase isn't ready. Waiting for it.")

			return reconcile.Result{
				Requeue:      true,
				RequeueAfter: time.Second,
			}, nil
		}
	}

	// Add finalizer
	updated, err := r.updateInstance(ctx, instance)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}
	// Check if it has been updated in order to stop this reconcile loop here for the moment
	if updated {
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: time.Second,
		}, nil
	}

	var usernameChanged, passwordChanged, rotateUserPasswordError bool

	var workSec *corev1.Secret

	var oldUsername string

	// Check if it is a provided user
	if instance.Spec.Mode == v1alpha1.ProvidedMode {
		workSec, oldUsername, passwordChanged, err = r.createOrUpdateWorkSecretForProvidedMode(ctx, reqLogger, instance)
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
	} else {
		workSec, oldUsername, passwordChanged, rotateUserPasswordError, err = r.createOrUpdateWorkSecretForManagedMode(
			ctx,
			reqLogger,
			instance,
		)
		// Check error
		if err != nil {
			return r.manageError(ctx, reqLogger, instance, originalPatch, err)
		}
	}

	// Save info
	username := string(workSec.Data[UsernameSecretKey])
	password := string(workSec.Data[PasswordSecretKey])

	// Ensure they aren't empty
	if username == "" || password == "" {
		return r.manageError(ctx, reqLogger, instance, originalPatch, errors.NewBadRequest("username or password in work secret are empty so something is interfering with operator"))
	}

	// Compute username changed
	usernameChanged = username != oldUsername && oldUsername != ""

	// Check if username have changed
	if usernameChanged {
		// Update status to add username for deletion
		instance.Status.OldPostgresRoles = append(instance.Status.OldPostgresRoles, oldUsername)
	}

	// Create PG instances
	pgInstancesCache, err := r.getPGInstances(ctx, reqLogger, dbCache, false)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	//
	// Now need to manage user creation
	//

	// Manage deletion with active sessions
	err = r.manageActiveSessionsAndDropOldRoles(
		ctx,
		reqLogger,
		instance,
		pgInstancesCache,
		pgecDBPrivilegeCache,
	)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}
	// Check if we are in the user password rotation error case and old roles haven't been cleaned
	if rotateUserPasswordError && len(instance.Status.OldPostgresRoles) != 0 {
		// Stop here and throw an error
		err := errors.NewBadRequest("Old user password rotation wasn't a success and another one must be done.")

		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Create or update user role if necessary
	err = r.managePGUserRoles(ctx, reqLogger, instance, pgInstancesCache, username, password, passwordChanged)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Save important status now
	// Note: This is important to have a chance to have old username for deletion
	instance.Status.PostgresRole = username
	instance.Status.RolePrefix = instance.Spec.RolePrefix

	if passwordChanged || usernameChanged || instance.Status.LastPasswordChangedTime == "" {
		instance.Status.LastPasswordChangedTime = time.Now().Format(time.RFC3339)
	}

	//
	// Now manage rights
	//

	// Manage rights
	err = r.managePGUserRights(ctx, reqLogger, instance, pgInstancesCache, pgecDBPrivilegeCache, username)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	//
	// Now manage secrets
	//

	// Manage secrets
	err = r.manageSecrets(ctx, reqLogger, instance, pgInstancesCache, pgecDBPrivilegeCache, username, password)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	// Clean old secrets
	err = r.cleanOldSecrets(ctx, reqLogger, instance, pgecDBPrivilegeCache)
	// Check error
	if err != nil {
		return r.manageError(ctx, reqLogger, instance, originalPatch, err)
	}

	return r.manageSuccess(ctx, reqLogger, instance, originalPatch)
}

func (r *PostgresqlUserRoleReconciler) manageSecrets(
	ctx context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
	pgInstanceCache map[string]postgres.PG,
	pgecDBPrivilegeCache map[string][]*dbPrivilegeCache,
	username, password string,
) error {
	// Loop
	for key, pgecDBPrivilegeList := range pgecDBPrivilegeCache {
		// Loop over dbs
		for _, privilegeCache := range pgecDBPrivilegeList {
			// Check if this Secret already exists
			secrFound := &corev1.Secret{}
			err := r.Get(
				ctx,
				types.NamespacedName{
					Name:      privilegeCache.UserPrivilege.GeneratedSecretName,
					Namespace: instance.Namespace,
				},
				secrFound,
			)
			// Check if error exists and not a not found error
			if err != nil && !errors.IsNotFound(err) {
				return err
			}

			// Generate "new" secret
			generatedSecret, err2 := r.newSecretForPGUser(
				instance,
				privilegeCache.UserPrivilege,
				privilegeCache.DBInstance,
				username, password,
				pgInstanceCache[key],
			)
			// Check error
			if err2 != nil {
				return err2
			}

			// Check if not found
			if err != nil && errors.IsNotFound(err) {
				// Save secret
				err = r.Create(ctx, generatedSecret)
				// Check error
				if err != nil {
					return err
				}

				logger.Info(
					"Successfully created secret for engine and database",
					"postgresqlEngine", key,
					"postgresqlDatabase", utils.CreateNameKey(privilegeCache.DBInstance.Name, privilegeCache.DBInstance.Namespace, instance.Namespace),
					"secret", generatedSecret.Name,
				)
				r.Recorder.Eventf(instance, "Normal", "Updated", "Generated secret %s saved", generatedSecret.Name)
			} else if !reflect.DeepEqual(secrFound.Data, generatedSecret.Data) { // Check if secret is valid, if not, update it
				// Update secret
				secrFound.Data = generatedSecret.Data

				// Save secret
				err = r.Update(ctx, secrFound)
				// Check error
				if err != nil {
					return err
				}

				logger.Info(
					"Successfully updated secret for engine and database",
					"postgresqlEngine", key,
					"postgresqlDatabase", utils.CreateNameKey(privilegeCache.DBInstance.Name, privilegeCache.DBInstance.Namespace, instance.Namespace),
					"secret", secrFound.Name,
				)
				r.Recorder.Eventf(instance, "Normal", "Updated", "Generated secret %s saved", secrFound.Name)
				r.Recorder.Event(secrFound, "Normal", "Updated", "Secret updated")
			}
		}
	}

	// Default
	return nil
}

func (r *PostgresqlUserRoleReconciler) cleanOldSecrets(
	ctx context.Context,
	_ logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
	pgecDBPrivilegeCache map[string][]*dbPrivilegeCache,
) error {
	// Save all secrets
	secretNames := []string{instance.Spec.WorkGeneratedSecretName}

	// Loop
	for _, pgecDBPrivilege := range pgecDBPrivilegeCache {
		// Loop over dbs
		for _, pgecDBInstance := range pgecDBPrivilege {
			// Save
			secretNames = append(secretNames, pgecDBInstance.UserPrivilege.GeneratedSecretName)
		}
	}

	// Check if there is old secrets that must be deleted
	// Create temporary values
	nextMarker := ""
	continueLoop := true

	for continueLoop {
		// Prepare list
		list := &corev1.SecretList{}

		// List request
		err := r.List(ctx, list, &client.ListOptions{Continue: nextMarker, Limit: ListLimit})
		// Check error
		if err != nil {
			return err
		}

		// Save data
		nextMarker = list.Continue
		continueLoop = nextMarker != ""

		// Loop over all secrets
		for _, it := range list.Items {
			item := it
			// Check if secret is in the spec secret list
			if funk.ContainsString(secretNames, item.Name) {
				// Normal secret => Ignoring it
				continue
			}

			// Check if secret is owned by the current instance
			foundMarker := funk.Find(item.ObjectMeta.OwnerReferences, func(it metav1.OwnerReference) bool {
				return it.UID == instance.UID
			})

			// Check if owner reference have been found
			if foundMarker != nil {
				// Delete secret
				err = r.Delete(ctx, &item)
				// Check error
				if err != nil {
					return err
				}
			}
		}
	}

	// Default
	return nil
}

func (r *PostgresqlUserRoleReconciler) newSecretForPGUser(
	instance *v1alpha1.PostgresqlUserRole,
	rolePrivilege *v1alpha1.PostgresqlUserRolePrivilege,
	dbInstance *v1alpha1.PostgresqlDatabase,
	username, password string,
	pg postgres.PG,
) (*corev1.Secret, error) {
	pgUserURL := postgres.TemplatePostgresqlURL(pg.GetHost(), username, password, dbInstance.Status.Database, pg.GetPort())
	pgUserURLWArgs := postgres.TemplatePostgresqlURLWithArgs(pg.GetHost(), username, password, pg.GetArgs(), dbInstance.Status.Database, pg.GetPort())
	labels := map[string]string{
		"app": instance.Name,
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rolePrivilege.GeneratedSecretName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Data: map[string][]byte{
			"POSTGRES_URL":      []byte(pgUserURL),
			"POSTGRES_URL_ARGS": []byte(pgUserURLWArgs),
			"PASSWORD":          []byte(password),
			"LOGIN":             []byte(username),
			"DATABASE":          []byte(dbInstance.Status.Database),
			"HOST":              []byte(pg.GetHost()),
			"PORT":              []byte(strconv.Itoa(pg.GetPort())),
			"ARGS":              []byte(pg.GetArgs()),
		},
	}

	// Set owner references
	err := controllerutil.SetControllerReference(instance, secret, r.Scheme)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *PostgresqlUserRoleReconciler) managePGUserRights(
	_ context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
	pgInstanceCache map[string]postgres.PG,
	pgecDBPrivilegeCache map[string][]*dbPrivilegeCache,
	username string,
) error {
	// Loop on pg instances
	for key, pgInstance := range pgInstanceCache {
		// Get membership
		memberOf, err := pgInstance.GetRoleMembership(username)
		// Check error
		if err != nil {
			return err
		}

		// Get set role settings for user
		setRoleSettings, err := pgInstance.GetSetRoleOnDatabasesRoleSettings(username)
		// Check error
		if err != nil {
			return err
		}

		// Get pgdb and user privilege
		dbPrivilegeCacheList := pgecDBPrivilegeCache[key]

		// Loop over privilege cache list
		for _, pcache := range dbPrivilegeCacheList {
			groupRole := r.getDBRoleFromPrivilege(pcache.DBInstance, pcache.UserPrivilege)
			// Check if item is in the list
			contains := funk.ContainsString(memberOf, groupRole)
			// Check if it doesn't contain
			if !contains {
				// Add right
				err = pgInstance.GrantRole(groupRole, username)
				// Check error
				if err != nil {
					return err
				}

				logger.Info("Successfully granted user in engine", "postgresqlEngine", key, "groupRole", groupRole)
				r.Recorder.Eventf(instance, "Normal", "Updated", "Successfully granted user to %s in engine %s", groupRole, key)
			} else {
				// Remove from list to keep only the deletion ones
				memberOf = funk.SubtractString(memberOf, []string{groupRole})
			}

			// Check if role setting isn't found
			found := funk.Find(setRoleSettings, func(c *postgres.SetRoleOnDatabaseRoleSetting) bool {
				return c.Database == pcache.DBInstance.Status.Database
			})
			// Check if not found or if group role have changed
			if found == nil || found.(*postgres.SetRoleOnDatabaseRoleSetting).Role != groupRole { //nolint:forcetypeassert//We know
				// Add alter
				err = pgInstance.AlterDefaultLoginRoleOnDatabase(username, groupRole, pcache.DBInstance.Status.Database)
				// Check error
				if err != nil {
					return err
				}

				logger.Info(
					"Successfully altered default login role in engine on specific database",
					"postgresqlEngine", key,
					"groupRole", groupRole,
					"postgresqlDatabase", utils.CreateNameKey(pcache.DBInstance.Name, pcache.DBInstance.Namespace, instance.Namespace),
				)
				r.Recorder.Eventf(
					instance,
					"Normal",
					"Updated",
					"Successfully altered default login role %s in engine %s on specific database %s",
					groupRole, key, utils.CreateNameKey(pcache.DBInstance.Name, pcache.DBInstance.Namespace, instance.Namespace),
				)
			}

			// Check if element have been found
			if found != nil {
				// Remove from list to keep only the deletion ones
				// TODO Check if can be better to use samber/lo instead of funk
				setRoleSettings, _ = funk.Subtract(setRoleSettings, []*postgres.SetRoleOnDatabaseRoleSetting{found.(*postgres.SetRoleOnDatabaseRoleSetting)}).([]*postgres.SetRoleOnDatabaseRoleSetting)
			}
		}

		// Manage revoke
		for _, role := range memberOf {
			// Revoke
			err = pgInstance.RevokeRole(role, username)
			// Check error
			if err != nil {
				return err
			}

			logger.Info("Successfully revoked role from user in engine", "postgresqlEngine", key, "role", role)
			r.Recorder.Eventf(instance, "Normal", "Updated", "Successfully revoked role %s from user in engine %s", role, key)
		}

		// Manage revoke set role
		for _, item := range setRoleSettings {
			err = pgInstance.RevokeUserSetRoleOnDatabase(item.Role, item.Database)
			// Check error
			if err != nil {
				return err
			}

			logger.Info("Successfully revoked set role from user on specific database in engine", "postgresqlEngine", key, "role", item.Role, "database", item.Database)
			r.Recorder.Eventf(instance, "Normal", "Updated", "Successfully revoked set role %s from user on specific database %s in engine %s", item.Role, item.Database, key)
		}
	}

	// Default
	return nil
}

func (*PostgresqlUserRoleReconciler) getDBRoleFromPrivilege(dbInstance *v1alpha1.PostgresqlDatabase, userRolePrivilege *v1alpha1.PostgresqlUserRolePrivilege) string {
	switch userRolePrivilege.Privilege {
	case v1alpha1.ReaderPrivilege:
		return dbInstance.Status.Roles.Reader
	case v1alpha1.WriterPrivilege:
		return dbInstance.Status.Roles.Writer
	default:
		return dbInstance.Status.Roles.Owner
	}
}

func (r *PostgresqlUserRoleReconciler) managePGUserRoles(
	_ context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
	pgInstanceCache map[string]postgres.PG,
	username, password string,
	passwordChanged bool,
) error {
	// Loop over all pg instances
	for key, pgInstance := range pgInstanceCache {
		// Check if user exists in database
		exists, err := pgInstance.IsRoleExist(username)
		// Check error
		if err != nil {
			return err
		}
		// Check if role doesn't exist to create it
		if !exists {
			// Create role
			_, err = pgInstance.CreateUserRole(username, password)
			// Check error
			if err != nil {
				return err
			}

			logger.Info("Successfully created user in engine", "postgresqlEngine", key)
			r.Recorder.Eventf(instance, "Normal", "Updated", "Successfully created user in engine %s", key)
			// Stop here
			continue
		}

		// Check if it is the first time this instance is managed
		// If yes and if the user exist, the password must be ensured
		// Or if the password have changed, change password
		if passwordChanged || instance.Status.Phase == v1alpha1.UserRoleNoPhase {
			err = pgInstance.UpdatePassword(username, password)
			// Check error
			if err != nil {
				return err
			}

			logger.Info("Successfully updated user password in engine", "postgresqlEngine", key)
			r.Recorder.Eventf(instance, "Normal", "Updated", "Successfully updated user password in engine %s", key)
		}
	}

	// Default
	return nil
}

func (r *PostgresqlUserRoleReconciler) createOrUpdateWorkSecretForManagedMode( //nolint:revive // We have multiple return, we know
	ctx context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
) (*corev1.Secret, string, bool, bool, error) {
	// Prepare values
	oldUsername := ""
	passwordChanged := false
	username := instance.Spec.RolePrefix + Login0Suffix
	password := utils.GetRandomString(ManagedPasswordSize)

	// Create or update work secret with imported secret values
	// Get current work secret
	workSec, err := utils.GetSecret(ctx, r.Client, instance.Spec.WorkGeneratedSecretName, instance.Namespace)
	// Check if error isn't a not found error
	if err != nil && !errors.IsNotFound(err) {
		return nil, "", false, false, err
	}
	// Check if error exist and not found
	// or check is secret must be updated.
	if err != nil && errors.IsNotFound(err) { //nolint:gocritic // Won't change to a switch
		// Check if we are in the init phase
		// If not, that case shouldn't happened and a password change must be ensured as we cannot compare with previous
		// Also we must compare with the username previously set to check if username have changed
		if instance.Status.Phase != v1alpha1.UserRoleNoPhase {
			// Save password changed to ensure password
			passwordChanged = true

			logger.Info("Detection of work secret that have been deleted whereas user have already been managed. Need to ensure password")

			// Check username
			if instance.Status.PostgresRole != "" && instance.Status.PostgresRole != username {
				// Save old username
				oldUsername = instance.Status.PostgresRole
				// Consider password as not changed in fact as the user have changed
				passwordChanged = false

				logger.Info("Detection of work secret that have been deleted whereas user have already been managed and username have changed")
			}
		}

		// Create new secret
		workSec, err = r.newWorkSecret(instance, username, password)
		// Check error
		if err != nil {
			return nil, "", false, false, err
		}

		// Save secret
		err = r.Create(ctx, workSec)
		// Check error
		if err != nil {
			return nil, "", false, false, err
		}

		logger.Info("Successfully created work secret")
		r.Recorder.Event(instance, "Normal", "Updated", "Work secret created")
	} else if (instance.Status.RolePrefix != "" && instance.Spec.RolePrefix != instance.Status.RolePrefix) ||
		string(workSec.Data[UsernameSecretKey]) == "" ||
		string(workSec.Data[PasswordSecretKey]) == "" { // Check if role have been changed or if work secret have been edited
		// Need to perform changes

		// Update flags
		passwordChanged = true
		oldUsername = string(workSec.Data[UsernameSecretKey])
		// Check if old username is the same as the new one
		// Note: This can happen when work secret is edited and only password is removed
		if oldUsername == username {
			oldUsername = ""
		}

		// Create new secret
		generatedSecret, err := r.newWorkSecret(instance, username, password)
		// Check error
		if err != nil {
			return nil, "", false, false, err
		}
		// Update secret with new content
		workSec.Data = generatedSecret.Data

		// Update secret
		err = r.Update(ctx, workSec)
		// Check error
		if err != nil {
			return nil, "", false, false, err
		}

		logger.Info("Successfully updated work secret with new user/password tuple because role name have changed or work secret have been edited")
		r.Recorder.Event(instance, "Normal", "Updated", "Work secret updated with new user/password tuple because role name have changed or work secret have been edited")
		r.Recorder.Event(workSec, "Normal", "Updated", "Secret updated by PostgresqlUserRole controller")
	} else if instance.Spec.UserPasswordRotationDuration != "" && instance.Status.LastPasswordChangedTime != "" { // Check if rolling password is enabled and a previous run have been performed
		// Get duration
		dur, err := time.ParseDuration(instance.Spec.UserPasswordRotationDuration)
		// Check error
		if err != nil {
			return nil, "", false, false, err
		}

		// Check if is time to change
		now := time.Now()
		lastChange, err := time.Parse(time.RFC3339, instance.Status.LastPasswordChangedTime)
		// Check error
		if err != nil {
			return nil, "", false, false, err
		}

		if now.Sub(lastChange) >= dur {
			// Need to change username/password with a new one

			// Get old username
			oldUsername = string(workSec.Data[UsernameSecretKey])
			// Prepare data
			username = instance.Spec.RolePrefix
			// Build "new" username
			if strings.HasSuffix(oldUsername, Login0Suffix) {
				username += Login1Suffix
			} else {
				username += Login0Suffix
			}

			// Check if this "new" username is in the "oldPostgresRoles" section
			// If yes, then ignore rolling and mark as error because the previous rolling wasn't a success
			// If no, continue
			if funk.ContainsString(instance.Status.OldPostgresRoles, username) {
				// Force stop without any action
				return workSec, "", false, true, nil
			}

			// Create new secret
			workSec, err = r.newWorkSecret(instance, username, password)
			// Check error
			if err != nil {
				return nil, "", false, false, err
			}

			// Update secret
			err = r.Update(ctx, workSec)
			// Check error
			if err != nil {
				return nil, "", false, false, err
			}

			// Save
			passwordChanged = true

			logger.Info("Successfully updated work secret with new user/password tuple because user password rotation have been triggered")
			r.Recorder.Event(instance, "Normal", "Updated", "Work secret updated with new user/password tuple because user password rotation have been triggered")
			r.Recorder.Event(workSec, "Normal", "Updated", "Secret updated by PostgresqlUserRole controller")
		}
	}

	return workSec, oldUsername, passwordChanged, false, nil
}

func (r *PostgresqlUserRoleReconciler) createOrUpdateWorkSecretForProvidedMode(
	ctx context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
) (*corev1.Secret, string, bool, error) {
	// Get import secret
	importSecret, err := utils.GetSecret(ctx, r.Client, instance.Spec.ImportSecretName, instance.Namespace)
	// Check error
	if err != nil {
		return nil, "", false, err
	}

	// Save data
	username := string(importSecret.Data[UsernameSecretKey])
	password := string(importSecret.Data[PasswordSecretKey])
	oldUsername := ""
	passwordChanged := false

	// Create or update work secret with imported secret values
	// Get current work secret
	workSec, err := utils.GetSecret(ctx, r.Client, instance.Spec.WorkGeneratedSecretName, instance.Namespace)
	// Check if error isn't a not found error
	if err != nil && !errors.IsNotFound(err) {
		return nil, "", false, err
	}
	// Check if error exist and not found
	// or check is secret must be updated.
	if err != nil && errors.IsNotFound(err) {
		// Check if we are in the init phase
		// If not, that case shouldn't happened and a password change must be ensured as we cannot compare with previous
		// Also we must compare with the username previously set to check if username have changed
		if instance.Status.Phase != v1alpha1.UserRoleNoPhase {
			// Save password changed to ensure password
			passwordChanged = true

			logger.Info("Detection of work secret that have been deleted whereas user have already been managed. Need to ensure password")

			// Check username
			if instance.Status.PostgresRole != "" && instance.Status.PostgresRole != username {
				// Save old username
				oldUsername = instance.Status.PostgresRole
				// Consider password as not changed in fact as the user have changed
				passwordChanged = false

				logger.Info("Detection of work secret that have been deleted whereas user have already been managed and username have changed")
			}
		}

		// Create new secret
		workSec, err = r.newWorkSecret(instance, username, password)
		// Check error
		if err != nil {
			return nil, "", false, err
		}

		// Save secret
		err = r.Create(ctx, workSec)
		// Check error
		if err != nil {
			return nil, "", false, err
		}

		logger.Info("Successfully created work secret")
		r.Recorder.Event(instance, "Normal", "Updated", "Work secret created")
	} else if string(workSec.Data[UsernameSecretKey]) != username || string(workSec.Data[PasswordSecretKey]) != password {
		// Update status
		oldUsername = string(workSec.Data[UsernameSecretKey])
		passwordChanged = string(workSec.Data[PasswordSecretKey]) != password
		// Update and save secret
		workSec.Data[UsernameSecretKey] = []byte(username)
		workSec.Data[PasswordSecretKey] = []byte(password)

		// Update secret
		err = r.Update(ctx, workSec)
		// Check error
		if err != nil {
			return nil, "", false, err
		}

		logger.Info("Successfully updated work secret")
		// Add event
		r.Recorder.Event(workSec, "Normal", "Updated", "Secret updated")
		r.Recorder.Event(instance, "Normal", "Updated", "Work secret updated")
	}

	return workSec, oldUsername, passwordChanged, nil
}

func (r *PostgresqlUserRoleReconciler) newWorkSecret(instance *v1alpha1.PostgresqlUserRole, username, password string) (*corev1.Secret, error) {
	labels := map[string]string{
		"app.kubernetes.io/name": instance.Name,
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.WorkGeneratedSecretName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Data: map[string][]byte{
			UsernameSecretKey: []byte(username),
			PasswordSecretKey: []byte(password),
		},
	}

	// Set owner references
	err := controllerutil.SetControllerReference(instance, secret, r.Scheme)
	if err != nil {
		return nil, err
	}

	return secret, err
}

func (r *PostgresqlUserRoleReconciler) manageActiveSessionsAndDropOldRoles(
	_ context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
	pgInstanceCache map[string]postgres.PG,
	pgecDBPrivilegeCache map[string][]*dbPrivilegeCache,
) error {
	// Build new list of old roles
	newOldRoleList := make([]string, 0)

	// Loop over old users
	for _, oldUsername := range instance.Status.OldPostgresRoles {
		// Loop over db cache
		for key, pgInstance := range pgInstanceCache {
			// Check if old username exists
			exists, err := pgInstance.IsRoleExist(oldUsername)
			// Check error
			if err != nil {
				return err
			}

			// Check if it exists
			if exists {
				// Check if there is a currently active session with this user
				sessionDetected, err := pgInstance.DoesRoleHaveActiveSession(oldUsername)
				// Check error
				if err != nil {
					return err
				}

				// Check session isn't active
				if !sessionDetected {
					// Get all databases linked to this engine
					dbPrivilegeCacheList := pgecDBPrivilegeCache[key]
					// Loop over databases
					for _, item := range dbPrivilegeCacheList {
						// Some PG instance are limited and this can be done in generic
						// This limitation needs to add the main user as member of the current role
						err = pgInstance.GrantRole(oldUsername, pgInstance.GetUser())
						// Check error
						if err != nil {
							return err
						}
						// Change and drop owner by
						err = pgInstance.ChangeAndDropOwnedBy(oldUsername, item.DBInstance.Status.Roles.Owner, item.DBInstance.Status.Database)
						// Check error
						if err != nil {
							return err
						}
					}
					// Drop it
					err = pgInstance.DropRole(oldUsername)
					// Check error
					if err != nil {
						return err
					}

					logger.Info("Role successfully deleted", "engine", key, "role", oldUsername)
					r.Recorder.Eventf(instance, "Normal", "Processing", "Role %s successfully deleted on engine %s", oldUsername, key)
				} else {
					// Active session, save account as must be deleted after
					newOldRoleList = append(newOldRoleList, oldUsername)
					logger.Info("Role still active sessions, ignoring deletion", "engine", key, "role", oldUsername)
					r.Recorder.Eventf(instance, "Warning", "Warning", "Role %s still have active session on engine %s, ignoring deletion", oldUsername, key)
				}
			}
		}
	}

	// Save new list
	instance.Status.OldPostgresRoles = funk.UniqString(newOldRoleList)

	// Default
	return nil
}

func (r *PostgresqlUserRoleReconciler) getPGInstances(
	ctx context.Context,
	logger logr.Logger,
	dbCache map[string]*v1alpha1.PostgresqlDatabase,
	ignoreNotFound bool,
) (map[string]postgres.PG, error) {
	// Prepare result
	res := make(map[string]postgres.PG)

	// Prepare temporary map
	// Loop
	for _, item := range dbCache {
		// Build key
		key := utils.CreateNameKey(item.Spec.EngineConfiguration.Name, item.Spec.EngineConfiguration.Namespace, item.Namespace)

		// Get value from cache
		_, ok := res[key]

		// Check if key have a value
		if ok {
			// Ignore
			continue
		}

		pgec, err := utils.FindPgEngineCfg(ctx, r.Client, item)
		// Check error
		if err != nil {
			if errors.IsNotFound(err) && ignoreNotFound {
				// Ignore and continue
				continue
			}

			// Return
			return nil, err
		}

		sec, err := utils.FindSecretPgEngineCfg(ctx, r.Client, pgec)
		// Check error
		if err != nil {
			if errors.IsNotFound(err) && ignoreNotFound {
				// Ignore and continue
				continue
			}

			// Return
			return nil, err
		}

		// Save
		res[key] = utils.CreatePgInstance(logger, sec.Data, pgec)
	}

	return res, nil
}

func (r *PostgresqlUserRoleReconciler) getDatabaseInstances(
	ctx context.Context,
	instance *v1alpha1.PostgresqlUserRole,
	ignoreNotFound bool,
) (map[string]*v1alpha1.PostgresqlDatabase, map[string][]*dbPrivilegeCache, error) {
	// Prepare result
	res := make(map[string]*v1alpha1.PostgresqlDatabase)
	res2 := make(map[string][]*dbPrivilegeCache)

	// Loop
	for _, item := range instance.Spec.Privileges {
		// Get PG DB instance
		pgdb, err := utils.FindPgDatabaseFromLink(ctx, r.Client, item.Database, instance.Namespace)
		// Check error
		if err != nil {
			if errors.IsNotFound(err) && ignoreNotFound {
				// Ignore and continue
				continue
			}

			// Return
			return nil, nil, err
		}
		// Save in first map
		res[utils.CreateNameKey(pgdb.Name, pgdb.Namespace, instance.Namespace)] = pgdb

		// Create pgec instance key
		pgecKey := utils.CreateNameKey(pgdb.Spec.EngineConfiguration.Name, pgdb.Spec.EngineConfiguration.Namespace, pgdb.Namespace)
		// Get item
		arry, ok := res2[pgecKey]
		// Check if array exists
		if !ok {
			arry = make([]*dbPrivilegeCache, 0)
		}

		// Append
		arry = append(arry, &dbPrivilegeCache{
			DBInstance:    pgdb,
			UserPrivilege: item,
		})

		// Save
		res2[pgecKey] = arry
	}

	return res, res2, nil
}

func (r *PostgresqlUserRoleReconciler) validateInstance(
	ctx context.Context,
	instance *v1alpha1.PostgresqlUserRole,
) error {
	// Validate secret in case of provided mode
	if instance.Spec.Mode == v1alpha1.ProvidedMode {
		// Check mode
		if instance.Spec.ImportSecretName == "" {
			return errors.NewBadRequest("PostgresqlUserRole is in provided mode without any ImportSecretName")
		}

		// Get secret
		sec, err := utils.GetSecret(ctx, r.Client, instance.Spec.ImportSecretName, instance.Namespace)
		// Check error
		if err != nil {
			return err
		}

		// Build username
		username := string(sec.Data[UsernameSecretKey])
		// Check values
		if username == "" || string(sec.Data[PasswordSecretKey]) == "" {
			return errors.NewBadRequest("Import secret must have a " + UsernameSecretKey + " and " + PasswordSecretKey + " valuated keys")
		}

		// Check if username length is acceptable
		if len(username) > postgres.MaxIdentifierLength {
			errStr := fmt.Sprintf("Username is too long. It must be <= %d. %s is %d character. Username length must be reduced", postgres.MaxIdentifierLength, username, len(username))

			return errors.NewBadRequest(errStr)
		}
	} else {
		// Validate Managed one

		// Must have a role prefix
		if instance.Spec.RolePrefix == "" {
			return errors.NewBadRequest("PostgresqlUserRole is in managed mode without any RolePrefix")
		}

		// Build username
		username := instance.Spec.RolePrefix + Login0Suffix + "X" // Adding extra item to have more space for the future.
		// Check if username length is acceptable
		if len(username) > postgres.MaxIdentifierLength {
			errStr := fmt.Sprintf("Role prefix is too long. It must be <= %d. %s is %d character. Role prefix length must be reduced", postgres.MaxIdentifierLength, username, len(username))

			return errors.NewBadRequest(errStr)
		}

		// Check if rolling update password is enabled
		if instance.Spec.UserPasswordRotationDuration != "" {
			// Try to parse duration
			_, err := time.ParseDuration(instance.Spec.UserPasswordRotationDuration)
			// Check error
			if err != nil {
				return err
			}
		}
	}

	// Validate not multiple time the same db in the list of privileges
	for i, privi := range instance.Spec.Privileges {
		// Search for the same db
		for j, privi2 := range instance.Spec.Privileges {
			// Check that this isn't the same item
			if i != j {
				// Prepare values
				priviNamespace := privi.Database.Namespace
				privi2Namespace := privi2.Database.Namespace

				// Populate with instance
				if priviNamespace == "" {
					priviNamespace = instance.Namespace
				}

				if privi2Namespace == "" {
					privi2Namespace = instance.Namespace
				}

				// Check
				if privi.Database.Name == privi2.Database.Name && priviNamespace == privi2Namespace {
					return errors.NewBadRequest("Privilege list mustn't have the same database listed multiple times")
				}
			}
		}
	}

	// Check if role prefix is set
	if instance.Spec.RolePrefix != "" {
		// Check that role prefix is unique in the whole cluster
		// Create temporary values
		nextMarker := ""
		continueLoop := true

		for continueLoop {
			// Prepare list
			list := &v1alpha1.PostgresqlUserRoleList{}

			// List request
			err := r.List(ctx, list, &client.ListOptions{Continue: nextMarker, Limit: ListLimit})
			// Check error
			if err != nil {
				return err
			}

			// Save data
			nextMarker = list.Continue
			continueLoop = nextMarker != ""

			// Loop over all users
			for _, userInstance := range list.Items {
				// Check that role prefix isn't declared in another user
				// TODO Try to validate that this is unique per engine and not for the whole cluster
				if userInstance.Name != instance.Name && userInstance.Namespace != instance.Namespace && userInstance.Spec.RolePrefix == instance.Spec.RolePrefix {
					return errors.NewBadRequest("RolePrefix is declared in another PostgresqlUser. This field value must be unique.")
				}
			}
		}
	}

	// Default
	return nil
}

func (r *PostgresqlUserRoleReconciler) updateInstance(
	ctx context.Context,
	instance *v1alpha1.PostgresqlUserRole,
) (bool, error) {
	// Deep copy
	oCopy := instance.DeepCopy()

	// Add finalizer
	controllerutil.AddFinalizer(instance, config.Finalizer)

	// Update work generated secret with a generated uuid
	if instance.Spec.WorkGeneratedSecretName == "" {
		instance.Spec.WorkGeneratedSecretName = DefaultWorkGeneratedSecretNamePrefix + strings.ToLower(utils.GetRandomString(DefaultWorkGeneratedSecretNameRandomLength))
	}

	// Check if update is needed
	if !reflect.DeepEqual(oCopy.ObjectMeta, instance.ObjectMeta) || !reflect.DeepEqual(oCopy.Spec, instance.Spec) {
		return true, r.Update(ctx, instance)
	}

	return false, nil
}

func (r *PostgresqlUserRoleReconciler) manageError(
	ctx context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
	originalPatch client.Patch,
	issue error,
) (reconcile.Result, error) {
	logger.Error(issue, "issue raised in reconcile")
	// Add kubernetes event
	r.Recorder.Event(instance, "Warning", "ProcessingError", issue.Error())

	// Update status
	instance.Status.Message = issue.Error()
	instance.Status.Ready = false
	instance.Status.Phase = v1alpha1.UserRoleFailedPhase

	// Patch status
	err := r.Status().Patch(ctx, instance, originalPatch)
	if err != nil {
		logger.Error(err, "unable to update status")
	}

	// Requeue
	return reconcile.Result{
		RequeueAfter: PGURRequeueDelayErrorNumberSeconds * time.Second,
		Requeue:      true,
	}, nil
}

func (r *PostgresqlUserRoleReconciler) manageSuccess(
	ctx context.Context,
	logger logr.Logger,
	instance *v1alpha1.PostgresqlUserRole,
	originalPatch client.Patch,
) (reconcile.Result, error) {
	// Update status
	instance.Status.Message = ""
	instance.Status.Ready = true
	instance.Status.Phase = v1alpha1.UserRoleCreatedPhase

	// Patch status
	err := r.Status().Patch(ctx, instance, originalPatch)
	if err != nil {
		logger.Error(err, "unable to update status")

		return reconcile.Result{
			RequeueAfter: PGURRequeueDelayErrorNumberSeconds * time.Second,
			Requeue:      true,
		}, nil
	}

	logger.Info("Reconcile done")

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlUserRoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PostgresqlUserRole{}).
		Complete(r)
}
