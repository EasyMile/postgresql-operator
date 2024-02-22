package postgresql

import (
	"errors"
	gerrors "errors"
	"fmt"
	"reflect"

	"github.com/easymile/postgresql-operator/api/postgresql/common"
	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PostgresqlDatabase tests", func() {
	AfterEach(cleanupFunction)

	It("shouldn't accept input without any specs", func() {
		err := k8sClient.Create(ctx, &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
		})

		Expect(err).To(HaveOccurred())

		// Cast error
		stErr, ok := err.(*apimachineryErrors.StatusError)

		Expect(ok).To(BeTrue())

		// Check that content is correct
		causes := stErr.Status().Details.Causes

		Expect(causes).To(HaveLen(2))

		// Search all fields
		fields := map[string]bool{
			"spec.database":            false,
			"spec.engineConfiguration": false,
		}

		// Loop over all causes
		for _, cause := range causes {
			fields[cause.Field] = true
		}

		// Check that all fields are found
		for key, value := range fields {
			if !value {
				err := fmt.Errorf("%s found be found in error causes", key)
				Expect(err).ToNot(HaveOccurred())
			}
		}
	})

	It("should fail to look a not found pgec", func() {
		// Create pgec
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      "fake",
					Namespace: "fake",
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(item.Status.Ready).To(BeFalse())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseFailedPhase))
		Expect(item.Status.Message).To(ContainSubstring("\"fake\" not found"))
	})

	It("should be ok to set only required values", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		ownerRole := fmt.Sprintf("%s-owner", pgdbDBName)
		readerRole := fmt.Sprintf("%s-reader", pgdbDBName)
		writerRole := fmt.Sprintf("%s-writer", pgdbDBName)

		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Database).To(Equal(pgdbDBName))
		Expect(item.Status.Message).To(BeEmpty())
		Expect(item.Status.Roles.Owner).To(Equal(ownerRole))
		Expect(item.Status.Roles.Reader).To(Equal(readerRole))
		Expect(item.Status.Roles.Writer).To(Equal(writerRole))

		// Check if DB exists
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Check if roles exists and are granted to default user
		checkRoleInSQLDb(postgresUser, ownerRole)
		checkRoleInSQLDb(postgresUser, readerRole)
		checkRoleInSQLDb(postgresUser, writerRole)

		// Check DB ownership
		isOwner, err := isRoleOwnerofSQLDB(pgdbDBName, ownerRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(isOwner).To(BeTrue())
	})

	It("should be ok to set all values (required & optional)", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				MasterRole:                  "master",
				DropOnDelete:                false,
				WaitLinkedResourcesDeletion: false,
				Schemas: postgresqlv1alpha1.DatabaseModulesList{
					List:              make([]string, 0),
					DropOnOnDelete:    false,
					DeleteWithCascade: false,
				},
				Extensions: postgresqlv1alpha1.DatabaseModulesList{
					List:              make([]string, 0),
					DropOnOnDelete:    false,
					DeleteWithCascade: false,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		ownerRole := "master"
		readerRole := fmt.Sprintf("%s-reader", pgdbDBName)
		writerRole := fmt.Sprintf("%s-writer", pgdbDBName)

		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Database).To(Equal(pgdbDBName))
		Expect(item.Status.Message).To(BeEmpty())
		Expect(item.Status.Roles.Owner).To(Equal(ownerRole))
		Expect(item.Status.Roles.Reader).To(Equal(readerRole))
		Expect(item.Status.Roles.Writer).To(Equal(writerRole))

		// Check if DB exists
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Check if roles exists and are granted to default user
		checkRoleInSQLDb(postgresUser, ownerRole)
		checkRoleInSQLDb(postgresUser, readerRole)
		checkRoleInSQLDb(postgresUser, writerRole)
	})

	It("should drop database on crd deletion if DropOnDelete set to true", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				DropOnDelete: true,
			},
		}

		// First create CR
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				// Check if status hasn't been updated
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check DB exists
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Then delete CR
		Expect(k8sClient.Delete(ctx, item)).Should(Succeed())

		deletedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, deletedItem)

				if err == nil {
					return errors.New("should be deleted but not deleted")
				}

				// Check if error isn't a not found error
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check DB does not exists anymore
		stillExists, stillErr := isSQLDBExists(pgdbDBName)
		Expect(stillErr).ToNot(HaveOccurred())
		Expect(stillExists).To(BeFalse())
	})

	It("should keep database on crd deletion if DropOnDelete set to false", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				DropOnDelete: false,
			},
		}

		// First create CR
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				// Check if status hasn't been updated
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check DB exists
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Then delete CR
		Expect(k8sClient.Delete(ctx, item)).Should(Succeed())

		deletedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, deletedItem)

				if err == nil {
					return errors.New("should be deleted but not deleted")
				}

				// Check if error isn't a not found error
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check DB still exists
		stillExists, stillErr := isSQLDBExists(pgdbDBName)
		Expect(stillErr).ToNot(HaveOccurred())
		Expect(stillExists).To(BeTrue())
	})

	It("should be ok to have a pgdb referencing an existing PG database", func() {
		// Create SQL db
		errDB := createSQLDB(pgdbDBName, postgresUser)
		Expect(errDB).ToNot(HaveOccurred())

		// Create pgec
		setupPGEC("10s", false)

		// Create pgdb
		item := setupPGDB(true)

		// Checks
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Database).To(Equal(pgdbDBName))
		Expect(item.Status.Message).To(BeEmpty())

		// Check if roles exists and are granted to default user
		ownerRole := fmt.Sprintf("%s-owner", pgdbDBName)
		checkRoleInSQLDb(postgresUser, ownerRole)

		readerRole := fmt.Sprintf("%s-reader", pgdbDBName)
		checkRoleInSQLDb(postgresUser, readerRole)

		writerRole := fmt.Sprintf("%s-writer", pgdbDBName)
		checkRoleInSQLDb(postgresUser, writerRole)
	})

	It("should be ok to have a pgdb referencing an existing master role", func() {
		// Create SQL role
		sqlRole := "super-role"
		errRole := createSQLRole(sqlRole)
		Expect(errRole).ToNot(HaveOccurred())

		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				MasterRole: sqlRole,
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Roles.Owner).To(Equal(sqlRole))

		// Check owner role in DB
		checkRoleInSQLDb(postgresUser, sqlRole)

		// Check DB ownership
		isOwner, err := isRoleOwnerofSQLDB(pgdbDBName, sqlRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(isOwner).To(BeTrue())
	})

	It("should be ok to have a pgdb referencing an existing editor role", func() {
		// Create SQL role
		sqlRole := fmt.Sprintf("%s-writer", pgdbDBName) // -> This is default writer role name used by pgdb
		errRole := createSQLRole(sqlRole)
		Expect(errRole).ToNot(HaveOccurred())

		// Create pgec
		setupPGEC("10s", false)

		// Create pgdb
		item := setupPGDB(true)

		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Roles.Writer).To(Equal(sqlRole))

		// Check writer role in DB
		checkRoleInSQLDb(postgresUser, sqlRole)
	})

	It("should be ok to declare 1 schema", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Schemas: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbSchemaName1},
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(len(item.Status.Schemas)).To(Equal(1))
		Expect(item.Status.Schemas).To(ContainElement(pgdbSchemaName1))

		// Check schema exists in sql db
		exists, err := isSQLSchemaExists(pgdbSchemaName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("should be ok to declare 2 schema", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Schemas: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbSchemaName1, pgdbSchemaName2},
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(len(item.Status.Schemas)).To(Equal(2))
		Expect(item.Status.Schemas).To(ContainElements(pgdbSchemaName1, pgdbSchemaName2))

		// Check schema exist in sql db
		firstExists, firstErr := isSQLSchemaExists(pgdbSchemaName1)
		Expect(firstErr).ToNot(HaveOccurred())
		Expect(firstExists).To(BeTrue())

		secondExists, secondErr := isSQLSchemaExists(pgdbSchemaName2)
		Expect(secondErr).ToNot(HaveOccurred())
		Expect(secondExists).To(BeTrue())
	})

	It("should be ok to declare 1 schema and add another one after", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Schemas: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbSchemaName1},
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Check schema exist in sql db
		exists, err := isSQLSchemaExists(pgdbSchemaName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Add one more schema
		item.Spec.Schemas.List = append(item.Spec.Schemas.List, pgdbSchemaName2)

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if schemas has been updated in pgdb
				if !reflect.DeepEqual(updatedItem.Status.Schemas, []string{pgdbSchemaName1, pgdbSchemaName2}) {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(len(updatedItem.Status.Schemas)).To(Equal(2))
		Expect(updatedItem.Status.Schemas).To(ContainElements(pgdbSchemaName1, pgdbSchemaName2))

		// Check first schema still exists in sql db
		exists, err = isSQLSchemaExists(pgdbSchemaName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Check second schema exists in sql db
		exists, err = isSQLSchemaExists(pgdbSchemaName2)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("should be ok to remove a schema with drop on delete without cascade", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Schemas: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbSchemaName1},
					DropOnOnDelete:    true,
					DeleteWithCascade: false,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(len(item.Status.Schemas)).To(Equal(1))
		Expect(item.Status.Schemas).To(ContainElement(pgdbSchemaName1))

		// Schema should be in sql db
		exists, err := isSQLSchemaExists(pgdbSchemaName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Then remove schema from pgdb
		item.Spec.Schemas.List = make([]string, 0)

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if schemas has been removed in pgdb
				if len(updatedItem.Status.Schemas) > 0 {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Schemas).To(BeEmpty())

		// Schema should not be in sql db anymore
		stillExists, stillErr := isSQLSchemaExists(pgdbSchemaName1)
		Expect(stillErr).ToNot(HaveOccurred())
		Expect(stillExists).To(BeFalse())
	})

	It("should be ok to remove a schema with drop on delete with cascade", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Schemas: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbSchemaName1},
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(len(item.Status.Schemas)).To(Equal(1))
		Expect(item.Status.Schemas).To(ContainElement(pgdbSchemaName1))

		// Schema should be in sql db
		exists, err := isSQLSchemaExists(pgdbSchemaName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Add table to schema
		tableName := "tt"
		createTableInSchema(pgdbSchemaName1, tableName)

		// Then remove schema from pgdb
		item.Spec.Schemas.List = make([]string, 0)

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if schemas has been removed in pgdb
				if len(updatedItem.Status.Schemas) > 0 {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Schemas).To(BeEmpty())

		// Schema should not be in sql db anymore
		stillExists, stillErr := isSQLSchemaExists(pgdbSchemaName1)
		Expect(stillErr).ToNot(HaveOccurred())
		Expect(stillExists).To(BeFalse())
	})

	It("should be ok to declare 2 schema and remove one of the 2", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Schemas: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbSchemaName1, pgdbSchemaName2},
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Then remove last schema from pgdb
		item.Spec.Schemas.List = item.Spec.Schemas.List[:len(item.Spec.Schemas.List)-1]

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if schema has been removed in pgdb
				if !reflect.DeepEqual(updatedItem.Status.Schemas, []string{pgdbSchemaName1}) {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())

		// First schema should still be in sql db, second should be gone
		firstExists, firstErr := isSQLSchemaExists(pgdbSchemaName1)
		Expect(firstErr).ToNot(HaveOccurred())
		Expect(firstExists).To(BeTrue())

		secondExists, secondErr := isSQLSchemaExists(pgdbSchemaName2)
		Expect(secondErr).ToNot(HaveOccurred())
		Expect(secondExists).To(BeFalse())
	})

	It("should be ok to declare 1 extension", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Extensions: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbExtensionName1}, // Should be available (-> SELECT * FROM pg_available_extensions)
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(item.Status.Message).To(BeEmpty())
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(len(item.Status.Extensions)).To(Equal(1))
		Expect(item.Status.Extensions).To(ContainElement(pgdbExtensionName1))

		// Check extension exists in sql db
		exists, err := isSQLExtensionExists(pgdbExtensionName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("should be ok to declare 1 extension and add another one after", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Extensions: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbExtensionName1}, // Should be available (-> SELECT * FROM pg_available_extensions)
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		item.Spec.Extensions.List = append(item.Spec.Extensions.List, pgdbExtensionName2)

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if extensions have been updated in pgdb
				if !reflect.DeepEqual(updatedItem.Status.Extensions, []string{pgdbExtensionName1, pgdbExtensionName2}) {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(len(updatedItem.Status.Extensions)).To(Equal(2))
		Expect(updatedItem.Status.Extensions).To(ContainElements(pgdbExtensionName1, pgdbExtensionName2))

		// Check extensions exist in sql db
		firstExists, firstErr := isSQLExtensionExists(pgdbExtensionName1)
		Expect(firstErr).ToNot(HaveOccurred())
		Expect(firstExists).To(BeTrue())

		secondExists, secondErr := isSQLExtensionExists(pgdbExtensionName2)
		Expect(secondErr).ToNot(HaveOccurred())
		Expect(secondExists).To(BeTrue())

	})

	It("should be ok to declare 2 extensions", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Extensions: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbExtensionName1, pgdbExtensionName2}, // Should be available (-> SELECT * FROM pg_available_extensions)
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(item.Status.Message).To(BeEmpty())
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(len(item.Status.Extensions)).To(Equal(2))
		Expect(item.Status.Extensions).To(ContainElements(pgdbExtensionName1, pgdbExtensionName2))

		// Check extensions exist in sql db
		firstExists, firstErr := isSQLExtensionExists(pgdbExtensionName1)
		Expect(firstErr).ToNot(HaveOccurred())
		Expect(firstExists).To(BeTrue())

		secondExists, secondErr := isSQLExtensionExists(pgdbExtensionName2)
		Expect(secondErr).ToNot(HaveOccurred())
		Expect(secondExists).To(BeTrue())
	})

	It("should be ok to remove an extension with drop on delete without cascade", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Extensions: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbExtensionName1},
					DropOnOnDelete:    true,
					DeleteWithCascade: false,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Then remove extension from pgdb
		item.Spec.Extensions.List = make([]string, 0)

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		// Wait for update
		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if extensions has been updated in pgdb
				if len(updatedItem.Status.Extensions) > 0 {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Extensions).To(BeEmpty())

		// Check extension does not exist anymore in sql db
		exists, err := isSQLExtensionExists(pgdbExtensionName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should be ok to remove an extension with drop on delete with cascade", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Extensions: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbExtensionName1},
					DropOnOnDelete:    true,
					DeleteWithCascade: true,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Then remove extension from pgdb
		item.Spec.Extensions.List = make([]string, 0)

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		// Wait for update
		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if extensions has been updated in pgdb
				if len(updatedItem.Status.Extensions) > 0 {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Extensions).To(BeEmpty())

		// Check extensions does not exist anymore in sql db
		exists, err := isSQLExtensionExists(pgdbExtensionName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should be ok to declare 2 extensions and remove one of the 2", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				Extensions: postgresqlv1alpha1.DatabaseModulesList{
					List:              []string{pgdbExtensionName1, pgdbExtensionName2},
					DropOnOnDelete:    true,
					DeleteWithCascade: false,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Then remove one extension from pgdb
		item.Spec.Extensions.List = item.Spec.Extensions.List[:len(item.Spec.Extensions.List)-1]

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		// Wait for update
		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if extensions has been updated in pgdb
				if !reflect.DeepEqual(updatedItem.Status.Extensions, []string{pgdbExtensionName1}) {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Extensions).To(ContainElement(pgdbExtensionName1))

		// Check extensions exist in sql db
		exists, err := isSQLExtensionExists(pgdbExtensionName1)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("should be ok to set a master role directly", func() {
		// Given
		masterRole := "super-owner"

		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				MasterRole: masterRole,
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Database).To(Equal(pgdbDBName))
		Expect(item.Status.Roles.Owner).To(Equal(masterRole))

		// Check if roles exists
		ownerRoleExists, ownerRoleErr := isSQLRoleExists(masterRole)
		Expect(ownerRoleErr).ToNot(HaveOccurred())
		Expect(ownerRoleExists).To(BeTrue())

		// Check if default user has owner role in DB
		checkRoleInSQLDb(postgresUser, masterRole)

		// Check DB ownership
		isOwner, err := isRoleOwnerofSQLDB(pgdbDBName, masterRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(isOwner).To(BeTrue())
	})

	It("should be ok to inject a simple instance and set a master role after", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Update role for pgdb
		masterRole := "super-owner"
		item.Spec.MasterRole = masterRole
		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		// Wait for update
		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if extensions has been updated in pgdb
				if updatedItem.Status.Roles.Owner != masterRole {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Roles.Owner).To(Equal(masterRole))

		// Check if new roles exists
		exists, err := isSQLRoleExists(masterRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Check old role does not exist anymore
		ownerRole := fmt.Sprintf("%s-owner", pgdbDBName)
		exists, err = isSQLRoleExists(ownerRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())

		// Check if default user has owner role in DB
		checkRoleInSQLDb(postgresUser, masterRole)

		// Check DB ownership
		isOwner, err := isRoleOwnerofSQLDB(pgdbDBName, masterRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(isOwner).To(BeTrue())
	})

	It("should be ok to inject a simple instance with a master role and change it after", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				MasterRole: "master",
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Then change master role
		masterRoleBis := "master-bis"
		item.Spec.MasterRole = masterRoleBis

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		// Wait for update
		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if extensions has been updated in pgdb
				if updatedItem.Status.Roles.Owner != masterRoleBis {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Roles.Owner).To(Equal(masterRoleBis))

		// Check if roles exists
		ownerRoleExists, ownerRoleErr := isSQLRoleExists(masterRoleBis)
		Expect(ownerRoleErr).ToNot(HaveOccurred())
		Expect(ownerRoleExists).To(BeTrue())

		// Check if default user has owner role in DB
		checkRoleInSQLDb(postgresUser, masterRoleBis)

		// Check DB ownership
		isOwner, err := isRoleOwnerofSQLDB(pgdbDBName, masterRoleBis)
		Expect(err).ToNot(HaveOccurred())
		Expect(isOwner).To(BeTrue())
	})

	It("should be ok to rename database", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		// Create pgdb
		it := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName + "-old",
				EngineConfiguration: &common.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlDatabase{}
		// Get updated pgdb
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, item)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if item.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Database).To(Equal(pgdbDBName + "-old"))

		// Change database name
		item.Spec.Database = pgdbDBName
		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		// Get updated pgdb
		updatedItem := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedItem.Status.Database != pgdbDBName {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(updatedItem.Status.Database).To(Equal(pgdbDBName))

		// Check if DB exists
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Check old one DB does not exists
		oldExists, oldErr := isSQLDBExists(pgdbDBName + "-old")
		Expect(oldErr).ToNot(HaveOccurred())
		Expect(oldExists).To(BeFalse())

	})

	It("should be ok to delete it with wait and nothing linked", func() {
		// Create pgec
		setupPGEC("10s", false)

		// Create pgdb
		item := setupPGDB(true)

		// Then delete pgdb
		Expect(k8sClient.Delete(ctx, item)).Should(Succeed())

		pgdb := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, pgdb)

				if err == nil {
					return errors.New("should be deleted but not deleted")
				}

				// Check if error isn't a not found error
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check DB does not exists anymore
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should be ok to delete it without wait and nothing linked", func() {
		// Create pgec
		setupPGEC("10s", false)

		// Create pgdb
		item := setupPGDB(false)

		// Then delete it
		Expect(k8sClient.Delete(ctx, item)).Should(Succeed())

		pgdb := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, pgdb)

				if err == nil {
					return errors.New("should be deleted but not deleted")
				}

				// Check if error isn't a not found error
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})

	It("should fail to delete it with wait and something linked (postgresql user)", func() {
		// Create pgec
		setupPGEC("10s", false)

		// Create pgdb
		it := setupPGDB(true)

		// Create user
		setupPGU()

		// Try to delete pgdb
		Expect(k8sClient.Delete(ctx, it)).Should(Succeed())

		pgdb := &postgresqlv1alpha1.PostgresqlDatabase{}
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

				// Check if status is no more ready
				if pgdb.Status.Phase != postgresqlv1alpha1.DatabaseFailedPhase {
					return gerrors.New("pgdb should not be valid anymore")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check that deletion is blocked
		Expect(pgdb.Status.Ready).To(BeFalse())
		Expect(pgdb.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseFailedPhase))
		Expect(pgdb.Status.Message).To(Equal(
			fmt.Sprintf("cannot remove resource because found user %s in namespace %s linked to this resource and wait for deletion flag is enabled", pguName, pguNamespace)))

		// Check DB has not been deleted
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("should be ok to delete it without wait and something linked (postgresql user)", func() {
		// Create pgec
		setupPGEC("10s", false)

		// Create pgdb
		it := setupPGDB(false)

		// Create user
		setupPGU()

		// Try to delete pgdb
		Expect(k8sClient.Delete(ctx, it)).Should(Succeed())

		pgdb := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, pgdb)

				if err == nil {
					return errors.New("should be deleted but not deleted")
				}

				// Check if error isn't a not found error
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check DB does not exists anymore
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should fail to delete it with wait and something linked (postgresql user role)", func() {
		// Create pgec
		setupPGEC("10s", false)

		// Create pgdb
		it := setupPGDB(true)

		// Create user
		setupManagedPGUR("")

		// Try to delete pgdb
		Expect(k8sClient.Delete(ctx, it)).Should(Succeed())

		pgdb := &postgresqlv1alpha1.PostgresqlDatabase{}
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

				// Check if status is no more ready
				if pgdb.Status.Phase != postgresqlv1alpha1.DatabaseFailedPhase {
					return gerrors.New("pgdb should not be valid anymore")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check that deletion is blocked
		Expect(pgdb.Status.Ready).To(BeFalse())
		Expect(pgdb.Status.Phase).To(Equal(postgresqlv1alpha1.DatabaseFailedPhase))
		Expect(pgdb.Status.Message).To(Equal(
			fmt.Sprintf("cannot remove resource because found user role %s in namespace %s linked to this resource and wait for deletion flag is enabled", pgurName, pgurNamespace)))

		// Check DB has not been deleted
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("should be ok to delete it without wait and something linked (postgresql user role)", func() {
		// Create pgec
		setupPGEC("10s", false)

		// Create pgdb
		it := setupPGDB(false)

		// Create user
		setupManagedPGUR("")

		// Try to delete pgdb
		Expect(k8sClient.Delete(ctx, it)).Should(Succeed())

		pgdb := &postgresqlv1alpha1.PostgresqlDatabase{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, pgdb)

				if err == nil {
					return errors.New("should be deleted but not deleted")
				}

				// Check if error isn't a not found error
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check DB does not exists anymore
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})
})
