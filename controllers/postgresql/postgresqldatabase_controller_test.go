package postgresql

import (
	"errors"
	"fmt"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/apis/postgresql/v1alpha1"
	. "github.com/onsi/ginkgo"
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
				EngineConfiguration: &postgresqlv1alpha1.CRLink{
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
		Expect(item.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineFailedPhase))
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
				EngineConfiguration: &postgresqlv1alpha1.CRLink{
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
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Database).To(BeEquivalentTo(pgdbDBName))
		Expect(item.Status.Message).To(BeEquivalentTo(""))
		Expect(item.Status.Roles.Owner).To(BeEquivalentTo(fmt.Sprintf("%s-owner", pgdbDBName)))
		Expect(item.Status.Roles.Reader).To(BeEquivalentTo(fmt.Sprintf("%s-reader", pgdbDBName)))
		Expect(item.Status.Roles.Writer).To(BeEquivalentTo(fmt.Sprintf("%s-writer", pgdbDBName)))

		// Check if DB exists
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Check if roles exists
		ownerRoleExists, ownerRoleErr := isSQLRoleExists(fmt.Sprintf("%s-owner", pgdbDBName))
		Expect(ownerRoleErr).ToNot(HaveOccurred())
		Expect(ownerRoleExists).To(BeTrue())

		readerRoleExists, readerRoleErr := isSQLRoleExists(fmt.Sprintf("%s-reader", pgdbDBName))
		Expect(readerRoleErr).ToNot(HaveOccurred())
		Expect(readerRoleExists).To(BeTrue())

		writerRoleExists, writerRoleErr := isSQLRoleExists(fmt.Sprintf("%s-writer", pgdbDBName))
		Expect(writerRoleErr).ToNot(HaveOccurred())
		Expect(writerRoleExists).To(BeTrue())
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
				EngineConfiguration: &postgresqlv1alpha1.CRLink{
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
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.DatabaseCreatedPhase))
		Expect(item.Status.Database).To(BeEquivalentTo(pgdbDBName))
		Expect(item.Status.Message).To(BeEquivalentTo(""))
		Expect(item.Status.Roles.Owner).To(BeEquivalentTo("master"))
		Expect(item.Status.Roles.Reader).To(BeEquivalentTo(fmt.Sprintf("%s-reader", pgdbDBName)))
		Expect(item.Status.Roles.Writer).To(BeEquivalentTo(fmt.Sprintf("%s-writer", pgdbDBName)))

		// Check if DB exists
		exists, err := isSQLDBExists(pgdbDBName)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Check if roles exists
		ownerRoleExists, ownerRoleErr := isSQLRoleExists("master")
		Expect(ownerRoleErr).ToNot(HaveOccurred())
		Expect(ownerRoleExists).To(BeTrue())

		readerRoleExists, readerRoleErr := isSQLRoleExists(fmt.Sprintf("%s-reader", pgdbDBName))
		Expect(readerRoleErr).ToNot(HaveOccurred())
		Expect(readerRoleExists).To(BeTrue())

		writerRoleExists, writerRoleErr := isSQLRoleExists(fmt.Sprintf("%s-writer", pgdbDBName))
		Expect(writerRoleErr).ToNot(HaveOccurred())
		Expect(writerRoleExists).To(BeTrue())
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
				EngineConfiguration: &postgresqlv1alpha1.CRLink{
					Name:      prov.Name,
					Namespace: prov.Namespace,
				},
				MasterRole:                  "master",
				DropOnDelete:                true,
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
		Expect(k8sClient.Delete(ctx, it)).Should(Succeed())

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
		).ShouldNot(Succeed())

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
				EngineConfiguration: &postgresqlv1alpha1.CRLink{
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
		Expect(k8sClient.Delete(ctx, it)).Should(Succeed())

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
		).ShouldNot(Succeed())

		// Check DB does not exists anymore
		stillExists, stillErr := isSQLDBExists(pgdbDBName)
		Expect(stillErr).ToNot(HaveOccurred())
		Expect(stillExists).To(BeTrue())
	})

	It("should be ok to declare 1 schema", func() {
		// TODO: Create pgdb with a schema
	})

	It("should be ok to declare 2 schema", func() {
		// TODO: Create pgdb with 2 schema
	})

	It("should be ok to declare 1 schema and add another one after", func() {
		// TODO
	})

	It("should be ok to remove a schema with drop on delete without cascade", func() {
		// TODO: Create pgdb with a schema with drop on delete without cascade, update with by removing it and check
	})

	It("should be ok to remove a schema with drop on delete with cascade", func() {
		// TODO: Create pgdb with a schema with drop on delete without cascade, update with by removing it and check
	})

	It("should be ok to declare 2 schema and remove one of the 2", func() {
		// TODO
	})

	It("should be ok to declare 1 extension", func() {
		// TODO: Create pgdb with an extension
	})

	It("should be ok to declare 1 extension and add another one after", func() {
		// TODO
	})

	It("should be ok to declare 2 extensions", func() {
		// TODO: Create pgdb with 2 extensions
	})

	It("should be ok to remove an extension with drop on delete without cascade", func() {
		// TODO: Create pgdb with an extension with drop on delete without cascade, update with by removing it and check
	})

	It("should be ok to remove an extension with drop on delete with cascade", func() {
		// TODO: Create pgdb with an extension with drop on delete without cascade, update with by removing it and check
	})

	It("should be ok to declare 2 extensions and remove one of the 2", func() {
		// TODO
	})

	It("should be ok to set a master role directly", func() {
		// TODO
	})

	It("should be ok to inject a simple instance and set a master role after", func() {
		// TODO
	})

	It("should be ok to inject a simple instance with a master role and change it after", func() {
		// TODO
	})

	It("should be ok to rename database", func() {
		// TODO: create a pgdb, update it with another db name
	})

	It("should be ok to delete it with wait and nothing linked", func() {
		// TODO: Create a pgdb with wait, delete pgdb => shouldn't be blocked
	})

	It("should be ok to delete it without wait and nothing linked", func() {
		// TODO: Create a pgdb without wait, delete pgdb => shouldn't be blocked
	})

	It("should be ok to delete it with wait and something linked", func() {
		// TODO: Create a pgdb with wait, create pgu, delete pgdb => should be blocked
	})

	It("should be ok to delete it without wait and something linked", func() {
		// TODO: Create a pgdb without wait, create pgu, delete pgdb => shouldn't be blocked
	})
})
