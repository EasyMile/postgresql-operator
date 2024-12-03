package postgresql

import (
	"errors"
	gerrors "errors"
	"fmt"
	"time"

	"github.com/easymile/postgresql-operator/api/postgresql/common"
	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PostgresqlPublication tests", func() {
	AfterEach(cleanupFunction)

	Describe("Spec error", func() {
		It("shouldn't accept input without any specs", func() {
			err := k8sClient.Create(ctx, &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
			})

			Expect(err).To(HaveOccurred())

			// Cast error
			stErr, ok := err.(*apimachineryErrors.StatusError)

			Expect(ok).To(BeTrue())

			// Check that content is correct
			causes := stErr.Status().Details.Causes

			Expect(causes).To(HaveLen(1))

			// Search all fields
			fields := map[string]bool{
				"spec.database": false,
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

		It("should fail when nothing is provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("name must have a value"))
		})

		It("should fail when no publication option is provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
					Name: pgpublicationPublicationName1,
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("nothing is selected for publication (no all tables, no tables in schema, no tables)"))
		})

		It("should fail when all tables and tables in schema are provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
					Name:           pgpublicationPublicationName1,
					AllTables:      true,
					TablesInSchema: []string{"fake"},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("all tables cannot be set with tables in schema or tables"))
		})

		It("should fail when all tables and tables are provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
					Name:      pgpublicationPublicationName1,
					AllTables: true,
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{
							TableName: "fake",
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("all tables cannot be set with tables in schema or tables"))
		})

		It("should fail when tables in schema with a empty string is provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
					Name:           pgpublicationPublicationName1,
					TablesInSchema: []string{""},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("tables in schema cannot have empty schema listed"))
		})

		It("should fail when tables with a empty string as table name is provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
					Name: pgpublicationPublicationName1,
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{
							TableName: "",
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("tables cannot have a columns list with an empty name or have a columns list with a table schema list enabled or an empty additional where"))
		})

		It("should fail when tables with a empty string in columns is provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
					Name: pgpublicationPublicationName1,
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{
							TableName: "fake",
							Columns:   &[]string{""},
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("tables cannot have a columns list with an empty name or have a columns list with a table schema list enabled or an empty additional where"))
		})

		It("should fail when tables with a empty string in additional where is provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
					Name: pgpublicationPublicationName1,
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{
							TableName:       "fake",
							AdditionalWhere: starAny(""),
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("tables cannot have a columns list with an empty name or have a columns list with a table schema list enabled or an empty additional where"))
		})

		It("should fail when tables with columns and tables in schema are provided", func() {
			it := &postgresqlv1alpha1.PostgresqlPublication{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgpublicationName,
					Namespace: pgpublicationNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlPublicationSpec{
					Database: &common.CRLink{
						Name:      pgdbName,
						Namespace: pgdbNamespace,
					},
					Name:           pgpublicationPublicationName1,
					TablesInSchema: []string{"fake1"},
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{
							TableName: "fake2",
							Columns:   &[]string{"id"},
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlPublication{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgpublicationName,
						Namespace: pgpublicationNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.PublicationNoPhase {
						return errors.New("pgpub hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal("tables cannot have a columns list with an empty name or have a columns list with a table schema list enabled or an empty additional where"))
		})
	})

	Describe("Creation", func() {
		Describe("For all tables", func() {
			It("should be ok without any tables", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{AllTables: true})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(true)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          true,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(0))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with tables", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{AllTables: true})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(true)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          true,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with pg with options", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					AllTables: true,
					WithParameters: &postgresqlv1alpha1.PostgresqlPublicationWith{
						Publish: "truncate",
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(true)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          true,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with pg with options and via partition root", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					AllTables: true,
					WithParameters: &postgresqlv1alpha1.PostgresqlPublicationWith{
						Publish:                 "truncate",
						PublishViaPartitionRoot: starAny(true),
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(true)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          true,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: true,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with a custom replication slot name", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					AllTables:           true,
					ReplicationSlotName: pgpublicationCustomReplicationSlotName,
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(true)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationCustomReplicationSlotName))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationCustomReplicationSlotName))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          true,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})

		Describe("For tables in schema", func() {
			It("should be ok without any tables", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{TablesInSchema: []string{"public"}})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(0))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with tables", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{TablesInSchema: []string{"public"}})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with pg with options", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					TablesInSchema: []string{"public"},
					WithParameters: &postgresqlv1alpha1.PostgresqlPublicationWith{
						Publish: "truncate",
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with pg with options and via partition root", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					TablesInSchema: []string{"public"},
					WithParameters: &postgresqlv1alpha1.PostgresqlPublicationWith{
						Publish:                 "truncate",
						PublishViaPartitionRoot: starAny(true),
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: true,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with custom replication slot name", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					TablesInSchema:      []string{"public"},
					ReplicationSlotName: pgpublicationCustomReplicationSlotName,
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationCustomReplicationSlotName))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationCustomReplicationSlotName))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})

		Describe("For specific tables", func() {
			It("should fail without any tables", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake"},
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeFalse())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
				Expect(item.Status.Message).To(Equal(`pq: relation "fake" does not exist`))
				Expect(item.Status.AllTables).To(BeNil())
				Expect(item.Status.Hash).To(Equal(""))
				Expect(item.Status.Name).To(Equal(""))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(BeNil())
				}
			})

			It("should be ok with tables with all columns", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake"},
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with tables selected columns", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id", "nb2"}},
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb2"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with additional where and all columns", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", AdditionalWhere: starAny(`'id' = 'value'`)},
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: starAny(`('id'::text = 'value'::text)`),
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with additional where and specific columns", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id", "nb2"}, AdditionalWhere: starAny(`'id' = 'value'`)},
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb2"},
								AdditionalWhere: starAny(`('id'::text = 'value'::text)`),
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with pg with options", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake"},
					},
					WithParameters: &postgresqlv1alpha1.PostgresqlPublicationWith{
						Publish: "truncate",
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with pg with options and via partition root", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake"},
					},
					WithParameters: &postgresqlv1alpha1.PostgresqlPublicationWith{
						Publish:                 "truncate",
						PublishViaPartitionRoot: starAny(true),
					},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: true,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok with pg with a custom replication slot name", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake"},
					},
					ReplicationSlotName: pgpublicationCustomReplicationSlotName,
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(item.Status.Message).To(Equal(""))
				Expect(item.Status.AllTables).To(Equal(starAny(false)))
				Expect(item.Status.Hash).NotTo(Equal(""))
				Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
				Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationCustomReplicationSlotName))
				Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
				Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationCustomReplicationSlotName))
				Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})
	})

	Describe("Update", func() {
		Describe("For all tables", func() {
			It("should fail to change for a table schema list", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					AllTables: true,
				})

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.AllTables = false
				item.Spec.TablesInSchema = []string{"public"}
				// Update
				err := k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Phase == item.Status.Phase {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeFalse())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
				Expect(updatedItem.Status.Message).To(Equal(`cannot change all tables flag on an upgrade`))
				Expect(*updatedItem.Status.AllTables).To(BeTrue())
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Name).To(Equal(item.Status.Name))
			})

			It("should fail to change for a table specific", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					AllTables: true,
				})

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.AllTables = false
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake"},
				}
				// Update
				err := k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Phase == item.Status.Phase {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeFalse())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
				Expect(updatedItem.Status.Message).To(Equal(`cannot change all tables flag on an upgrade`))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(true)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Name).To(Equal(item.Status.Name))
			})

			It("should be ok to change pg with option", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{AllTables: true})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.WithParameters = &postgresqlv1alpha1.PostgresqlPublicationWith{
					Publish: "truncate",
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(true)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          true,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to rename", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					AllTables: true,
				})

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Save
				hash := item.Status.Hash
				// Build new name
				oldName := item.Spec.Name
				newName := oldName + "rename"
				// Update
				item.Spec.Name = newName
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(*updatedItem.Status.AllTables).To(BeTrue())
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Name).NotTo(Equal(item.Status.Name))
				Expect(updatedItem.Status.Name).To(Equal(newName))

				oldData, err := getPublication(oldName)
				Expect(err).NotTo(HaveOccurred())
				Expect(oldData).To(BeNil())

				data, err := getPublication(newName)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          true,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(newName)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})

		Describe("For tables in schema", func() {
			It("should fail to change to a for all tables", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{TablesInSchema: []string{"public"}})

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.AllTables = true
				item.Spec.TablesInSchema = []string{}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Phase == item.Status.Phase {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())
				// Checks
				Expect(updatedItem.Status.Ready).To(BeFalse())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
				Expect(updatedItem.Status.Message).To(Equal(`cannot change all tables flag on an upgrade`))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))
			})

			It("should be ok to change for a table specific with specific columns", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					TablesInSchema: []string{"public"},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.TablesInSchema = []string{}
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id", "nb2"}},
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb2"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change pg with option", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					TablesInSchema: []string{"public"},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.WithParameters = &postgresqlv1alpha1.PostgresqlPublicationWith{
					Publish: "truncate",
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to rename", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					TablesInSchema: []string{"public"},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Build new name
				oldName := item.Spec.Name
				newName := oldName + "rename"
				// Update
				item.Spec.Name = newName
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).NotTo(Equal(item.Status.Name))
				Expect(updatedItem.Status.Name).To(Equal(newName))

				oldData, err := getPublication(oldName)
				Expect(err).NotTo(HaveOccurred())
				Expect(oldData).To(BeNil())

				data, err := getPublication(newName)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(updatedItem.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})

		Describe("For specific tables", func() {
			It("should fail to change to a for all tables", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{{TableName: "fake"}},
				})

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.AllTables = true
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Phase == item.Status.Phase {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())
				// Checks
				Expect(updatedItem.Status.Ready).To(BeFalse())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
				Expect(updatedItem.Status.Message).To(Equal(`cannot change all tables flag on an upgrade`))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))
			})

			It("should be ok to change to a schema list", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{{TableName: "fake", Columns: &[]string{"id", "nb2"}}},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.TablesInSchema = []string{"public"}
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change remove table", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id", "nb2"}},
						{TableName: "fake2", Columns: &[]string{"id", "test"}},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id", "nb2"}},
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb2"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change add table", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id", "nb2"}},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id", "nb2"}},
					{TableName: "fake2", Columns: &[]string{"id", "test"}},
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(2))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb2"},
								AdditionalWhere: nil,
							},
							{
								SchemaName:      "public",
								TableName:       "fake2",
								Columns:         []string{"id", "test"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change remove columns", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id", "nb2"}},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id"}},
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change add columns", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id"}},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id", "nb"}},
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id", "nb"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change add additional where", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id"}},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id"}, AdditionalWhere: starAny("'id' = 'value'")},
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id"},
								AdditionalWhere: starAny(`('id'::text = 'value'::text)`),
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change remove additional where", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id"}, AdditionalWhere: starAny("'id' = 'value'")},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id"}},
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change change additional where", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id"}, AdditionalWhere: starAny("'id' = 'value2'")},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.Tables = []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id"}, AdditionalWhere: starAny("'id' = 'value'")},
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id"},
								AdditionalWhere: starAny(`('id'::text = 'value'::text)`),
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to change pg with option", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id"}},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Update
				item.Spec.WithParameters = &postgresqlv1alpha1.PostgresqlPublicationWith{
					Publish: "truncate",
				}
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).To(Equal(pgpublicationPublicationName1))

				data, err := getPublication(item.Status.Name)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             false,
						Update:             false,
						Delete:             false,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(item.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})

			It("should be ok to rename", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
						{TableName: "fake", Columns: &[]string{"id"}},
					},
				})

				// Save hash
				hash := item.Status.Hash

				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Build new name
				oldName := item.Spec.Name
				newName := oldName + "rename"
				// Update
				item.Spec.Name = newName
				// Update
				err = k8sClient.Update(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				updatedItem := &postgresqlv1alpha1.PostgresqlPublication{}

				Eventually(
					func() error {
						err := k8sClient.Get(ctx, types.NamespacedName{
							Name:      item.Name,
							Namespace: item.Namespace,
						}, updatedItem)
						// Check error
						if err != nil {
							return err
						}

						// Check if status hasn't been updated
						if updatedItem.Status.Hash == hash {
							return gerrors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				// Checks
				Expect(updatedItem.Status.Ready).To(BeTrue())
				Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
				Expect(updatedItem.Status.Message).To(Equal(""))
				Expect(updatedItem.Status.AllTables).To(Equal(starAny(false)))
				Expect(updatedItem.Status.Hash).NotTo(Equal(""))
				Expect(updatedItem.Status.Hash).NotTo(Equal(item.Status.Hash))
				Expect(updatedItem.Status.Name).NotTo(Equal(item.Status.Name))
				Expect(updatedItem.Status.Name).To(Equal(newName))

				oldData, err := getPublication(oldName)
				Expect(err).NotTo(HaveOccurred())
				Expect(oldData).To(BeNil())

				data, err := getPublication(newName)

				if Expect(err).NotTo(HaveOccurred()) {
					// Assert
					Expect(data).To(Equal(&PublicationResult{
						AllTables:          false,
						Insert:             true,
						Update:             true,
						Delete:             true,
						Truncate:           true,
						PublicationViaRoot: false,
					}))

					// Get details
					details, err := getPublicationTableDetails(updatedItem.Status.Name)
					if Expect(err).NotTo(HaveOccurred()) {
						Expect(details).To(HaveLen(1))
						Expect(details).To(Equal([]*PublicationTableDetail{
							{
								SchemaName:      "public",
								TableName:       "fake",
								Columns:         []string{"id"},
								AdditionalWhere: nil,
							},
						}))
					}
				}

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})
	})

	Describe("Deletion", func() {
		Describe("For all tables", func() {
			It("should be ok to delete a publication with drop on delete", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					AllTables:    true,
					DropOnDelete: true,
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Delete object
				err := k8sClient.Delete(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				Eventually(
					func() error {
						data, err := getPublication(item.Status.Name)
						if err != nil {
							return err
						}

						if data != nil {
							return errors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				data, err := getPublication(item.Status.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(BeNil())
			})

			It("should be ok to ignore a publication without drop on delete", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					AllTables:    true,
					DropOnDelete: false,
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Delete object
				err := k8sClient.Delete(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				// Here as we cannot ensure that this have been ignored by operator programmatically, just sleep
				time.Sleep(time.Second)

				data, err := getPublication(item.Status.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(&PublicationResult{
					AllTables:          true,
					Insert:             true,
					Update:             true,
					Delete:             true,
					Truncate:           true,
					PublicationViaRoot: false,
				}))

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})

		Describe("For tables in schema", func() {
			It("should be ok to delete a publication with drop on delete", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)
				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					DropOnDelete:   true,
					TablesInSchema: []string{"public"},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Delete object
				err = k8sClient.Delete(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				Eventually(
					func() error {
						data, err := getPublication(item.Status.Name)
						if err != nil {
							return err
						}

						if data != nil {
							return errors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				data, err := getPublication(item.Status.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(BeNil())
			})

			It("should be ok to ignore a publication without drop on delete", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)
				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					DropOnDelete:   false,
					TablesInSchema: []string{"public"},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Delete object
				err = k8sClient.Delete(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				// Here as we cannot ensure that this have been ignored by operator programmatically, just sleep
				time.Sleep(time.Second)

				data, err := getPublication(item.Status.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(&PublicationResult{
					AllTables:          false,
					Insert:             true,
					Update:             true,
					Delete:             true,
					Truncate:           true,
					PublicationViaRoot: false,
				}))

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})

		Describe("For specific tables", func() {
			It("should be ok to delete a publication with drop on delete", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)
				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					DropOnDelete: true,
					Tables:       []*postgresqlv1alpha1.PostgresqlPublicationTable{{TableName: "fake"}},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Delete object
				err = k8sClient.Delete(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				Eventually(
					func() error {
						data, err := getPublication(item.Status.Name)
						if err != nil {
							return err
						}

						if data != nil {
							return errors.New("hasn't been updated by operator")
						}

						return nil
					},
					generalEventuallyTimeout,
					generalEventuallyInterval,
				).
					Should(Succeed())

				data, err := getPublication(item.Status.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(BeNil())
			})

			It("should be ok to ignore a publication without drop on delete", func() {
				// Setup pgec
				setupPGEC("30s", false)
				// Create pgdb
				setupPGDB(false)
				// Create tables
				err := create2KnownTablesWithColumnsInPublicSchema()
				Expect(err).NotTo(HaveOccurred())

				// Setup a pg publication
				item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
					DropOnDelete: false,
					Tables:       []*postgresqlv1alpha1.PostgresqlPublicationTable{{TableName: "fake"}},
				})

				// Checks
				Expect(item.Status.Ready).To(BeTrue())
				Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))

				// Delete object
				err = k8sClient.Delete(ctx, item)
				Expect(err).NotTo(HaveOccurred())

				// Here as we cannot ensure that this have been ignored by operator programmatically, just sleep
				time.Sleep(time.Second)

				data, err := getPublication(item.Status.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(Equal(&PublicationResult{
					AllTables:          false,
					Insert:             true,
					Update:             true,
					Delete:             true,
					Truncate:           true,
					PublicationViaRoot: false,
				}))

				data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(data2).To(Equal(&replicationSlotResult{
						SlotName: item.Status.ReplicationSlotName,
						Plugin:   DefaultReplicationSlotPlugin,
						Database: pgdbDBName,
					}))
				}
			})
		})
	})

	Describe("Reconcile", func() {
		It("should be ok to reconcile an existing table schema list publication to a table list", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create tables
			err := create2KnownTablesWithColumnsInPublicSchema()
			Expect(err).NotTo(HaveOccurred())

			// Create publication
			err = rawSQLQuery("CREATE PUBLICATION " + pgpublicationPublicationName1 + " FOR TABLES IN SCHEMA public")
			Expect(err).NotTo(HaveOccurred())

			// Setup a pg publication
			item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
				Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id"}},
				},
			})

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.AllTables).To(Equal(starAny(false)))
			Expect(item.Status.Hash).NotTo(Equal(""))
			Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
			Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
			Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
			Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
			Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

			data, err := getPublication(item.Status.Name)

			if Expect(err).NotTo(HaveOccurred()) {
				// Assert
				Expect(data).To(Equal(&PublicationResult{
					AllTables:          false,
					Insert:             true,
					Update:             true,
					Delete:             true,
					Truncate:           true,
					PublicationViaRoot: false,
				}))

				// Get details
				details, err := getPublicationTableDetails(item.Status.Name)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(details).To(HaveLen(1))
					Expect(details).To(Equal([]*PublicationTableDetail{
						{
							SchemaName:      "public",
							TableName:       "fake",
							Columns:         []string{"id"},
							AdditionalWhere: nil,
						},
					}))
				}
			}

			data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
			if Expect(err).NotTo(HaveOccurred()) {
				Expect(data2).To(Equal(&replicationSlotResult{
					SlotName: item.Status.ReplicationSlotName,
					Plugin:   DefaultReplicationSlotPlugin,
					Database: pgdbDBName,
				}))
			}
		})

		It("should be ok to reconcile an existing table list publication to a tables in schema list", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create tables
			err := create2KnownTablesWithColumnsInPublicSchema()
			Expect(err).NotTo(HaveOccurred())

			// Create publication
			err = rawSQLQuery("CREATE PUBLICATION " + pgpublicationPublicationName1 + " FOR TABLE fake (id)")
			Expect(err).NotTo(HaveOccurred())

			// Setup a pg publication
			item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
				TablesInSchema: []string{"public"},
			})

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.AllTables).To(Equal(starAny(false)))
			Expect(item.Status.Hash).NotTo(Equal(""))
			Expect(item.Status.Name).To(Equal(pgpublicationPublicationName1))
			Expect(item.Status.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
			Expect(item.Status.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))
			Expect(item.Spec.ReplicationSlotName).To(Equal(pgpublicationPublicationName1))
			Expect(item.Spec.ReplicationSlotPlugin).To(Equal(DefaultReplicationSlotPlugin))

			data, err := getPublication(item.Status.Name)

			if Expect(err).NotTo(HaveOccurred()) {
				// Assert
				Expect(data).To(Equal(&PublicationResult{
					AllTables:          false,
					Insert:             true,
					Update:             true,
					Delete:             true,
					Truncate:           true,
					PublicationViaRoot: false,
				}))

				// Get details
				details, err := getPublicationTableDetails(item.Status.Name)
				if Expect(err).NotTo(HaveOccurred()) {
					Expect(details).To(HaveLen(2))
					Expect(details).To(Equal([]*PublicationTableDetail{
						{
							SchemaName:      "public",
							TableName:       "fake",
							Columns:         []string{"id", "nb", "nb2"},
							AdditionalWhere: nil,
						},
						{
							SchemaName:      "public",
							TableName:       "fake2",
							Columns:         []string{"id", "test"},
							AdditionalWhere: nil,
						},
					}))
				}
			}

			data2, err := getReplicationSlot(item.Status.ReplicationSlotName)
			if Expect(err).NotTo(HaveOccurred()) {
				Expect(data2).To(Equal(&replicationSlotResult{
					SlotName: item.Status.ReplicationSlotName,
					Plugin:   DefaultReplicationSlotPlugin,
					Database: pgdbDBName,
				}))
			}
		})

		It("should fail to reconcile an existing for all tables publication to a table list", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create tables
			err := create2KnownTablesWithColumnsInPublicSchema()
			Expect(err).NotTo(HaveOccurred())

			// Create publication
			err = rawSQLQuery("CREATE PUBLICATION " + pgpublicationPublicationName1 + " FOR ALL TABLES")
			Expect(err).NotTo(HaveOccurred())

			// Setup a pg publication
			item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
				Tables: []*postgresqlv1alpha1.PostgresqlPublicationTable{
					{TableName: "fake", Columns: &[]string{"id"}},
				},
			})

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal(`publication in database and spec are out of sync for 'for all tables' and values must be aligned to continue`))
			Expect(item.Status.AllTables).To(BeNil())
			Expect(item.Status.Hash).To(Equal(""))
			Expect(item.Status.Name).To(Equal(""))

			data, err := getPublication(pgpublicationPublicationName1)

			if Expect(err).NotTo(HaveOccurred()) {
				// Assert
				Expect(data).To(Equal(&PublicationResult{
					AllTables:          true,
					Insert:             true,
					Update:             true,
					Delete:             true,
					Truncate:           true,
					PublicationViaRoot: false,
				}))
			}
		})

		It("should fail to reconcile an existing for all tables publication to a tables in schema list", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create tables
			err := create2KnownTablesWithColumnsInPublicSchema()
			Expect(err).NotTo(HaveOccurred())

			// Create publication
			err = rawSQLQuery("CREATE PUBLICATION " + pgpublicationPublicationName1 + " FOR ALL TABLES")
			Expect(err).NotTo(HaveOccurred())

			// Setup a pg publication
			item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{
				TablesInSchema: []string{"public"},
			})

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal(`publication in database and spec are out of sync for 'for all tables' and values must be aligned to continue`))
			Expect(item.Status.AllTables).To(BeNil())
			Expect(item.Status.Hash).To(Equal(""))
			Expect(item.Status.Name).To(Equal(""))

			data, err := getPublication(pgpublicationPublicationName1)

			if Expect(err).NotTo(HaveOccurred()) {
				// Assert
				Expect(data).To(Equal(&PublicationResult{
					AllTables:          true,
					Insert:             true,
					Update:             true,
					Delete:             true,
					Truncate:           true,
					PublicationViaRoot: false,
				}))
			}
		})

		It("should fail to reconcile with an existing replication slot for another database", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create replication slot
			createReplicationSlotInMainDB(pgpublicationPublicationName1, DefaultReplicationSlotPlugin)

			// Create tables
			err := create2KnownTablesWithColumnsInPublicSchema()
			Expect(err).NotTo(HaveOccurred())

			// Setup a pg publication
			item := setupPGPublicationWithPartialSpec(postgresqlv1alpha1.PostgresqlPublicationSpec{AllTables: true})

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.PublicationFailedPhase))
			Expect(item.Status.Message).To(Equal(`replication slot with the same name already exists for another database`))
			Expect(item.Status.AllTables).To(BeNil())
			Expect(item.Status.Hash).To(Equal(""))
			Expect(item.Status.Name).To(Equal(""))
			Expect(item.Status.ReplicationSlotName).To(Equal(""))
			Expect(item.Status.ReplicationSlotPlugin).To(Equal(""))
		})
	})
})
