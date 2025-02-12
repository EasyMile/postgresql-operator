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
	"database/sql"
	gerrors "errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/easymile/postgresql-operator/api/postgresql/common"
	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/internal/controller/config"
	"github.com/easymile/postgresql-operator/internal/controller/postgresql/postgres"
	"github.com/easymile/postgresql-operator/internal/controller/utils"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc
var generalEventuallyTimeout = 60 * time.Second
var generalEventuallyInterval = time.Second
var pgpublicationNamespace = "pgpub-ns"
var pgpublicationName = "pgpub-object"
var pgpublicationPublicationName1 = "pub1"
var pgpublicationCustomReplicationSlotName = "replslotname"
var pgecNamespace = "pgec-ns"
var pgecName = "pgec-object"
var pgecSecretName = "pgec-secret"
var pgdbNamespace = "pgdb-ns"
var pgdbName = "pgdb-object"
var pgdbName2 = "pgdb-object2"
var pgdbDBName = "super-db"
var pgdbDBName2 = "super-db2"
var pguNamespace = "pgu-ns"
var pguName = "pgu-object"
var pgurNamespace = "pgur-ns"
var pgurName = "pgur-object"
var pgurWorkSecretName = "pgur-work-secret"
var pgurDBSecretName = "pgur-db-secret"
var pgurDBSecretName2 = "pgur-db-secret2"
var pgurImportSecretName = "pgu-import-secret"
var pgurImportUsername = "fake-username"
var pgurImportPassword = "fake-password"
var pgurRolePrefix = "role-prefix"
var pgdbSchemaName1 = "one_schema"
var pgdbSchemaName2 = "second_schema"
var pgPublicSchemaName = "public"
var pgdbExtensionName1 = "uuid-ossp"
var pgdbExtensionName2 = "cube"
var postgresUser = "postgres"
var postgresPassword = "postgres"
var postgresUrlWithDbTemplate = "postgresql://%s:%s@localhost:5432/%s?sslmode=disable"
var postgresUrl = "postgresql://postgres:postgres@localhost:5432/?sslmode=disable"
var postgresUrlToDB = "postgresql://postgres:postgres@localhost:5432/" + pgdbDBName + "?sslmode=disable"
var editedSecretName = "updated-secret-name"
var dbConns = map[string]*struct {
	tx *sql.Tx
	db *sql.DB
}{}
var mainDBConn *sql.DB
var controllerRuntimeDetailedErrorTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "controller_runtime_reconcile_detailed_errors_total",
		Help: "Total number of reconciliation errors per controller detailed with resource namespace and name.",
	},
	[]string{"controller", "namespace", "name"},
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func(_ context.Context) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = postgresqlv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	resyncPeriod := 10 * time.Second
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:     scheme.Scheme,
		SyncPeriod: &resyncPeriod,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sManager).ToNot(BeNil())

	Expect((&PostgresqlEngineConfigurationReconciler{
		Client:                              k8sClient,
		Log:                                 logf.Log.WithName("controllers"),
		Recorder:                            k8sManager.GetEventRecorderFor("controller"),
		Scheme:                              scheme.Scheme,
		ControllerRuntimeDetailedErrorTotal: controllerRuntimeDetailedErrorTotal,
		ControllerName:                      "postgresqlengineconfiguration",
		ReconcileTimeout:                    10 * time.Second,
	}).SetupWithManager(k8sManager)).ToNot(HaveOccurred())

	Expect((&PostgresqlDatabaseReconciler{
		Client:                              k8sClient,
		Log:                                 logf.Log.WithName("controllers"),
		Recorder:                            k8sManager.GetEventRecorderFor("controller"),
		Scheme:                              scheme.Scheme,
		ControllerRuntimeDetailedErrorTotal: controllerRuntimeDetailedErrorTotal,
		ControllerName:                      "postgresqldatabase",
		ReconcileTimeout:                    10 * time.Second,
	}).SetupWithManager(k8sManager)).ToNot(HaveOccurred())

	Expect((&PostgresqlUserRoleReconciler{
		Client:                              k8sClient,
		Log:                                 logf.Log.WithName("controllers"),
		Recorder:                            k8sManager.GetEventRecorderFor("controller"),
		Scheme:                              scheme.Scheme,
		ControllerRuntimeDetailedErrorTotal: controllerRuntimeDetailedErrorTotal,
		ControllerName:                      "postgresqluserrole",
		ReconcileTimeout:                    10 * time.Second,
	}).SetupWithManager(k8sManager)).ToNot(HaveOccurred())

	Expect((&PostgresqlPublicationReconciler{
		Client:                              k8sClient,
		Log:                                 logf.Log.WithName("controllers"),
		Recorder:                            k8sManager.GetEventRecorderFor("controller"),
		Scheme:                              scheme.Scheme,
		ControllerRuntimeDetailedErrorTotal: controllerRuntimeDetailedErrorTotal,
		ControllerName:                      "postgresqlpublication",
		ReconcileTimeout:                    10 * time.Second,
	}).SetupWithManager(k8sManager)).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: pgecNamespace,
		},
	})).ToNot(HaveOccurred())

	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: pgdbNamespace,
		},
	})).ToNot(HaveOccurred())

	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: pguNamespace,
		},
	})).ToNot(HaveOccurred())

	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: pgurNamespace,
		},
	})).ToNot(HaveOccurred())

	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: pgpublicationNamespace,
		},
	})).ToNot(HaveOccurred())
}, NodeTimeout(60*time.Second))

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())

	// Close db
	for k, _ := range dbConns {
		disconnectConnFromKey(k)
	}
	if mainDBConn != nil {
		Expect(mainDBConn.Close()).To(Succeed())
	}
})

func starAny[T any](s T) *T {
	return &s
}

func cleanupFunction() {
	for k, _ := range dbConns {
		disconnectConnFromKey(k)
	}

	// Force delete pgec
	err := deletePGEC(ctx, k8sClient, pgecName, pgecNamespace)
	Expect(err).ToNot(HaveOccurred())
	err = deleteSecret(ctx, k8sClient, pgecSecretName, pgecNamespace)
	Expect(err).ToNot(HaveOccurred())

	Expect(deletePGPublication(ctx, k8sClient, pgpublicationName, pgpublicationNamespace)).ToNot(HaveOccurred())
	Expect(deletePGUR(ctx, k8sClient, pgurName, pgurNamespace)).ToNot(HaveOccurred())
	Expect(deletePGDB(ctx, k8sClient, pgdbName, pgdbNamespace)).ToNot(HaveOccurred())
	Expect(deletePGDB(ctx, k8sClient, pgdbName2, pgdbNamespace)).ToNot(HaveOccurred())

	// Close all connections in operator pool
	// For this, use utils methods and official pool methods
	Expect(postgres.CloseDatabaseSavedPoolsForName(
		utils.CreateNameKeyForSavedPools(pgecName, pgecNamespace),
		pgdbDBName,
	)).ToNot(HaveOccurred())
	Expect(postgres.CloseDatabaseSavedPoolsForName(
		utils.CreateNameKeyForSavedPools(pgecName, pgecNamespace),
		pgdbDBName2,
	)).ToNot(HaveOccurred())

	Expect(dropReplicationSlot(pgpublicationPublicationName1))
	Expect(dropReplicationSlot(pgpublicationCustomReplicationSlotName))
	Expect(deleteSQLDBs(pgdbDBName)).ToNot(HaveOccurred())
	Expect(deleteSQLDBs(pgdbDBName2)).ToNot(HaveOccurred())
	Expect(deleteSQLRoles()).ToNot(HaveOccurred())

	// Force delete secrets
	err = deleteSecret(ctx, k8sClient, pgurImportSecretName, pgurNamespace)
	Expect(err).ToNot(HaveOccurred())
	err = deleteSecret(ctx, k8sClient, pgurDBSecretName, pgurNamespace)
	Expect(err).ToNot(HaveOccurred())
	err = deleteSecret(ctx, k8sClient, pgurDBSecretName2, pgurNamespace)
	Expect(err).ToNot(HaveOccurred())
	err = deleteSecret(ctx, k8sClient, pgurWorkSecretName, pgurNamespace)
	Expect(err).ToNot(HaveOccurred())
	err = deleteSecret(ctx, k8sClient, editedSecretName, pgurNamespace)
	Expect(err).ToNot(HaveOccurred())
}

func getSecret(ctx context.Context, cli client.Client, name, namespace string) (*corev1.Secret, error) {
	sec := &corev1.Secret{}
	// Get secret
	err := cli.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, sec)

	return sec, err
}

func deleteSecret(ctx context.Context, cl client.Client, name, namespace string) error {
	// Create secret structure
	secret := &corev1.Secret{}
	// Delete
	return deleteObject(ctx, cl, name, namespace, secret)
}

func deleteObject(
	ctx context.Context,
	cl client.Client,
	name, namespace string,
	obj client.Object,
) error {
	// Get item
	err := cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj)
	// Check error
	if err != nil {
		// Check if error is a not found error
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	// Delete finalizer
	controllerutil.RemoveFinalizer(obj, config.Finalizer)

	// Update it
	err = cl.Update(ctx, obj)
	// Check error
	if err != nil {
		// Check if error is a not found error
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	// Do the remove
	err = cl.Delete(ctx, obj)
	// Check error
	if err != nil {
		// Check if error is a not found error
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	// Get item to force cache clean
	// Loop until it is cleaned or max try
	for i := 0; i < 1000; i++ {
		err = cl.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj)
		// Check error
		if err != nil {
			// Check if error is a not found error
			if errors.IsNotFound(err) {
				return nil
			}

			return err
		}
		// Check if object is cleaned
		if obj == nil {
			return nil
		}
	}

	return gerrors.New("object not cleaned")
}

func setupPGECSecret() *corev1.Secret {
	// Create secret
	sec := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgecSecretName,
			Namespace: pgecNamespace,
		},
		StringData: map[string]string{
			"user":     postgresUser,
			"password": postgresPassword,
		},
	}

	Expect(k8sClient.Create(ctx, sec)).To(Succeed())

	return sec
}

func setupPGURImportSecret() *corev1.Secret {
	// Create secret
	sec := &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurImportSecretName,
			Namespace: pgurNamespace,
		},
		StringData: map[string]string{
			"USERNAME": pgurImportUsername,
			"PASSWORD": pgurImportPassword,
		},
	}

	Expect(k8sClient.Create(ctx, sec)).To(Succeed())

	return sec
}

func setupProvidedPGUR() *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                    postgresqlv1alpha1.ProvidedMode,
			ImportSecretName:        pgurImportSecretName,
			WorkGeneratedSecretName: pgurWorkSecretName,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
			},
		},
	}

	return setupSavePGURInternal(it)
}
func setupProvidedPGURAndPartialCustomAttributes() *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                    postgresqlv1alpha1.ProvidedMode,
			ImportSecretName:        pgurImportSecretName,
			WorkGeneratedSecretName: pgurWorkSecretName,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
			},
			RoleAttributes: &postgresqlv1alpha1.PostgresqlUserRoleAttributes{
				Replication:     starAny(true),
				ConnectionLimit: starAny(5),
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupProvidedPGURWithBouncer() *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                    postgresqlv1alpha1.ProvidedMode,
			ImportSecretName:        pgurImportSecretName,
			WorkGeneratedSecretName: pgurWorkSecretName,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					ConnectionType:      postgresqlv1alpha1.BouncerConnectionType,
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupProvidedPGURWith2Databases() *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                    postgresqlv1alpha1.ProvidedMode,
			ImportSecretName:        pgurImportSecretName,
			WorkGeneratedSecretName: pgurWorkSecretName,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
				{
					Privilege:           postgresqlv1alpha1.WriterPrivilege,
					Database:            &common.CRLink{Name: pgdbName2, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName2,
				},
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupProvidedPGURWith2DatabasesWithPrimaryAndBouncer() *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                    postgresqlv1alpha1.ProvidedMode,
			ImportSecretName:        pgurImportSecretName,
			WorkGeneratedSecretName: pgurWorkSecretName,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					ConnectionType:      postgresqlv1alpha1.PrimaryConnectionType,
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
				{
					ConnectionType:      postgresqlv1alpha1.BouncerConnectionType,
					Privilege:           postgresqlv1alpha1.WriterPrivilege,
					Database:            &common.CRLink{Name: pgdbName2, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName2,
				},
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupManagedPGURWithBouncer(userPasswordRotationDuration string) *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                         postgresqlv1alpha1.ManagedMode,
			RolePrefix:                   pgurRolePrefix,
			WorkGeneratedSecretName:      pgurWorkSecretName,
			UserPasswordRotationDuration: userPasswordRotationDuration,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					ConnectionType:      postgresqlv1alpha1.BouncerConnectionType,
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupManagedPGUR(userPasswordRotationDuration string) *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                         postgresqlv1alpha1.ManagedMode,
			RolePrefix:                   pgurRolePrefix,
			WorkGeneratedSecretName:      pgurWorkSecretName,
			UserPasswordRotationDuration: userPasswordRotationDuration,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupManagedPGURWithPartialCustomAttributes() *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                    postgresqlv1alpha1.ManagedMode,
			RolePrefix:              pgurRolePrefix,
			WorkGeneratedSecretName: pgurWorkSecretName,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
			},
			RoleAttributes: &postgresqlv1alpha1.PostgresqlUserRoleAttributes{
				Replication:     starAny(true),
				ConnectionLimit: starAny(5),
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupManagedPGURWith2Databases() *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                    postgresqlv1alpha1.ManagedMode,
			RolePrefix:              pgurRolePrefix,
			WorkGeneratedSecretName: pgurWorkSecretName,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
				{
					Privilege:           postgresqlv1alpha1.WriterPrivilege,
					Database:            &common.CRLink{Name: pgdbName2, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName2,
				},
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupManagedPGURWith2DatabasesWithPrimaryAndBouncer() *postgresqlv1alpha1.PostgresqlUserRole {
	it := &postgresqlv1alpha1.PostgresqlUserRole{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgurName,
			Namespace: pgurNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
			Mode:                    postgresqlv1alpha1.ManagedMode,
			RolePrefix:              pgurRolePrefix,
			WorkGeneratedSecretName: pgurWorkSecretName,
			Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
				{
					ConnectionType:      postgresqlv1alpha1.PrimaryConnectionType,
					Privilege:           postgresqlv1alpha1.OwnerPrivilege,
					Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName,
				},
				{
					ConnectionType:      postgresqlv1alpha1.BouncerConnectionType,
					Privilege:           postgresqlv1alpha1.WriterPrivilege,
					Database:            &common.CRLink{Name: pgdbName2, Namespace: pgdbNamespace},
					GeneratedSecretName: pgurDBSecretName2,
				},
			},
		},
	}

	return setupSavePGURInternal(it)
}

func setupSavePGURInternal(it *postgresqlv1alpha1.PostgresqlUserRole) *postgresqlv1alpha1.PostgresqlUserRole {
	// Create user
	Expect(k8sClient.Create(ctx, it)).Should(Succeed())

	// Get updated user
	Eventually(
		func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      it.Name,
				Namespace: it.Namespace,
			}, it)
			// Check error
			if err != nil {
				return err
			}

			// Check if status hasn't been updated
			if it.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
				return gerrors.New("pgur hasn't been updated by operator")
			}

			return nil
		},
		generalEventuallyTimeout,
		generalEventuallyInterval,
	).
		Should(Succeed())

	return it
}

func setupPGPublicationWithPartialSpec(partialSpec postgresqlv1alpha1.PostgresqlPublicationSpec) *postgresqlv1alpha1.PostgresqlPublication {
	it := &postgresqlv1alpha1.PostgresqlPublication{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgpublicationName,
			Namespace: pgpublicationNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
			Database:              &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
			Name:                  pgpublicationPublicationName1,
			AllTables:             partialSpec.AllTables,
			TablesInSchema:        partialSpec.TablesInSchema,
			Tables:                partialSpec.Tables,
			WithParameters:        partialSpec.WithParameters,
			DropOnDelete:          partialSpec.DropOnDelete,
			ReplicationSlotName:   partialSpec.ReplicationSlotName,
			ReplicationSlotPlugin: partialSpec.ReplicationSlotPlugin,
		},
	}

	return setupSavePGPublicationInternal(it)
}

func setupSavePGPublicationInternal(it *postgresqlv1alpha1.PostgresqlPublication) *postgresqlv1alpha1.PostgresqlPublication {
	// Create user
	Expect(k8sClient.Create(ctx, it)).Should(Succeed())

	// Get updated user
	Eventually(
		func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      it.Name,
				Namespace: it.Namespace,
			}, it)
			// Check error
			if err != nil {
				return err
			}

			// Check if status hasn't been updated
			if it.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
				return gerrors.New("pgpub hasn't been updated by operator")
			}

			return nil
		},
		generalEventuallyTimeout,
		generalEventuallyInterval,
	).
		Should(Succeed())

	return it
}

func setupPGEC(
	checkInterval string,
	waitLinkedResourcesDeletion bool,
) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, *corev1.Secret) {
	return setupPGECInternal(checkInterval, waitLinkedResourcesDeletion, &postgresqlv1alpha1.GenericUserConnection{
		Host:    "localhost",
		Port:    5432,
		URIArgs: "sslmode=disable",
	}, nil, nil, nil, false)
}

func setupPGECWithBouncer(
	checkInterval string,
	waitLinkedResourcesDeletion bool,
) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, *corev1.Secret) {
	return setupPGECInternal(checkInterval, waitLinkedResourcesDeletion, &postgresqlv1alpha1.GenericUserConnection{
		Host:    "localhost",
		Port:    5432,
		URIArgs: "sslmode=disable",
	}, &postgresqlv1alpha1.GenericUserConnection{
		Host:    "localhost",
		Port:    5433,
		URIArgs: "sslmode=disable",
	}, nil, nil, false)
}

func setupPGECWithReplica(
	checkInterval string,
	waitLinkedResourcesDeletion bool,
) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, *corev1.Secret) {
	uc := &postgresqlv1alpha1.GenericUserConnection{
		Host:    "localhost",
		Port:    5432,
		URIArgs: "sslmode=disable",
	}
	return setupPGECInternal(checkInterval, waitLinkedResourcesDeletion, uc, nil, []*postgresqlv1alpha1.GenericUserConnection{uc}, nil, false)
}

func setupPGECWithBouncerAndReplica(
	checkInterval string,
	waitLinkedResourcesDeletion bool,
) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, *corev1.Secret) {
	uc := &postgresqlv1alpha1.GenericUserConnection{
		Host:    "localhost",
		Port:    5432,
		URIArgs: "sslmode=disable",
	}
	buc := &postgresqlv1alpha1.GenericUserConnection{
		Host:    "localhost",
		Port:    5433,
		URIArgs: "sslmode=disable",
	}
	return setupPGECInternal(checkInterval, waitLinkedResourcesDeletion, uc, buc, []*postgresqlv1alpha1.GenericUserConnection{uc}, []*postgresqlv1alpha1.GenericUserConnection{buc}, false)
}

func setupPGECWithAllowGrantAdminOption(
	checkInterval string,
	waitLinkedResourcesDeletion bool,
) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, *corev1.Secret) {
	return setupPGECInternal(checkInterval, waitLinkedResourcesDeletion, &postgresqlv1alpha1.GenericUserConnection{
		Host:    "localhost",
		Port:    5432,
		URIArgs: "sslmode=disable",
	}, nil, nil, nil, true)
}

func setupPGECInternal(
	checkInterval string,
	waitLinkedResourcesDeletion bool,
	primaryUserConnection, bouncerUserConnection *postgresqlv1alpha1.GenericUserConnection,
	replicaUserConnections, replicaBouncerUserConnections []*postgresqlv1alpha1.GenericUserConnection,
	allowGrantAdminOption bool,
) (*postgresqlv1alpha1.PostgresqlEngineConfiguration, *corev1.Secret) {
	// Create secret
	sec := setupPGECSecret()

	// Create pgec
	pgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgecName,
			Namespace: pgecNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlEngineConfigurationSpec{
			Provider:                    "",
			Host:                        "localhost",
			Port:                        5432,
			URIArgs:                     "sslmode=disable",
			DefaultDatabase:             "postgres",
			CheckInterval:               checkInterval,
			AllowGrantAdminOption:       allowGrantAdminOption,
			WaitLinkedResourcesDeletion: waitLinkedResourcesDeletion,
			SecretName:                  pgecSecretName,
			UserConnections: &postgresqlv1alpha1.UserConnections{
				PrimaryConnection:         primaryUserConnection,
				BouncerConnection:         bouncerUserConnection,
				ReplicaConnections:        replicaUserConnections,
				ReplicaBouncerConnections: replicaBouncerUserConnections,
			},
		},
	}

	// Create
	Expect(k8sClient.Create(ctx, pgec)).Should(Succeed())

	// Get updated
	Eventually(
		func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgecName,
				Namespace: pgecNamespace,
			}, pgec)
			// Check error
			if err != nil {
				return err
			}

			// Check if status hasn't been updated
			if pgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
				return gerrors.New("pgec hasn't been updated by operator")
			}

			// Check if status is ready
			if !pgec.Status.Ready {
				return gerrors.New("pgec isn't valid")
			}

			return nil
		},
		generalEventuallyTimeout,
		generalEventuallyInterval,
	).
		Should(Succeed())

	return pgec, sec
}

func deletePGEC(ctx context.Context, cl client.Client, name, namespace string) error {
	// Create provider structure
	prov := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
	// Delete
	return deleteObject(ctx, cl, name, namespace, prov)
}

func setupPGDB(
	waitLinkedResourcesDeletion bool,
) *postgresqlv1alpha1.PostgresqlDatabase {
	return setupSavePGDBInternal(waitLinkedResourcesDeletion, pgdbName, pgdbDBName)
}

func setupPGDB2() *postgresqlv1alpha1.PostgresqlDatabase {
	return setupSavePGDBInternal(false, pgdbName2, pgdbDBName2)
}

func setupSavePGDBInternal(
	waitLinkedResourcesDeletion bool,
	name string,
	dbName string,
) *postgresqlv1alpha1.PostgresqlDatabase {
	// Create pgdb
	pgdb := &postgresqlv1alpha1.PostgresqlDatabase{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: pgdbNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
			Database:                    dbName,
			WaitLinkedResourcesDeletion: waitLinkedResourcesDeletion,
			EngineConfiguration: &common.CRLink{
				Name:      pgecName,
				Namespace: pgecNamespace,
			},
			DropOnDelete: true,
		},
	}

	// Create
	Expect(k8sClient.Create(ctx, pgdb)).Should(Succeed())

	// Get updated
	Eventually(
		func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      name,
				Namespace: pgdbNamespace,
			}, pgdb)
			// Check error
			if err != nil {
				return err
			}

			// Check if status hasn't been updated
			if pgdb.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
				return gerrors.New("pgdb hasn't been updated by operator")
			}

			// Check if status is ready
			if !pgdb.Status.Ready {
				return gerrors.New("pgdb isn't valid")
			}

			return nil
		},
		generalEventuallyTimeout,
		generalEventuallyInterval,
	).
		Should(Succeed())

	return pgdb
}

func deletePGDB(ctx context.Context, cl client.Client, name, namespace string) error {
	// Create structure
	st := &postgresqlv1alpha1.PostgresqlDatabase{}
	// Delete
	return deleteObject(ctx, cl, name, namespace, st)
}

func deletePGUR(ctx context.Context, cl client.Client, name, namespace string) error {
	// Create structure
	st := &postgresqlv1alpha1.PostgresqlUserRole{}
	// Delete
	return deleteObject(ctx, cl, name, namespace, st)
}

func deletePGPublication(ctx context.Context, cl client.Client, name, namespace string) error {
	// Create structure
	st := &postgresqlv1alpha1.PostgresqlPublication{}
	// Delete
	return deleteObject(ctx, cl, name, namespace, st)
}

func deleteSQLDBs(name string) error {
	// Query template
	GetAllCreatedSQLDBTemplate := "SELECT datname FROM pg_database WHERE datname LIKE '%" + name + "%';"

	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return err
		}
		mainDBConn = db
	}

	res, err := mainDBConn.Query(GetAllCreatedSQLDBTemplate)
	if err != nil {
		return err
	}

	var dbname string
	for res.Next() {
		err = res.Scan(&dbname)
		if err != nil {
			return err
		}

		// Try to delete
		for i := 0; i < 1000; i++ {
			_, err = mainDBConn.Exec(fmt.Sprintf(postgres.DropDatabaseSQLTemplate, dbname))
			if err == nil {
				break
			}

			// Try to cast error
			// Error code 3D000 is returned if database doesn't exist
			// Error code 55006 is returned if there are connections still open
			pqErr, ok := err.(*pq.Error)

			if !ok || (pqErr.Code != "3D000" && pqErr.Code != "55006") {
				return err
			}
		}
	}

	// Default
	return nil
}

func createSQLDB(name, role string) error {
	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return err
		}
		mainDBConn = db
	}

	_, err := mainDBConn.Exec(fmt.Sprintf(postgres.CreateDBSQLTemplate, name, role))
	if err != nil {
		// eat DUPLICATE DATABASE ERROR
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != postgres.DuplicateDatabaseErrorCode {
			return err
		}
	}

	return nil
}

func isSQLDBExists(name string) (bool, error) {
	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return false, err
		}
		mainDBConn = db
	}

	res, err := mainDBConn.Exec(fmt.Sprintf(`SELECT 1 FROM pg_database WHERE datname='%s'`, name))
	if err != nil {
		return false, err
	}
	// Get affected rows
	nb, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return nb == 1, nil
}

func deleteSQLRoles() error {
	// Query template
	GetAllCreatedRolesSQLTemplate := `SELECT rolname FROM pg_roles WHERE rolname NOT LIKE 'pg\_%' AND rolname != 'postgres'`

	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return err
		}
		mainDBConn = db
	}

	res, err := mainDBConn.Query(GetAllCreatedRolesSQLTemplate)
	if err != nil {
		return err
	}

	var role string
	for res.Next() {
		err = res.Scan(&role)
		if err != nil {
			return err
		}

		_, err = mainDBConn.Exec(fmt.Sprintf(postgres.DropRoleSQLTemplate, role))
		if err != nil {
			return err
		}
	}

	return nil
}

func createSQLRole(role string) error {
	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return err
		}
		mainDBConn = db
	}

	_, err := mainDBConn.Exec(fmt.Sprintf(postgres.CreateGroupRoleSQLTemplate, role))
	if err != nil {
		// eat DUPLICATE ROLE ERROR
		// Try to cast error
		pqErr, ok := err.(*pq.Error)
		if !ok || pqErr.Code != postgres.DuplicateRoleErrorCode {
			return err
		}
	}

	return nil
}

func isSQLRoleExists(name string) (bool, error) {
	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return false, err
		}
		mainDBConn = db
	}

	res, err := mainDBConn.Exec(fmt.Sprintf(postgres.IsRoleExistSQLTemplate, name))
	if err != nil {
		return false, err
	}
	// Get affected rows
	nb, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return nb == 1, nil
}

func isSQLSchemaExists(name string) (bool, error) {
	// Query template
	IsSchemaExistSQLTemplate := `SELECT 1 FROM information_schema.schemata WHERE schema_name='%s'`

	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return false, err
	}

	defer func() error {
		return db.Close()
	}()

	res, err := db.Exec(fmt.Sprintf(IsSchemaExistSQLTemplate, name))
	if err != nil {
		return false, err
	}
	// Get affected rows
	nb, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return nb == 1, nil
}

func isSQLExtensionExists(name string) (bool, error) {
	// Query template
	IsExtensionExistSQLTemplate := `SELECT 1 FROM pg_extension WHERE extname='%s'`

	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return false, err
	}

	defer func() error {
		return db.Close()
	}()

	res, err := db.Exec(fmt.Sprintf(IsExtensionExistSQLTemplate, name))
	if err != nil {
		return false, err
	}
	// Get affected rows
	nb, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return nb == 1, nil
}

func getTableOwnerInSchema(dbName, schemaName, tableName string) (string, error) {
	// Connect
	db, err := sql.Open("postgres", fmt.Sprintf(postgresUrlWithDbTemplate, postgresUser, postgresPassword, dbName))
	// Check error
	if err != nil {
		return "", err
	}

	defer db.Close()

	sqlTemplate := `select tableowner from pg_tables where tablename = '%s' and schemaname = '%s';`
	res, err := db.Query(fmt.Sprintf(sqlTemplate, tableName, schemaName))
	if err != nil {
		return "", err
	}

	var owner string
	for res.Next() {
		err = res.Scan(&owner)
		if err != nil {
			return "", err
		}
	}

	// Rows error
	err = res.Err()
	// Check error
	if err != nil {
		return "", err
	}

	return owner, nil
}

func rawSQLQuery(raw string) error {
	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	_, err = db.Exec(raw)
	if err != nil {
		return err
	}

	return nil
}

func createTableInSchemaAsAdmin(schema, table string) error {
	// Query template
	CreateTableInSchemaTemplate := `CREATE TABLE %s.%s()`

	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	_, err = db.Exec(fmt.Sprintf(CreateTableInSchemaTemplate, schema, table))
	if err != nil {
		return err
	}

	return nil
}

func createColumnInTable(table, columnName, columnType string) error {
	tmpl := `ALTER TABLE IF EXISTS %s ADD COLUMN %s %s;`

	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	_, err = db.Exec(fmt.Sprintf(tmpl, table, columnName, columnType))
	if err != nil {
		return err
	}

	return nil
}

func create2KnownTablesWithColumnsInPublicSchema() error {
	err := createTableInSchemaAsAdmin("public", "fake")
	if err != nil {
		return err
	}

	err = createTableInSchemaAsAdmin("public", "fake2")
	if err != nil {
		return err
	}

	err = createColumnInTable("public.fake", "id", "text")
	if err != nil {
		return err
	}
	err = createColumnInTable("public.fake", "nb", "integer")
	if err != nil {
		return err
	}
	err = createColumnInTable("public.fake", "nb2", "integer")
	if err != nil {
		return err
	}

	err = createColumnInTable("public.fake2", "id", "text")
	if err != nil {
		return err
	}
	err = createColumnInTable("public.fake2", "test", "integer")
	if err != nil {
		return err
	}

	return nil
}

// Here we are considering that type cannot be in another schema just for test.
// This is easier for test cases.
func getTypeOwner(dbName, typeName string) (string, error) {
	// Connect
	db, err := sql.Open("postgres", fmt.Sprintf(postgresUrlWithDbTemplate, postgresUser, postgresPassword, dbName))
	// Check error
	if err != nil {
		return "", err
	}

	defer db.Close()

	sqlTemplate := `SELECT typowner::regrole FROM pg_type WHERE typname = '%s';`
	res, err := db.Query(fmt.Sprintf(sqlTemplate, typeName))
	if err != nil {
		return "", err
	}

	var owner string
	for res.Next() {
		err = res.Scan(&owner)
		if err != nil {
			return "", err
		}
	}

	// Rows error
	err = res.Err()
	// Check error
	if err != nil {
		return "", err
	}

	// Clean member to remove extra "
	owner = strings.ReplaceAll(owner, `"`, "")

	return owner, nil
}

func createTypeInSchemaAsAdmin(schema, typeName string) error {
	// Query template
	CreateTypeInSchemaTemplate := `CREATE TYPE "%s"."%s" AS ENUM ('new', 'open', 'closed');`

	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	_, err = db.Exec(fmt.Sprintf(CreateTypeInSchemaTemplate, schema, typeName))
	if err != nil {
		return err
	}

	return nil
}

type PublicationResult struct {
	Owner              string
	AllTables          bool
	Insert             bool
	Update             bool
	Delete             bool
	Truncate           bool
	PublicationViaRoot bool
}

func getPublication(name string) (*PublicationResult, error) {
	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return nil, err
	}

	defer func() error {
		return db.Close()
	}()

	// Get rows
	rows, err := db.Query(fmt.Sprintf(postgres.GetPublicationSQLTemplate, name))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var res PublicationResult

	var foundOne bool

	for rows.Next() {
		// Scan
		err = rows.Scan(&res.Owner, &res.AllTables, &res.Insert, &res.Update, &res.Delete, &res.Truncate, &res.PublicationViaRoot)
		// Check error
		if err != nil {
			return nil, err
		}

		// Update marker
		foundOne = true
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	// Check if found marker isn't set
	if !foundOne {
		return nil, nil
	}

	return &res, nil
}

type PublicationTableDetail struct {
	SchemaName      string
	TableName       string
	Columns         []string
	AdditionalWhere *string
}

func getPublicationTableDetails(name string) ([]*PublicationTableDetail, error) {
	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return nil, err
	}

	defer func() error {
		return db.Close()
	}()

	// Get rows
	rows, err := db.Query(fmt.Sprintf(`SELECT
  schemaname, tablename, attnames, rowfilter
FROM pg_catalog.pg_publication_tables
WHERE pubname = '%s';`, name))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := make([]*PublicationTableDetail, 0)

	for rows.Next() {
		var it PublicationTableDetail
		var pqSA pq.StringArray
		// Scan
		err = rows.Scan(&it.SchemaName, &it.TableName, &pqSA, &it.AdditionalWhere)
		// Check error
		if err != nil {
			return nil, err
		}
		// Save
		// ? Note: getting a list of string from pg imply a decode
		// ? See issue: https://github.com/cockroachdb/cockroach/issues/39770#issuecomment-576170805
		it.Columns = pqSA

		// Save value
		res = append(res, &it)
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func dropReplicationSlot(name string) error {
	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	DropReplicationSlotSQLTemplate := `SELECT pg_drop_replication_slot('%s')`
	count := 0

	for count <= 20 {
		repl1, err := getReplicationSlotInternal(db, name)
		if err != nil {
			return err
		}

		if repl1 != nil {
			_, err = db.Exec(fmt.Sprintf(DropReplicationSlotSQLTemplate, name))
			if err != nil {
				return err
			}

			count += 1
			// Wait
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	repl1, err := getReplicationSlotInternal(db, name)
	if err != nil {
		return err
	}

	if repl1 != nil {
		return fmt.Errorf("replication slot not deleted after %d round", count)
	}

	// Default
	return nil
}

func createReplicationSlotInMainDB(name, plugin string) error {
	// Connect
	db, err := sql.Open("postgres", postgresUrl)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	CreateReplicationSlotSQLTemplate := `SELECT pg_create_logical_replication_slot('%s', '%s')`

	_, err = db.Exec(fmt.Sprintf(CreateReplicationSlotSQLTemplate, name, plugin))
	if err != nil {
		return err
	}

	// Default
	return nil
}

type replicationSlotResult struct {
	SlotName string
	Plugin   string
	Database string
}

func getReplicationSlot(name string) (*replicationSlotResult, error) {
	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return nil, err
	}

	defer func() error {
		return db.Close()
	}()

	return getReplicationSlotInternal(db, name)
}

func getReplicationSlotInternal(db *sql.DB, name string) (*replicationSlotResult, error) {
	GetReplicationSlotSQLTemplate := `SELECT slot_name,plugin,database FROM pg_replication_slots WHERE slot_name = '%s'`

	// Get rows
	rows, err := db.Query(fmt.Sprintf(GetReplicationSlotSQLTemplate, name))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var res replicationSlotResult

	var foundOne bool

	for rows.Next() {
		// Scan
		err = rows.Scan(&res.SlotName, &res.Plugin, &res.Database)
		// Check error
		if err != nil {
			return nil, err
		}

		// Update marker
		foundOne = true
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	// Check if found marker isn't set
	if !foundOne {
		return nil, nil
	}

	return &res, nil
}

type RoleAttributes struct {
	ConnectionLimit *int
	Replication     *bool
	BypassRLS       *bool
}

func getRoleAttributes(role string) (*RoleAttributes, error) {
	res := &RoleAttributes{
		ConnectionLimit: new(int),
		Replication:     new(bool),
		BypassRLS:       new(bool),
	}

	// Connect
	db, err := sql.Open("postgres", postgresUrl)
	// Check error
	if err != nil {
		return nil, err
	}

	defer func() error {
		return db.Close()
	}()

	GetRoleAttributesSQLTemplate := `select rolconnlimit, rolreplication, rolbypassrls FROM pg_roles WHERE rolname = '%s'`
	rows, err := db.Query(fmt.Sprintf(GetRoleAttributesSQLTemplate, role))
	if err != nil {
		return res, err
	}

	defer rows.Close()

	for rows.Next() {
		// Scan
		err = rows.Scan(res.ConnectionLimit, res.Replication, res.BypassRLS)
		// Check error
		if err != nil {
			return res, err
		}
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return res, err
	}

	return res, nil
}

func checkRoleInSQLDb(role string) {
	roleExists, roleErr := isSQLRoleExists(role)
	Expect(roleErr).ToNot(HaveOccurred())
	Expect(roleExists).To(BeTrue())
}

func connectAs(username, password string) (string, error) {
	u := fmt.Sprintf(postgresUrlWithDbTemplate, username, password, "postgres")
	// Connect
	db, err := sql.Open("postgres", u)
	// Check error
	if err != nil {
		return "", err
	}

	tx, err := db.Begin()
	// Check error
	if err != nil {
		return "", err
	}

	// Save
	dbConns[u] = &struct {
		tx *sql.Tx
		db *sql.DB
	}{
		tx: tx,
		db: db,
	}

	return u, nil
}

func disconnectConnFromKey(key string) error {
	if dbConns[key] == nil {
		return nil
	}

	err := dbConns[key].tx.Commit()
	if err != nil {
		return err
	}

	err = dbConns[key].db.Close()
	if err != nil {
		return err
	}

	delete(dbConns, key)

	return nil
}

func changeDBOwner(dbname, role string) error {
	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return err
		}
		mainDBConn = db
	}

	sqlTemplate := `ALTER DATABASE "%s" OWNER TO "%s"`
	_, err := mainDBConn.Exec(fmt.Sprintf(sqlTemplate, dbname, role))
	if err != nil {
		return err
	}

	return nil
}

func isRoleOwnerofSQLDB(dbname, role string) (bool, error) {
	// Query template
	IsRoleOwnerOfDbSQLTemplate := `SELECT 1 FROM pg_catalog.pg_database d WHERE d.datname = '%s' AND pg_catalog.pg_get_userbyid(d.datdba) = '%s';`

	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return false, err
		}
		mainDBConn = db
	}

	res, err := mainDBConn.Exec(fmt.Sprintf(IsRoleOwnerOfDbSQLTemplate, dbname, role))
	if err != nil {
		return false, err
	}
	// Get affected rows
	nb, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return nb == 1, nil
}

func getSQLRoleMembershipWithAdminOption(role string) (map[string]bool, error) {
	sqlTemplate := "SELECT member::regrole, admin_option FROM pg_auth_members where roleid='%s'::regrole;"

	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return nil, err
		}
		mainDBConn = db
	}

	rows, err := mainDBConn.Query(fmt.Sprintf(sqlTemplate, role))
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := map[string]bool{}

	for rows.Next() {
		member := ""
		adminOption := false
		// Scan
		err = rows.Scan(&member, &adminOption)
		// Check error
		if err != nil {
			return nil, err
		}
		// Clean member to remove extra "
		member = strings.ReplaceAll(member, `"`, "")
		// Save
		res[member] = adminOption
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return nil, err
	}

	return res, nil
}

func isSetRoleOnDatabasesRoleSettingsExists(username, databaseInput, groupRole string) (bool, error) {
	GetRoleSettingsSQLTemplate := `SELECT pg_catalog.split_part(pg_catalog.unnest(setconfig), '=', 1) as parameter_type, pg_catalog.split_part(pg_catalog.unnest(setconfig), '=', 2) as parameter_value, d.datname as database FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_db_role_setting c ON (c.setrole = r.oid) JOIN pg_catalog.pg_database d ON (d.oid = c.setdatabase) WHERE r.rolcanlogin AND r.rolname='%s'`

	if mainDBConn == nil {
		db, err := sql.Open("postgres", postgresUrl)
		if err != nil {
			return false, err
		}
		mainDBConn = db
	}

	rows, err := mainDBConn.Query(fmt.Sprintf(GetRoleSettingsSQLTemplate, username))
	if err != nil {
		return false, err
	}

	defer rows.Close()

	for rows.Next() {
		parameterType := ""
		parameterValue := ""
		database := ""
		// Scan
		err = rows.Scan(&parameterType, &parameterValue, &database)
		// Check error
		if err != nil {
			return false, err
		}

		// Check parameter type
		if parameterType != "role" {
			// Ignore
			continue
		}

		if database == databaseInput && parameterValue == groupRole {
			return true, nil
		}
	}

	// Rows error
	err = rows.Err()
	// Check error
	if err != nil {
		return false, err
	}

	return false, nil
}

func checkPGURSecretValues(
	name, namespace, dbName, username, password string,
	pgec *postgresqlv1alpha1.PostgresqlEngineConfiguration,
	userConnectionType postgresqlv1alpha1.ConnectionTypesSpecEnum,
) {
	checkPGURSecretValuesWithExtraArgs(name, namespace, dbName, username, password, pgec, userConnectionType, map[string]string{})
}

func checkPGURSecretValuesWithExtraArgs(
	name, namespace, dbName, username, password string,
	pgec *postgresqlv1alpha1.PostgresqlEngineConfiguration,
	userConnectionType postgresqlv1alpha1.ConnectionTypesSpecEnum,
	extraArgsMap map[string]string,
) {
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, secret)
	Expect(err).ToNot(HaveOccurred())

	// Init user connection with PRIMARY choice
	userCon := pgec.Spec.UserConnections.PrimaryConnection
	// Check if bouncer is selected
	if userConnectionType == postgresqlv1alpha1.BouncerConnectionType {
		userCon = pgec.Spec.UserConnections.BouncerConnection
	}

	// Compute uri args from main ones to user defined ones
	uriArgList := []string{userCon.URIArgs}
	// Loop over user defined list
	for k, v := range extraArgsMap {
		uriArgList = append(uriArgList, fmt.Sprintf("%s=%s", k, v))
	}
	// Join
	uriArgs := strings.Join(uriArgList, "&")

	Expect(string(secret.Data["POSTGRES_URL"])).To(Equal(
		fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", secret.Data["LOGIN"], secret.Data["PASSWORD"], userCon.Host, userCon.Port, dbName),
	))
	Expect(string(secret.Data["POSTGRES_URL_ARGS"])).To(Equal(fmt.Sprintf("%s?%s", secret.Data["POSTGRES_URL"], secret.Data["ARGS"])))
	Expect(secret.Data["PASSWORD"]).ToNot(BeEmpty())
	Expect(string(secret.Data["PASSWORD"])).To(Equal(password))
	Expect(string(secret.Data["LOGIN"])).To(Equal(username))
	Expect(string(secret.Data["DATABASE"])).To(Equal(dbName))
	Expect(string(secret.Data["HOST"])).To(Equal(userCon.Host))
	Expect(string(secret.Data["PORT"])).To(Equal(fmt.Sprint(userCon.Port)))
	Expect(string(secret.Data["ARGS"])).To(Equal(uriArgs))

	// Check replica data
	rucList := pgec.Spec.UserConnections.ReplicaConnections
	// Check if bouncer is selected
	if userConnectionType == postgresqlv1alpha1.BouncerConnectionType {
		rucList = pgec.Spec.UserConnections.ReplicaBouncerConnections
	}
	// Loop over them to validate
	for i, userCon := range rucList {
		// Compute uri args from main ones to user defined ones
		uriArgList := []string{userCon.URIArgs}
		// Loop over user defined list
		for k, v := range extraArgsMap {
			uriArgList = append(uriArgList, fmt.Sprintf("%s=%s", k, v))
		}
		// Join
		uriArgs := strings.Join(uriArgList, "&")

		Expect(string(secret.Data["REPLICA_"+strconv.Itoa(i)+"_POSTGRES_URL"])).To(Equal(
			fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", secret.Data["LOGIN"], secret.Data["PASSWORD"], userCon.Host, userCon.Port, dbName),
		))
		Expect(string(secret.Data["REPLICA_"+strconv.Itoa(i)+"_POSTGRES_URL_ARGS"])).To(Equal(fmt.Sprintf("%s?%s", secret.Data["POSTGRES_URL"], secret.Data["ARGS"])))
		Expect(secret.Data["REPLICA_"+strconv.Itoa(i)+"_PASSWORD"]).ToNot(BeEmpty())
		Expect(string(secret.Data["REPLICA_"+strconv.Itoa(i)+"_PASSWORD"])).To(Equal(password))
		Expect(string(secret.Data["REPLICA_"+strconv.Itoa(i)+"_LOGIN"])).To(Equal(username))
		Expect(string(secret.Data["REPLICA_"+strconv.Itoa(i)+"_DATABASE"])).To(Equal(dbName))
		Expect(string(secret.Data["REPLICA_"+strconv.Itoa(i)+"_HOST"])).To(Equal(userCon.Host))
		Expect(string(secret.Data["REPLICA_"+strconv.Itoa(i)+"_PORT"])).To(Equal(fmt.Sprint(userCon.Port)))
		Expect(string(secret.Data["REPLICA_"+strconv.Itoa(i)+"_ARGS"])).To(Equal(uriArgs))
	}
}
