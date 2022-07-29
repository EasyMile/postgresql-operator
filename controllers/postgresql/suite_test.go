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
	"testing"
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/apis/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/controllers/config"
	"github.com/easymile/postgresql-operator/controllers/postgresql/postgres"
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
var pgecNamespace = "pgec-ns"
var pgecName = "pgec-object"
var pgecSecretName = "pgec-secret"
var pgdbNamespace = "pgdb-ns"
var pgdbName = "pgdb-object"
var pgdbDBName = "super-db"
var pguNamespace = "pgu-ns"
var pguName = "pgu-object"
var pgdbSchemaName1 = "one_schema"
var pgdbSchemaName2 = "second_schema"
var pgdbExtensionName1 = "uuid-ossp"
var pgdbExtensionName2 = "cube"
var postgresUser = "postgres"
var postgresPassword = "postgres"
var postgresUrl = "postgresql://postgres:postgres@localhost:5432/?sslmode=disable"
var postgresUrlToDB = "postgresql://postgres:postgres@localhost:5432/" + pgdbDBName + "?sslmode=disable"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
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

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sManager).ToNot(BeNil())

	Expect((&PostgresqlEngineConfigurationReconciler{
		Client:   k8sClient,
		Log:      logf.Log.WithName("controllers"),
		Recorder: k8sManager.GetEventRecorderFor("controller"),
		Scheme:   scheme.Scheme,
	}).SetupWithManager(k8sManager)).ToNot(HaveOccurred())

	Expect((&PostgresqlDatabaseReconciler{
		Client:   k8sClient,
		Log:      logf.Log.WithName("controllers"),
		Recorder: k8sManager.GetEventRecorderFor("controller"),
		Scheme:   scheme.Scheme,
	}).SetupWithManager(k8sManager)).ToNot(HaveOccurred())

	Expect((&PostgresqlUserReconciler{
		Client:   k8sClient,
		Log:      logf.Log.WithName("controllers"),
		Recorder: k8sManager.GetEventRecorderFor("controller"),
		Scheme:   scheme.Scheme,
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
}, 60)

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func cleanupFunction() {
	// Force delete pgec
	err := deletePGEC(ctx, k8sClient, pgecName, pgecNamespace)
	Expect(err).ToNot(HaveOccurred())
	// Force delete secrets
	err = deleteSecret(ctx, k8sClient, pgecSecretName, pgecNamespace)
	Expect(err).ToNot(HaveOccurred())

	Expect(deletePGU(ctx, k8sClient, pguName, pguNamespace)).ToNot(HaveOccurred())
	Expect(deletePGDB(ctx, k8sClient, pgdbName, pgdbNamespace)).ToNot(HaveOccurred())
	Expect(deleteSQLDB(pgdbDBName)).ToNot(HaveOccurred())
	Expect(deleteSQLRoles()).ToNot(HaveOccurred())
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
	obj controllerutil.Object,
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

func setupPGEC(
	checkInterval string,
	waitLinkedResourcesDeletion bool,
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
			WaitLinkedResourcesDeletion: waitLinkedResourcesDeletion,
			SecretName:                  pgecSecretName,
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
	// Create pgdb
	pgdb := &postgresqlv1alpha1.PostgresqlDatabase{
		ObjectMeta: v1.ObjectMeta{
			Name:      pgdbName,
			Namespace: pgdbNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
			Database:                    pgdbDBName,
			WaitLinkedResourcesDeletion: waitLinkedResourcesDeletion,
			EngineConfiguration: &postgresqlv1alpha1.CRLink{
				Name:      pgecName,
				Namespace: pgecNamespace,
			},
		},
	}

	// Create
	Expect(k8sClient.Create(ctx, pgdb)).Should(Succeed())

	// Get updated
	Eventually(
		func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgdbName,
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

func setupPGU() *postgresqlv1alpha1.PostgresqlUser {
	// Create pgu
	item := &postgresqlv1alpha1.PostgresqlUser{
		ObjectMeta: v1.ObjectMeta{
			Name:      pguName,
			Namespace: pguNamespace,
		},
		Spec: postgresqlv1alpha1.PostgresqlUserSpec{
			RolePrefix: "pgu",
			Database: &postgresqlv1alpha1.CRLink{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			GeneratedSecretNamePrefix: "pgu",
			Privileges:                postgresqlv1alpha1.OwnerPrivilege,
		},
	}

	// Create
	Expect(k8sClient.Create(ctx, item)).Should(Succeed())

	// Get created
	Eventually(
		func() error {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      pguName,
				Namespace: pguNamespace,
			}, item)
			// Check error
			if err != nil {
				return err
			}

			// Check if status hasn't been updated
			if item.Status.Phase == postgresqlv1alpha1.UserNoPhase {
				return gerrors.New("pgu hasn't been updated by operator")
			}

			return nil
		},
		generalEventuallyTimeout,
		generalEventuallyInterval,
	).
		Should(Succeed())

	return item
}

func deletePGU(ctx context.Context, cl client.Client, name, namespace string) error {
	// Create structure
	st := &postgresqlv1alpha1.PostgresqlUser{}
	// Delete
	return deleteObject(ctx, cl, name, namespace, st)
}

func deleteSQLDB(name string) error {
	// Connect
	db, err := sql.Open("postgres", postgresUrl)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	// Try to delete
	for i := 0; i < 1000; i++ {
		_, err = db.Exec(fmt.Sprintf(postgres.DropDatabaseSQLTemplate, name))
		if err == nil {
			return nil
		}

		// Try to cast error
		// Error code 3D000 is returned if database doesn't exist
		// Error code 55006 is returned if there are connections still open
		pqErr, ok := err.(*pq.Error)

		if !ok || (pqErr.Code != "3D000" && pqErr.Code != "55006") {
			return err
		}

		if pqErr.Code == "3D000" {
			return nil
		}

	}

	// Default
	return nil
}

func createSQLDB(name, role string) error {
	// Connect
	db, err := sql.Open("postgres", postgresUrl)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	_, err = db.Exec(fmt.Sprintf(postgres.CreateDBSQLTemplate, name, role))
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
	// Connect
	db, err := sql.Open("postgres", postgresUrl)
	// Check error
	if err != nil {
		return false, err
	}

	defer func() error {
		return db.Close()
	}()

	res, err := db.Exec(fmt.Sprintf(postgres.IsDatabaseExistSQLTemplate, name))
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
	db, err := sql.Open("postgres", postgresUrl)
	if err != nil {
		return err
	}

	res, err := db.Query(postgres.GetAllCreatedRolesSQLTemplate)
	if err != nil {
		return err
	}

	var role string
	for res.Next() {
		err = res.Scan(&role)
		if err != nil {
			return err
		}

		_, err = db.Exec(fmt.Sprintf(postgres.DropRoleSQLTemplate, role))
		if err != nil {
			return err
		}
	}

	return nil
}

func createSQLRole(role string) error {
	// Connect
	db, err := sql.Open("postgres", postgresUrl)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	_, err = db.Exec(fmt.Sprintf(postgres.CreateGroupRoleSQLTemplate, role))
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
	// Connect
	db, err := sql.Open("postgres", postgresUrl)
	// Check error
	if err != nil {
		return false, err
	}

	defer func() error {
		return db.Close()
	}()

	res, err := db.Exec(fmt.Sprintf(postgres.IsRoleExistSQLTemplate, name))
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
	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return false, err
	}

	defer func() error {
		return db.Close()
	}()

	res, err := db.Exec(fmt.Sprintf(postgres.IsSchemaExistSQLTemplate, name))
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
	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return false, err
	}

	defer func() error {
		return db.Close()
	}()

	res, err := db.Exec(fmt.Sprintf(postgres.IsExtensionExistSQLTemplate, name))
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

func createTableInSchema(schema, table string) error {
	// Connect
	db, err := sql.Open("postgres", postgresUrlToDB)
	// Check error
	if err != nil {
		return err
	}

	defer func() error {
		return db.Close()
	}()

	_, err = db.Exec(fmt.Sprintf(postgres.CreateTableInSchemaTemplate, schema, table))
	if err != nil {
		return err
	}

	return nil
}

func isSQLUserMemberOf(user, group string) (bool, error) {
	// Connect
	db, err := sql.Open("postgres", postgresUrl)
	// Check error
	if err != nil {
		return false, err
	}

	defer func() error {
		return db.Close()
	}()

	res, err := db.Exec(fmt.Sprintf(postgres.IsMemberOfSQLTemplate, user, group))
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

func checkRoleInSQLDb(user, role string) {
	roleExists, roleErr := isSQLRoleExists(role)
	Expect(roleErr).ToNot(HaveOccurred())
	Expect(roleExists).To(BeTrue())

	memberOf, memberOfErr := isSQLUserMemberOf(user, role)
	Expect(memberOfErr).ToNot(HaveOccurred())
	Expect(memberOf).To(BeTrue())
}
