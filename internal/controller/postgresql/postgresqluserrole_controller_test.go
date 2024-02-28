package postgresql

import (
	"errors"
	"fmt"
	"time"

	"github.com/easymile/postgresql-operator/api/postgresql/common"
	"github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PostgresqlUserRole tests", func() {
	AfterEach(cleanupFunction)

	It("shouldn't accept input without any specs", func() {
		err := k8sClient.Create(ctx, &postgresqlv1alpha1.PostgresqlUserRole{
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
			"spec.privileges": false,
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

	Describe("Provided mode", func() {
		It("should fail when import secret isn't provided", func() {
			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode: postgresqlv1alpha1.ProvidedMode,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("PostgresqlUserRole is in provided mode without any ImportSecretName"))
		})

		It("should fail when privileges contains 2 times the same db (twice fully declared)", func() {
			setupPGURImportSecret()

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:             postgresqlv1alpha1.ProvidedMode,
					ImportSecretName: pgurImportSecretName,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
						{
							Privilege:           postgresqlv1alpha1.WriterPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Privilege list mustn't have the same database listed multiple times"))
		})

		It("should fail when privileges contains 2 times the same db (1 fully declared, 1 without namespace)", func() {
			setupPGURImportSecret()

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:             postgresqlv1alpha1.ProvidedMode,
					ImportSecretName: pgurImportSecretName,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgurNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
						{
							Privilege:           postgresqlv1alpha1.WriterPrivilege,
							Database:            &common.CRLink{Name: pgdbName},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Privilege list mustn't have the same database listed multiple times"))
		})

		It("should fail when import secret have no USERNAME and PASSWORD", func() {
			// Create secret
			sec := &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurImportSecretName,
					Namespace: pgurNamespace,
				},
				StringData: map[string]string{},
			}

			Expect(k8sClient.Create(ctx, sec)).To(Succeed())

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:             postgresqlv1alpha1.ProvidedMode,
					ImportSecretName: pgurImportSecretName,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Import secret must have a USERNAME and PASSWORD valuated keys"))
		})

		It("should fail when import secret have no PASSWORD", func() {
			// Create secret
			sec := &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurImportSecretName,
					Namespace: pgurNamespace,
				},
				StringData: map[string]string{
					"USERNAME": "fake",
				},
			}

			Expect(k8sClient.Create(ctx, sec)).To(Succeed())

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:             postgresqlv1alpha1.ProvidedMode,
					ImportSecretName: pgurImportSecretName,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Import secret must have a USERNAME and PASSWORD valuated keys"))
		})

		It("should fail when import secret have no USERNAME", func() {
			// Create secret
			sec := &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurImportSecretName,
					Namespace: pgurNamespace,
				},
				StringData: map[string]string{
					"PASSWORD": "fake",
				},
			}

			Expect(k8sClient.Create(ctx, sec)).To(Succeed())

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:             postgresqlv1alpha1.ProvidedMode,
					ImportSecretName: pgurImportSecretName,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Import secret must have a USERNAME and PASSWORD valuated keys"))
		})

		It("should fail when import secret have a too long USERNAME", func() {
			// Create secret
			sec := &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurImportSecretName,
					Namespace: pgurNamespace,
				},
				StringData: map[string]string{
					"PASSWORD": "fake",
					"USERNAME": "fakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefake",
				},
			}

			Expect(k8sClient.Create(ctx, sec)).To(Succeed())

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:             postgresqlv1alpha1.ProvidedMode,
					ImportSecretName: pgurImportSecretName,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Username is too long. It must be <= 63. fakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefake is 96 character. Username length must be reduced"))
		})

		It("should fail to look a not found pgdb", func() {
			// Create secret
			setupPGURImportSecret()

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:             postgresqlv1alpha1.ProvidedMode,
					ImportSecretName: pgurImportSecretName,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: "fake", Namespace: "fake"},
							GeneratedSecretName: "pgur",
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(ContainSubstring("\"fake\" not found"))
		})

		It("should fail with a non ready pgdb", func() {
			// Create pgdb
			pgdb := &postgresqlv1alpha1.PostgresqlDatabase{
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
					DropOnDelete: true,
				},
			}

			// Create
			Expect(k8sClient.Create(ctx, pgdb)).Should(Succeed())

			// Create secret
			setupPGURImportSecret()

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

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			time.Sleep(5 * time.Second)

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item)
			Expect(err).Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleNoPhase))
		})

		It("should be ok without work secret name", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:             postgresqlv1alpha1.ProvidedMode,
					ImportSecretName: pgurImportSecretName,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).ToNot(Equal(pgurWorkSecretName))
			Expect(item.Spec.WorkGeneratedSecretName).To(MatchRegexp(DefaultWorkGeneratedSecretNamePrefix + ".*"))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(pgurImportUsername))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(pgurImportPassword))

			// Get db secret
			sec = &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword, pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(pgurImportUsername, pgurImportPassword)
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(pgurImportUsername, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(pgurImportUsername, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok with work secret name", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(pgurImportUsername))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(pgurImportPassword))

			// Get db secret
			sec = &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword, pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(pgurImportUsername, pgurImportPassword)
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(pgurImportUsername, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(pgurImportUsername, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok with 2 databases", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)
			pgdb2 := setupPGDB2()

			// Create secret
			setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			item := setupProvidedPGURWith2Databases()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(pgurImportUsername))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(pgurImportPassword))

			// Get db secret
			sec = &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())
			sec2 := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName2,
				Namespace: pgurNamespace,
			}, sec2)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword, pgec, v1alpha1.PrimaryConnectionType)
			checkPGURSecretValues(pgurDBSecretName2, pgurNamespace, pgdbDBName2, pgurImportUsername, pgurImportPassword, pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(pgurImportUsername, pgurImportPassword)
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(pgurImportUsername, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			memberOf, err = isSQLUserMemberOf(pgurImportUsername, pgdb2.Status.Roles.Writer)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(pgurImportUsername, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			sett, err = isSetRoleOnDatabasesRoleSettingsExists(pgurImportUsername, pgdbDBName2, pgdb2.Status.Roles.Writer)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to edit work secret", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(pgurImportUsername))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(pgurImportPassword))

			oldValue := sec.Data[PasswordSecretKey]
			sec.Data[PasswordSecretKey] = []byte("updated")
			// Save
			Expect(k8sClient.Update(ctx, sec)).To(Succeed())

			// Get work secret
			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					if string(sec2.Data[PasswordSecretKey]) != string(oldValue) {
						return errors.New("work secret password not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should be ok to remove key in work secret", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(pgurImportUsername))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(pgurImportPassword))

			oldValue := sec.Data[PasswordSecretKey]
			delete(sec.Data, PasswordSecretKey)
			// Save
			Expect(k8sClient.Update(ctx, sec)).To(Succeed())

			// Get work secret
			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					if string(sec2.Data[PasswordSecretKey]) != string(oldValue) {
						return errors.New("work secret password not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should be ok to edit db secret", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			oldValue := sec.Data["POSTGRES_URL_ARGS"]
			sec.Data["POSTGRES_URL_ARGS"] = []byte("updated")
			// Save
			Expect(k8sClient.Update(ctx, sec)).To(Succeed())

			// Get work secret
			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurDBSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					if string(sec2.Data["POSTGRES_URL_ARGS"]) != string(oldValue) {
						return errors.New("db secret not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should be ok to remove key in db secret", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(pgurImportUsername))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(pgurImportPassword))

			oldValue := string(sec.Data["POSTGRES_URL_ARGS"])
			delete(sec.Data, "POSTGRES_URL_ARGS")
			// Save
			Expect(k8sClient.Update(ctx, sec)).To(Succeed())

			// Get work secret
			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					if string(sec2.Data["POSTGRES_URL_ARGS"]) != oldValue {
						return errors.New("db secret not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should be ok to change password", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			// Create secret
			isec := setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Wait
			time.Sleep(2 * time.Second)

			updatedPass := "updated"
			// Update password
			isec.Data[PasswordSecretKey] = []byte(updatedPass)
			// Save
			Expect(k8sClient.Update(ctx, isec)).To(Succeed())

			// Get work secret
			sec := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec)
					// Check error
					if err != nil {
						return err
					}

					if string(sec.Data[PasswordSecretKey]) != updatedPass {
						return errors.New("work secret password not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(pgurImportUsername))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(updatedPass))

			// Get pgur
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item)).To(Succeed())
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d2, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d2.After(d)).To(BeTrue())
			Expect(d2.After(preDate)).To(BeTrue())

			// Get db secret
			sec = &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, pgurImportUsername, updatedPass, pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(pgurImportUsername, updatedPass)
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(pgurImportUsername, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(pgurImportUsername, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to change username", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			// Create secret
			isec := setupPGURImportSecret()

			preDate := time.Now().Add(-time.Second)

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(""))
			Expect(item.Status.PostgresRole).To(Equal(pgurImportUsername))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Wait
			time.Sleep(time.Second)

			updatedUser := "updated"
			// Update username
			isec.Data[UsernameSecretKey] = []byte(updatedUser)
			// Save
			Expect(k8sClient.Update(ctx, isec)).To(Succeed())

			// Get work secret
			sec := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec)
					// Check error
					if err != nil {
						return err
					}

					if string(sec.Data[UsernameSecretKey]) != updatedUser {
						return errors.New("work secret username not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(updatedUser))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(pgurImportPassword))

			// Get pgur
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item)).To(Succeed())
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d2, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d2.After(d)).To(BeTrue())
			Expect(d2.After(preDate)).To(BeTrue())
			Expect(item.Status.PostgresRole).To(Equal(updatedUser))

			// Get db secret
			sec = &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, updatedUser, pgurImportPassword, pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(updatedUser, pgurImportPassword)
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			exists, err = isSQLRoleExists(updatedUser)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(updatedUser, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(updatedUser, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to change username and wait old user disconnection", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			isec := setupPGURImportSecret()

			item := setupProvidedPGUR()

			// Connect
			k, err := connectAs(pgurImportUsername, pgurImportPassword)
			Expect(err).To(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))

			updatedUser := "updated"
			// Update username
			isec.Data[UsernameSecretKey] = []byte(updatedUser)
			// Save
			Expect(k8sClient.Update(ctx, isec)).To(Succeed())

			// Get work secret
			sec := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec)
					// Check error
					if err != nil {
						return err
					}

					if string(sec.Data[UsernameSecretKey]) != updatedUser {
						return errors.New("work secret username not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(updatedUser))
			Expect(string(sec.Data[PasswordSecretKey])).To(Equal(pgurImportPassword))

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, updatedUser, pgurImportPassword, pgec, v1alpha1.PrimaryConnectionType)

			// Get pgur
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item)).To(Succeed())

			Expect(item.Status.OldPostgresRoles).To(Equal([]string{pgurImportUsername}))

			exists, err := isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			// Disconnect
			Expect(disconnectConnFromKey(k)).To(Succeed())

			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					if len(item.Status.OldPostgresRoles) != 0 {
						return errors.New("old user not cleaned after disconnection")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

				// Connect to check user
			_, err = connectAs(updatedUser, pgurImportPassword)
			Expect(err).To(Succeed())

			exists, err = isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			exists, err = isSQLRoleExists(updatedUser)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())
		})

		It("should be ok to recover a work secret removal", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			setupProvidedPGUR()

			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurWorkSecretName,
				Namespace: pgurNamespace,
			}, workSec)).To(Succeed())

			Expect(k8sClient.Delete(ctx, workSec)).To(Succeed())

			workSec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurWorkSecretName,
						Namespace: pgurNamespace,
					}, workSec2)
					// Check error
					if err != nil {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(workSec.Data).To(Equal(workSec2.Data))
		})

		It("should be ok to recover a db secret removal", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			setupProvidedPGUR()

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).To(Succeed())

			Expect(k8sClient.Delete(ctx, sec)).To(Succeed())

			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurDBSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(sec.Data).To(Equal(sec2.Data))
		})

		It("should be ok to change rights", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGUR()

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).To(Succeed())

			memberOf, err := isSQLUserMemberOf(pgurImportUsername, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(pgurImportUsername, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			// Update
			item.Spec.Privileges[0].Privilege = postgresqlv1alpha1.WriterPrivilege
			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			Eventually(
				func() error {
					memberOf, err := isSQLUserMemberOf(pgurImportUsername, pgdb.Status.Roles.Writer)
					// Check error
					if err != nil {
						return err
					}

					if !memberOf {
						return errors.New("user in pg not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			memberOf, err = isSQLUserMemberOf(pgurImportUsername, pgdb.Status.Roles.Writer)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err = isSetRoleOnDatabasesRoleSettingsExists(pgurImportUsername, pgdbDBName, pgdb.Status.Roles.Writer)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			// Get work secret
			sec2 := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec2)).To(Succeed())

			Expect(sec.Data).To(Equal(sec2.Data))
		})

		It("should be ok to remove a non valid item", func() {
			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode: postgresqlv1alpha1.ProvidedMode,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Delete item
			Expect(k8sClient.Delete(ctx, item)).To(Succeed())

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Ensure this is deleted
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item2)

					// Check error
					if err == nil {
						return errors.New("object not deleted")
					}

					if err != nil && !apimachineryErrors.IsNotFound(err) {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should be ok to remove a valid item", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGUR()

			// Delete item
			Expect(k8sClient.Delete(ctx, item)).To(Succeed())

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Ensure this is deleted
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item2)

					// Check error
					if err == nil {
						return errors.New("object not deleted")
					}

					if err != nil && !apimachineryErrors.IsNotFound(err) {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			exists, err := isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())
		})

		It("should be ok to remove a valid item but blocked by connection", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGUR()

			k, err := connectAs(pgurImportUsername, pgurImportPassword)
			Expect(err).To(Succeed())

			// Delete item
			Expect(k8sClient.Delete(ctx, item)).To(Succeed())

			it := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Ensure this is deleted
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, it)
					if err != nil {
						return err
					}

					if it.Status.Phase != postgresqlv1alpha1.UserRoleFailedPhase {
						return errors.New("not updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(it.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(it.Status.OldPostgresRoles).To(Equal([]string{pgurImportUsername}))
			Expect(it.Status.Message).To(Equal("old postgres roles still present"))

			Expect(disconnectConnFromKey(k)).To(Succeed())

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item2)).To(Succeed())

			// Ensure this is deleted
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item2)

					// Check error
					if err == nil {
						return errors.New("object not deleted")
					}

					if err != nil && !apimachineryErrors.IsNotFound(err) {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			exists, err := isSQLRoleExists(pgurImportUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())
		})

		It("should be ok to change db secret name", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			dbsecOri := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsecOri)).To(Succeed())

			// Edit secret name
			item.Spec.Privileges[0].GeneratedSecretName = editedSecretName

			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			Eventually(
				func() error {
					dbsec := &corev1.Secret{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurDBSecretName,
						Namespace: pgurNamespace,
					}, dbsec)
					// Check error
					if err != nil && !apimachineryErrors.IsNotFound(err) {
						return err
					}

					if err != nil && apimachineryErrors.IsNotFound(err) {
						return nil
					}

					if err == nil || dbsec.DeletionTimestamp == nil {
						return errors.New("secret still present")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item2)).To(Succeed())

			Expect(item2.Status.Ready).To(BeTrue())

			dbsec := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      editedSecretName,
						Namespace: pgurNamespace,
					}, dbsec)
					// Check error
					if err != nil {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(dbsecOri.Data).To(Equal(dbsec.Data))
		})

		It("should be ok to change work secret name", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			worksecOri := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurWorkSecretName,
				Namespace: pgurNamespace,
			}, worksecOri)).To(Succeed())

			// Edit secret name
			item.Spec.WorkGeneratedSecretName = editedSecretName

			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			Eventually(
				func() error {
					worksec := &corev1.Secret{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurWorkSecretName,
						Namespace: pgurNamespace,
					}, worksec)
					// Check error
					if err != nil && !apimachineryErrors.IsNotFound(err) {
						return err
					}

					if err != nil && apimachineryErrors.IsNotFound(err) {
						return nil
					}

					if err == nil || worksec.DeletionTimestamp == nil {
						return errors.New("secret still present")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item2)).To(Succeed())

			Expect(item2.Status.Ready).To(BeTrue())

			worksec := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      editedSecretName,
						Namespace: pgurNamespace,
					}, worksec)
					// Check error
					if err != nil {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(worksecOri.Data[UsernameSecretKey]).To(Equal(worksec.Data[UsernameSecretKey]))
		})

		It("should be ok to generate a primary user role with a bouncer enabled pgec", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			// Validate
			checkPGURSecretValues(item.Spec.Privileges[0].GeneratedSecretName, pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword, pgec, v1alpha1.PrimaryConnectionType)
		})

		It("should be ok to generate a bouncer secret with a bouncer user role", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGURWithBouncer()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			// Validate
			checkPGURSecretValues(item.Spec.Privileges[0].GeneratedSecretName, pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword, pgec, v1alpha1.BouncerConnectionType)
		})

		It("should be fail when a bouncer user role is asked but pgec isn't supporting it", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGURWithBouncer()

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("bouncer connection asked but not supported in engine configuration"))
		})

		It("should be ok to generate a bouncer and a primary secret with a bouncer and a primary user role", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)
			setupPGDB2()

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGURWith2DatabasesWithPrimaryAndBouncer()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword, pgec,
				v1alpha1.PrimaryConnectionType,
			)
			checkPGURSecretValues(
				item.Spec.Privileges[1].GeneratedSecretName,
				pgurNamespace, pgdbDBName2, pgurImportUsername, pgurImportPassword, pgec,
				v1alpha1.BouncerConnectionType,
			)
		})

		It("should be ok to create a primary user role and change it to a bouncer one", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGUR()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword,
				pgec, v1alpha1.PrimaryConnectionType,
			)

			// Get current secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.Privileges[0].GeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).To(Succeed())

			// Update privilege for a bouncer one
			item.Spec.Privileges[0].ConnectionType = v1alpha1.BouncerConnectionType
			// Save
			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.Privileges[0].GeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					// Check if sec have been updated
					if string(sec.Data["POSTGRES_URL"]) == string(sec2.Data["POSTGRES_URL"]) {
						return errors.New("Secret not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

				// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword,
				pgec, v1alpha1.BouncerConnectionType,
			)
		})

		It("should be ok to create a bouncer user role and change it to a primary one", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)

			// Create secret
			setupPGURImportSecret()

			item := setupProvidedPGURWithBouncer()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword,
				pgec, v1alpha1.BouncerConnectionType,
			)

			// Get current secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.Privileges[0].GeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).To(Succeed())

			// Update privilege for a bouncer one
			item.Spec.Privileges[0].ConnectionType = v1alpha1.PrimaryConnectionType
			// Save
			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.Privileges[0].GeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					// Check if sec have been updated
					if string(sec.Data["POSTGRES_URL"]) == string(sec2.Data["POSTGRES_URL"]) {
						return errors.New("Secret not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

				// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, pgurImportUsername, pgurImportPassword,
				pgec, v1alpha1.PrimaryConnectionType,
			)
		})
	})

	Describe("Managed mode", func() {
		It("should fail when role prefix isn't provided", func() {
			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode: postgresqlv1alpha1.ManagedMode,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("PostgresqlUserRole is in managed mode without any RolePrefix"))
		})

		It("should fail when privileges contains 2 times the same db (twice fully declared)", func() {
			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:       postgresqlv1alpha1.ManagedMode,
					RolePrefix: pgurRolePrefix,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
						{
							Privilege:           postgresqlv1alpha1.WriterPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Privilege list mustn't have the same database listed multiple times"))
		})

		It("should fail when privileges contains 2 times the same db (1 fully declared, 1 without namespace)", func() {
			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:       postgresqlv1alpha1.ManagedMode,
					RolePrefix: pgurRolePrefix,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgurNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
						{
							Privilege:           postgresqlv1alpha1.WriterPrivilege,
							Database:            &common.CRLink{Name: pgdbName},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Privilege list mustn't have the same database listed multiple times"))
		})

		It("should fail when role prefix is too long", func() {
			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:       postgresqlv1alpha1.ManagedMode,
					RolePrefix: "fakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefake",
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("Role prefix is too long. It must be <= 63. fakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefakefake-0X is 99 character. Role prefix length must be reduced"))
		})

		It("should fail when UserPasswordRotationDuration isn't a valid duration", func() {
			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:                         postgresqlv1alpha1.ManagedMode,
					RolePrefix:                   pgurRolePrefix,
					UserPasswordRotationDuration: "fake",
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal(`time: invalid duration "fake"`))
		})

		It("should fail to look a not found pgdb", func() {
			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:       postgresqlv1alpha1.ManagedMode,
					RolePrefix: pgurRolePrefix,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: "fake", Namespace: "fake"},
							GeneratedSecretName: "pgur",
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(ContainSubstring("\"fake\" not found"))
		})

		It("should fail with a non ready pgdb", func() {
			// Create pgdb
			pgdb := &postgresqlv1alpha1.PostgresqlDatabase{
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
					DropOnDelete: true,
				},
			}

			// Create
			Expect(k8sClient.Create(ctx, pgdb)).Should(Succeed())

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
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			time.Sleep(5 * time.Second)

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item)
			Expect(err).Should(Succeed())

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleNoPhase))
			Expect(item.Status.Message).To(Equal(""))
		})

		It("should be ok without work secret name", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			it := &postgresqlv1alpha1.PostgresqlUserRole{
				ObjectMeta: v1.ObjectMeta{
					Name:      pgurName,
					Namespace: pgurNamespace,
				},
				Spec: postgresqlv1alpha1.PostgresqlUserRoleSpec{
					Mode:       postgresqlv1alpha1.ManagedMode,
					RolePrefix: pgurRolePrefix,
					Privileges: []*postgresqlv1alpha1.PostgresqlUserRolePrivilege{
						{
							Privilege:           postgresqlv1alpha1.OwnerPrivilege,
							Database:            &common.CRLink{Name: pgdbName, Namespace: pgdbNamespace},
							GeneratedSecretName: pgurDBSecretName,
						},
					},
				},
			}

			// Create user
			Expect(k8sClient.Create(ctx, it)).Should(Succeed())

			item := &postgresqlv1alpha1.PostgresqlUserRole{}
			// Get updated user
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item)
					// Check error
					if err != nil {
						return err
					}

					// Check if status hasn't been updated
					if item.Status.Phase == postgresqlv1alpha1.UserRoleNoPhase {
						return errors.New("pgur hasn't been updated by operator")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			username := pgurRolePrefix + Login0Suffix
			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).ToNot(Equal(pgurWorkSecretName))
			Expect(item.Spec.WorkGeneratedSecretName).To(MatchRegexp(DefaultWorkGeneratedSecretNamePrefix + ".*"))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(username))
			Expect(string(sec.Data[PasswordSecretKey])).ToNot(Equal(""))
			Expect(string(sec.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username, string(sec.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(username, string(sec.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok with work secret name", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("")

			username := pgurRolePrefix + Login0Suffix
			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(username))
			Expect(string(sec.Data[PasswordSecretKey])).ToNot(Equal(""))
			Expect(string(sec.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username, string(sec.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(username, string(sec.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok with 2 databases", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)
			pgdb2 := setupPGDB2()

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGURWith2Databases()

			username := pgurRolePrefix + Login0Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(username))
			Expect(string(sec.Data[PasswordSecretKey])).ToNot(Equal(""))
			Expect(string(sec.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())
			dbsec2 := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName2,
				Namespace: pgurNamespace,
			}, dbsec2)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username, string(sec.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)
			checkPGURSecretValues(pgurDBSecretName2, pgurNamespace, pgdbDBName2, username, string(sec.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(username, string(sec.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			memberOf, err = isSQLUserMemberOf(username, pgdb2.Status.Roles.Writer)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			sett, err = isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName2, pgdb2.Status.Roles.Writer)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to edit work secret", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("")

			username := pgurRolePrefix + Login0Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			oldValue := string(sec.Data[PasswordSecretKey])
			sec.Data[PasswordSecretKey] = []byte("updated")
			// Save
			Expect(k8sClient.Update(ctx, sec)).To(Succeed())

			// Get work secret
			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					if string(sec2.Data[PasswordSecretKey]) != "updated" {
						return errors.New("work secret password not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(string(sec2.Data[PasswordSecretKey])).NotTo(Equal(oldValue))
		})

		It("should be ok to remove key in work secret", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("")

			username := pgurRolePrefix + Login0Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			oldValue := sec.Data[UsernameSecretKey]
			delete(sec.Data, UsernameSecretKey)
			// Save
			Expect(k8sClient.Update(ctx, sec)).To(Succeed())

			// Get work secret
			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					if string(sec2.Data[UsernameSecretKey]) != string(oldValue) {
						return errors.New("work secret password not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should be ok to edit db secret", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			preDate := time.Now().Add(-time.Second)
			username := pgurRolePrefix + Login0Suffix

			item := setupManagedPGUR("")

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			oldValue := sec.Data["POSTGRES_URL_ARGS"]
			sec.Data["POSTGRES_URL_ARGS"] = []byte("updated")
			// Save
			Expect(k8sClient.Update(ctx, sec)).To(Succeed())

			// Get work secret
			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurDBSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					if string(sec2.Data["POSTGRES_URL_ARGS"]) != string(oldValue) {
						return errors.New("db secret not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should be ok to remove key in db secret", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			preDate := time.Now().Add(-time.Second)
			username := pgurRolePrefix + Login0Suffix

			item := setupManagedPGUR("")

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			oldValue := string(sec.Data["POSTGRES_URL_ARGS"])
			delete(sec.Data, "POSTGRES_URL_ARGS")
			// Save
			Expect(k8sClient.Update(ctx, sec)).To(Succeed())

			// Get work secret
			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurDBSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					if string(sec2.Data["POSTGRES_URL_ARGS"]) != oldValue {
						return errors.New("db secret not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should be ok to change role prefix", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("")

			username := pgurRolePrefix + Login0Suffix
			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Wait
			time.Sleep(time.Second)

			oldUsername := username
			updatedUserPrefix := "updated"
			username = updatedUserPrefix + Login0Suffix
			// Update username
			item.Spec.RolePrefix = updatedUserPrefix
			// Save
			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			// Get work secret
			sec := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.WorkGeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec)
					// Check error
					if err != nil {
						return err
					}

					if string(sec.Data[UsernameSecretKey]) != username {
						return errors.New("work secret username not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(username))
			Expect(string(sec.Data[PasswordSecretKey])).ToNot(Equal(""))
			Expect(string(sec.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))

			// Get pgur
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item)).To(Succeed())
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			Expect(item.Status.RolePrefix).To(Equal(updatedUserPrefix))
			d2, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d2.After(d)).To(BeTrue())
			Expect(d2.After(preDate)).To(BeTrue())
			Expect(item.Status.PostgresRole).To(Equal(username))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username, string(sec.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(username, string(sec.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(oldUsername)
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			exists, err = isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to manage a work secret removal", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			setupManagedPGUR("")

			username := pgurRolePrefix + Login0Suffix

			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurWorkSecretName,
				Namespace: pgurNamespace,
			}, workSec)).To(Succeed())

			Expect(k8sClient.Delete(ctx, workSec)).To(Succeed())

			workSec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurWorkSecretName,
						Namespace: pgurNamespace,
					}, workSec2)
					// Check error
					if err != nil {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(workSec.Data).NotTo(Equal(workSec2.Data))
			Expect(string(workSec.Data[UsernameSecretKey])).To(Equal(string(workSec2.Data[UsernameSecretKey])))
			Expect(string(workSec2.Data[UsernameSecretKey])).To(Equal(username))
			Expect(string(workSec.Data[PasswordSecretKey])).NotTo(Equal(string(workSec2.Data[PasswordSecretKey])))
			Expect(string(workSec2.Data[PasswordSecretKey])).NotTo(Equal(""))
			Expect(string(workSec2.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username, string(workSec2.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err := connectAs(username, string(workSec2.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to recover a db secret removal", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			setupManagedPGUR("")

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).To(Succeed())

			Expect(k8sClient.Delete(ctx, sec)).To(Succeed())

			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurDBSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(sec.Data).To(Equal(sec2.Data))
		})

		It("should be ok to change rights", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			item := setupManagedPGUR("")

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec)).To(Succeed())

			username := item.Status.PostgresRole

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			// Update
			item.Spec.Privileges[0].Privilege = postgresqlv1alpha1.WriterPrivilege
			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			Eventually(
				func() error {
					memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Writer)
					// Check error
					if err != nil {
						return err
					}

					if !memberOf {
						return errors.New("user in pg not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			memberOf, err = isSQLUserMemberOf(username, pgdb.Status.Roles.Writer)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err = isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Writer)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			// Get work secret
			sec2 := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, sec2)).To(Succeed())

			Expect(sec.Data).To(Equal(sec2.Data))
		})

		It("should be ok to have rolling password enabled but not performed yet", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("60s")

			username := pgurRolePrefix + Login0Suffix
			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).Should(Succeed())

			Expect(string(sec.Data[UsernameSecretKey])).To(Equal(username))
			Expect(string(sec.Data[PasswordSecretKey])).ToNot(Equal(""))
			Expect(string(sec.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username, string(sec.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(username, string(sec.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to have rolling password enabled and performed", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("5s")

			username := pgurRolePrefix + Login0Suffix
			username2 := pgurRolePrefix + Login1Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			Expect(item.Status.OldPostgresRoles).To(Equal([]string{}))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Wait
			time.Sleep(4 * time.Second)

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Eventually(
				func() error {
					k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item2)
					// Check error
					if err != nil {
						return err
					}

					if item.Status.PostgresRole == item2.Status.PostgresRole {
						return errors.New("pgur not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item2.Status.Ready).To(BeTrue())
			Expect(item2.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item2.Status.Message).To(Equal(""))
			Expect(item2.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item2.Status.PostgresRole).To(Equal(username2))
			Expect(item2.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item2.Status.OldPostgresRoles).To(Equal([]string{}))
			d2, err := time.Parse(time.RFC3339, item2.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d2.After(d)).To(BeTrue())
			Expect(d2.After(preDate)).To(BeTrue())

			// Get work secret
			workSec2 := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item2.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec2)).Should(Succeed())

			Expect(string(workSec2.Data[UsernameSecretKey])).To(Equal(username2))
			Expect(string(workSec2.Data[PasswordSecretKey])).ToNot(Equal(""))
			Expect(string(workSec2.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))
			Expect(string(workSec2.Data[PasswordSecretKey])).ToNot(Equal(string(workSec.Data[PasswordSecretKey])))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username2, string(workSec2.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(username2, string(workSec2.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeFalse())

			exists, err = isSQLRoleExists(username2)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username2, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username2, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to have rolling password enabled and performed with old user still connected", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("5s")

			username := pgurRolePrefix + Login0Suffix
			username2 := pgurRolePrefix + Login1Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			Expect(item.Status.OldPostgresRoles).To(Equal([]string{}))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Connect
			_, err = connectAs(username, string(workSec.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			// Wait
			time.Sleep(4 * time.Second)

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Eventually(
				func() error {
					k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item2)
					// Check error
					if err != nil {
						return err
					}

					if item.Status.PostgresRole == item2.Status.PostgresRole {
						return errors.New("pgur not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item2.Status.Ready).To(BeTrue())
			Expect(item2.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item2.Status.Message).To(Equal(""))
			Expect(item2.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item2.Status.PostgresRole).To(Equal(username2))
			Expect(item2.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item2.Status.OldPostgresRoles).To(Equal([]string{username}))
			d2, err := time.Parse(time.RFC3339, item2.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d2.After(d)).To(BeTrue())
			Expect(d2.After(preDate)).To(BeTrue())

			// Get work secret
			workSec2 := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item2.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec2)).Should(Succeed())

			Expect(string(workSec2.Data[UsernameSecretKey])).To(Equal(username2))
			Expect(string(workSec2.Data[PasswordSecretKey])).ToNot(Equal(""))
			Expect(string(workSec2.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))
			Expect(string(workSec2.Data[PasswordSecretKey])).ToNot(Equal(string(workSec.Data[PasswordSecretKey])))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username2, string(workSec2.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(username2, string(workSec2.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			exists, err = isSQLRoleExists(username2)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			memberOf, err = isSQLUserMemberOf(username2, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			sett, err = isSetRoleOnDatabasesRoleSettingsExists(username2, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())
		})

		It("should be ok to have rolling password enabled and performed with old user still connected and finally released", func() {
			// Setup pgec
			pgec, _ := setupPGEC("30s", false)
			// Create pgdb
			pgdb := setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("10s")

			username := pgurRolePrefix + Login0Suffix
			username2 := pgurRolePrefix + Login1Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			Expect(item.Status.OldPostgresRoles).To(Equal([]string{}))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Connect
			key, err := connectAs(username, string(workSec.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			// Wait
			time.Sleep(9 * time.Second)

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Eventually(
				func() error {
					k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item2)
					// Check error
					if err != nil {
						return err
					}

					if item.Status.PostgresRole == item2.Status.PostgresRole {
						return errors.New("pgur not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Checks
			Expect(item2.Status.Ready).To(BeTrue())
			Expect(item2.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item2.Status.Message).To(Equal(""))
			Expect(item2.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item2.Status.PostgresRole).To(Equal(username2))
			Expect(item2.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item2.Status.OldPostgresRoles).To(Equal([]string{username}))
			d2, err := time.Parse(time.RFC3339, item2.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d2.After(d)).To(BeTrue())
			Expect(d2.After(preDate)).To(BeTrue())

			// Get work secret
			workSec2 := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item2.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec2)).Should(Succeed())

			Expect(string(workSec2.Data[UsernameSecretKey])).To(Equal(username2))
			Expect(string(workSec2.Data[PasswordSecretKey])).ToNot(Equal(""))
			Expect(string(workSec2.Data[PasswordSecretKey])).To(HaveLen(ManagedPasswordSize))
			Expect(string(workSec2.Data[PasswordSecretKey])).ToNot(Equal(string(workSec.Data[PasswordSecretKey])))

			// Get db secret
			dbsec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(pgurDBSecretName, pgurNamespace, pgdbDBName, username2, string(workSec2.Data[PasswordSecretKey]), pgec, v1alpha1.PrimaryConnectionType)

			// Connect to check user
			_, err = connectAs(username2, string(workSec2.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			exists, err := isSQLRoleExists(username)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			exists, err = isSQLRoleExists(username2)
			Expect(err).To(Succeed())
			Expect(exists).To(BeTrue())

			memberOf, err := isSQLUserMemberOf(username, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			memberOf, err = isSQLUserMemberOf(username2, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(memberOf).To(BeTrue())

			sett, err := isSetRoleOnDatabasesRoleSettingsExists(username, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			sett, err = isSetRoleOnDatabasesRoleSettingsExists(username2, pgdbDBName, pgdb.Status.Roles.Owner)
			Expect(err).To(Succeed())
			Expect(sett).To(BeTrue())

			// Disconnect old
			Expect(disconnectConnFromKey(key)).To(Succeed())

			item3 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Eventually(
				func() error {
					k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item3)
					// Check error
					if err != nil {
						return err
					}

					if len(item3.Status.OldPostgresRoles) == 0 {
						return errors.New("pgur not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())
		})

		It("should fail to have rolling password enabled and performed 2 times with 2 users connected", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("3s")

			username := pgurRolePrefix + Login0Suffix
			username2 := pgurRolePrefix + Login1Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			Expect(item.Status.OldPostgresRoles).To(Equal([]string{}))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Connect
			_, err = connectAs(username, string(workSec.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			// Wait
			time.Sleep(2 * time.Second)

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Eventually(
				func() error {
					k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item2)
					// Check error
					if err != nil {
						return err
					}

					if item.Status.PostgresRole == item2.Status.PostgresRole {
						return errors.New("pgur not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			// Get work secret
			workSec2 := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec2)).Should(Succeed())

			// Connect
			_, err = connectAs(username2, string(workSec2.Data[PasswordSecretKey]))
			Expect(err).To(Succeed())

			// Wait
			time.Sleep(4 * time.Second)

			item3 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Eventually(
				func() error {
					k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurName,
						Namespace: pgurNamespace,
					}, item3)
					// Check error
					if err != nil {
						return err
					}

					if item3.Status.Phase != postgresqlv1alpha1.UserRoleFailedPhase {
						return errors.New("pgur not in failure status")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(item3.Status.Ready).To(BeFalse())
			Expect(item3.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item3.Status.PostgresRole).To(Equal(username2))
			Expect(item3.Status.RolePrefix).To(Equal(item3.Spec.RolePrefix))
			Expect(item3.Status.OldPostgresRoles).To(Equal([]string{username}))
			Expect(item3.Status.Message).To(Equal("Old user password rotation wasn't a success and another one must be done."))
		})

		It("should be ok to change db secret name", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("")

			username := pgurRolePrefix + Login0Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			Expect(item.Status.OldPostgresRoles).To(Equal([]string{}))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			dbsecOri := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurDBSecretName,
				Namespace: pgurNamespace,
			}, dbsecOri)).To(Succeed())

			// Edit secret name
			item.Spec.Privileges[0].GeneratedSecretName = editedSecretName

			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			Eventually(
				func() error {
					dbsec := &corev1.Secret{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurDBSecretName,
						Namespace: pgurNamespace,
					}, dbsec)
					// Check error
					if err != nil && !apimachineryErrors.IsNotFound(err) {
						return err
					}

					if err != nil && apimachineryErrors.IsNotFound(err) {
						return nil
					}

					if err == nil || dbsec.DeletionTimestamp == nil {
						return errors.New("secret still present")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item2)).To(Succeed())

			Expect(item2.Status.Ready).To(BeTrue())

			dbsec := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      editedSecretName,
						Namespace: pgurNamespace,
					}, dbsec)
					// Check error
					if err != nil {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(dbsecOri.Data).To(Equal(dbsec.Data))
		})

		It("should be ok to change work secret name", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			preDate := time.Now().Add(-time.Second)

			item := setupManagedPGUR("")

			username := pgurRolePrefix + Login0Suffix

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))
			Expect(item.Status.Message).To(Equal(""))
			Expect(item.Status.RolePrefix).To(Equal(pgurRolePrefix))
			Expect(item.Status.PostgresRole).To(Equal(username))
			Expect(item.Spec.WorkGeneratedSecretName).To(Equal(pgurWorkSecretName))
			Expect(item.Spec.Privileges[0].ConnectionType).To(Equal(postgresqlv1alpha1.PrimaryConnectionType))
			Expect(item.Status.OldPostgresRoles).To(Equal([]string{}))
			d, err := time.Parse(time.RFC3339, item.Status.LastPasswordChangedTime)
			Expect(err).To(Succeed())
			Expect(d.After(preDate)).To(BeTrue())

			worksecOri := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurWorkSecretName,
				Namespace: pgurNamespace,
			}, worksecOri)).To(Succeed())

			// Edit secret name
			item.Spec.WorkGeneratedSecretName = editedSecretName

			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			Eventually(
				func() error {
					worksec := &corev1.Secret{}
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      pgurWorkSecretName,
						Namespace: pgurNamespace,
					}, worksec)
					// Check error
					if err != nil && !apimachineryErrors.IsNotFound(err) {
						return err
					}

					if err != nil && apimachineryErrors.IsNotFound(err) {
						return nil
					}

					if err == nil || worksec.DeletionTimestamp == nil {
						return errors.New("secret still present")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			item2 := &postgresqlv1alpha1.PostgresqlUserRole{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      pgurName,
				Namespace: pgurNamespace,
			}, item2)).To(Succeed())

			Expect(item2.Status.Ready).To(BeTrue())

			worksec := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      editedSecretName,
						Namespace: pgurNamespace,
					}, worksec)
					// Check error
					if err != nil {
						return err
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

			Expect(worksecOri.Data[UsernameSecretKey]).To(Equal(worksec.Data[UsernameSecretKey]))
		})

		It("should be ok to generate a primary secret with a bouncer enabled pgec", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)

			item := setupManagedPGUR("")

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			username := pgurRolePrefix + Login0Suffix
			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, username, string(workSec.Data[PasswordSecretKey]),
				pgec, v1alpha1.PrimaryConnectionType,
			)
		})

		It("should be ok to generate a bouncer secret with a bouncer user role", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)

			item := setupManagedPGURWithBouncer("")

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			username := pgurRolePrefix + Login0Suffix
			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, username, string(workSec.Data[PasswordSecretKey]),
				pgec, v1alpha1.BouncerConnectionType,
			)
		})

		It("should be fail when a bouncer user role is asked but pgec isn't supporting it", func() {
			// Setup pgec
			setupPGEC("30s", false)
			// Create pgdb
			setupPGDB(false)

			item := setupManagedPGURWithBouncer("")

			// Checks
			Expect(item.Status.Ready).To(BeFalse())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleFailedPhase))
			Expect(item.Status.Message).To(Equal("bouncer connection asked but not supported in engine configuration"))
		})

		It("should be ok to generate a bouncer and a primary secret with a bouncer and a primary user role", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)
			setupPGDB2()

			item := setupManagedPGURWith2DatabasesWithPrimaryAndBouncer()

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			username := pgurRolePrefix + Login0Suffix
			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, username, string(workSec.Data[PasswordSecretKey]),
				pgec, v1alpha1.PrimaryConnectionType,
			)
			checkPGURSecretValues(
				item.Spec.Privileges[1].GeneratedSecretName,
				pgurNamespace, pgdbDBName2, username, string(workSec.Data[PasswordSecretKey]),
				pgec, v1alpha1.BouncerConnectionType,
			)
		})

		It("should be ok to create a primary user role and change it to a bouncer one", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)

			item := setupManagedPGUR("")

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			username := pgurRolePrefix + Login0Suffix
			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, username, string(workSec.Data[PasswordSecretKey]),
				pgec, v1alpha1.PrimaryConnectionType,
			)

			// Get current secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.Privileges[0].GeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).To(Succeed())

			// Update privilege for a bouncer one
			item.Spec.Privileges[0].ConnectionType = v1alpha1.BouncerConnectionType
			// Save
			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.Privileges[0].GeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					// Check if sec have been updated
					if string(sec.Data["POSTGRES_URL"]) == string(sec2.Data["POSTGRES_URL"]) {
						return errors.New("Secret not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

				// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, username, string(workSec.Data[PasswordSecretKey]),
				pgec, v1alpha1.BouncerConnectionType,
			)
		})

		It("should be ok to create a bouncer user role and change it to a primary one", func() {
			// Setup pgec
			pgec, _ := setupPGECWithBouncer("30s", false)
			// Create pgdb
			setupPGDB(false)

			item := setupManagedPGURWithBouncer("")

			// Checks
			Expect(item.Status.Ready).To(BeTrue())
			Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserRoleCreatedPhase))

			username := pgurRolePrefix + Login0Suffix
			// Get work secret
			workSec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.WorkGeneratedSecretName,
				Namespace: pgurNamespace,
			}, workSec)).Should(Succeed())

			// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, username, string(workSec.Data[PasswordSecretKey]),
				pgec, v1alpha1.BouncerConnectionType,
			)

			// Get current secret
			sec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      item.Spec.Privileges[0].GeneratedSecretName,
				Namespace: pgurNamespace,
			}, sec)).To(Succeed())

			// Update privilege for a bouncer one
			item.Spec.Privileges[0].ConnectionType = v1alpha1.PrimaryConnectionType
			// Save
			Expect(k8sClient.Update(ctx, item)).To(Succeed())

			sec2 := &corev1.Secret{}
			Eventually(
				func() error {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      item.Spec.Privileges[0].GeneratedSecretName,
						Namespace: pgurNamespace,
					}, sec2)
					// Check error
					if err != nil {
						return err
					}

					// Check if sec have been updated
					if string(sec.Data["POSTGRES_URL"]) == string(sec2.Data["POSTGRES_URL"]) {
						return errors.New("Secret not updated")
					}

					return nil
				},
				generalEventuallyTimeout,
				generalEventuallyInterval,
			).
				Should(Succeed())

				// Validate
			checkPGURSecretValues(
				item.Spec.Privileges[0].GeneratedSecretName,
				pgurNamespace, pgdbDBName, username, string(workSec.Data[PasswordSecretKey]),
				pgec, v1alpha1.PrimaryConnectionType,
			)
		})
	})
})
