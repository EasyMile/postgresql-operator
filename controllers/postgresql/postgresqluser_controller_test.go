package postgresql

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/easymile/postgresql-operator/apis/postgresql/common"
	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/apis/postgresql/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("PostgresqlUser tests", func() {
	AfterEach(cleanupFunction)

	It("shouldn't accept input without any specs", func() {
		err := k8sClient.Create(ctx, &postgresqlv1alpha1.PostgresqlUser{
			ObjectMeta: v1.ObjectMeta{
				Name:      pguName,
				Namespace: pguNamespace,
			},
		})

		Expect(err).To(HaveOccurred())

		// Cast error
		stErr, ok := err.(*apimachineryErrors.StatusError)

		Expect(ok).To(BeTrue())

		// Check that content is correct
		causes := stErr.Status().Details.Causes

		Expect(causes).To(HaveLen(4))

		// Search all fields
		fields := map[string]bool{
			"spec.rolePrefix":                false,
			"spec.database":                  false,
			"spec.generatedSecretNamePrefix": false,
			"spec.privileges":                false,
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

	It("should fail to look a not found pgdb", func() {
		it := &postgresqlv1alpha1.PostgresqlUser{
			ObjectMeta: v1.ObjectMeta{
				Name:      pguName,
				Namespace: pguNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlUserSpec{
				RolePrefix: "pgu",
				Database: &common.CRLink{
					Name:      "fake",
					Namespace: "fake",
				},
				GeneratedSecretNamePrefix: "pgu",
				Privileges:                postgresqlv1alpha1.OwnerPrivilege,
			},
		}

		// Create user
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlUser{}
		// Get updated user
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
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserFailedPhase))
		Expect(item.Status.Message).To(ContainSubstring("\"fake\" not found"))
	})

	It("should be ok to set only required values", func() {
		// Setup pgec
		pgec, _ := setupPGEC("10s", false)

		// Setup pgdb
		pgdb := setupPGDB(true)

		it := &postgresqlv1alpha1.PostgresqlUser{
			ObjectMeta: v1.ObjectMeta{
				Name:      pguName,
				Namespace: pguNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlUserSpec{
				RolePrefix: "pgu",
				Database: &common.CRLink{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				},
				GeneratedSecretNamePrefix: "pgu",
				Privileges:                postgresqlv1alpha1.OwnerPrivilege,
			},
		}

		// Create user
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlUser{}
		// Get updated user
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
					return errors.New("pgu hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserCreatedPhase))
		Expect(item.Status.Message).To(BeEmpty())
		Expect(item.Status.RolePrefix).To(Equal("pgu"))
		Expect(item.Status.PostgresLogin).To(ContainSubstring("pgu-"))
		Expect(item.Status.PostgresRole).To(ContainSubstring("pgu-"))
		Expect(item.Status.PostgresGroup).To(Equal(pgdb.Status.Roles.Owner))
		Expect(item.Status.PostgresDatabaseName).To(Equal(pgdb.Status.Database))
		Expect(item.Status.LastPasswordChangedTime).ToNot(BeEmpty())

		// Check if user exists
		postgresRoleExists, postgresRoleErr := isSQLRoleExists(item.Status.PostgresRole)
		Expect(postgresRoleErr).ToNot(HaveOccurred())
		Expect(postgresRoleExists).To(BeTrue())

		// Check user is in correct group
		isMember, err := isSQLUserMemberOf(item.Status.PostgresRole, pgdb.Status.Roles.Owner)
		Expect(err).ToNot(HaveOccurred())
		Expect(isMember).To(BeTrue())

		// Check secret values
		secretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, pguName)
		secretNamespace := pguNamespace
		checkPGUSecretValues(secretName, secretNamespace, "pgu", pgec)
	})

	It("should be ok to set all values (required & optional)", func() {
		// Setup pgec
		pgec, _ := setupPGEC("10s", false)

		// Setup pgdb
		pgdb := setupPGDB(true)

		it := &postgresqlv1alpha1.PostgresqlUser{
			ObjectMeta: v1.ObjectMeta{
				Name:      pguName,
				Namespace: pguNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlUserSpec{
				RolePrefix: "pgu",
				Database: &common.CRLink{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				},
				GeneratedSecretNamePrefix:    "pgu",
				Privileges:                   postgresqlv1alpha1.OwnerPrivilege,
				UserPasswordRotationDuration: "24h",
			},
		}

		// Create user
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlUser{}
		// Get updated user
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
					return errors.New("pgu hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.UserCreatedPhase))
		Expect(item.Status.Message).To(BeEmpty())
		Expect(item.Status.RolePrefix).To(Equal("pgu"))
		Expect(item.Status.PostgresLogin).To(ContainSubstring("pgu-"))
		Expect(item.Status.PostgresRole).To(ContainSubstring("pgu-"))
		Expect(item.Status.PostgresGroup).To(Equal(pgdb.Status.Roles.Owner))
		Expect(item.Status.PostgresDatabaseName).To(Equal(pgdb.Status.Database))
		Expect(item.Status.LastPasswordChangedTime).ToNot(BeEmpty())

		// Check if user exists
		postgresRoleExists, postgresRoleErr := isSQLRoleExists(item.Status.PostgresRole)
		Expect(postgresRoleErr).ToNot(HaveOccurred())
		Expect(postgresRoleExists).To(BeTrue())

		// Check user is in correct group
		isMember, err := isSQLUserMemberOf(item.Status.PostgresRole, pgdb.Status.Roles.Owner)
		Expect(err).ToNot(HaveOccurred())
		Expect(isMember).To(BeTrue())

		// Check secret values
		secretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, pguName)
		secretNamespace := pguNamespace
		checkPGUSecretValues(secretName, secretNamespace, "pgu", pgec)
	})

	It("should be ok to change role prefix", func() {
		// Setup pgec
		pgec, _ := setupPGEC("10s", false)

		// Setup pgdb
		pgdb := setupPGDB(true)

		// Setup PGU
		item := setupPGU()

		// Change role prefix
		pguNewRolePrefix := "pgubis"
		item.Spec.RolePrefix = pguNewRolePrefix

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		updatedItem := &postgresqlv1alpha1.PostgresqlUser{}
		// Get updated user
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguName,
					Namespace: pguNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedItem.Status.RolePrefix != pguNewRolePrefix {
					return errors.New("pgu hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.UserCreatedPhase))
		Expect(updatedItem.Status.Message).To(BeEmpty())
		Expect(updatedItem.Status.RolePrefix).To(Equal(pguNewRolePrefix))
		Expect(updatedItem.Status.PostgresLogin).To(ContainSubstring(fmt.Sprintf("%s-", pguNewRolePrefix)))
		Expect(updatedItem.Status.PostgresRole).To(ContainSubstring(fmt.Sprintf("%s-", pguNewRolePrefix)))

		// Check if user exists
		exists, err := isSQLRoleExists(updatedItem.Status.PostgresRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		// Check user is in correct group
		isMember, err := isSQLUserMemberOf(updatedItem.Status.PostgresRole, pgdb.Status.Roles.Owner)
		Expect(err).ToNot(HaveOccurred())
		Expect(isMember).To(BeTrue())

		// Check previous user does not exist anymore
		exists, err = isSQLRoleExists(item.Status.PostgresRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())

		// Check secret values
		secretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, pguName)
		secretNamespace := pguNamespace
		checkPGUSecretValues(secretName, secretNamespace, pguNewRolePrefix, pgec)
	})

	It("should be ok to change privileges (OWNER -> READ)", func() {
		// Setup pgec
		pgec, _ := setupPGEC("10s", false)

		// Setup pgdb
		pgdb := setupPGDB(true)

		// Setup PGU
		item := setupPGU()

		// Change role prefix
		item.Spec.Privileges = postgresqlv1alpha1.ReaderPrivilege

		Expect(k8sClient.Update(ctx, item)).Should(Succeed())

		updatedItem := &postgresqlv1alpha1.PostgresqlUser{}
		// Get updated user
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguName,
					Namespace: pguNamespace,
				}, updatedItem)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedItem.Status.PostgresGroup != pgdb.Status.Roles.Reader {
					return errors.New("pgu hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedItem.Status.Ready).To(BeTrue())
		Expect(updatedItem.Status.Phase).To(Equal(postgresqlv1alpha1.UserCreatedPhase))
		Expect(updatedItem.Status.Message).To(BeEmpty())
		Expect(updatedItem.Status.PostgresGroup).To(Equal(pgdb.Status.Roles.Reader))

		// Check user is in correct group
		isMember, err := isSQLUserMemberOf(item.Status.PostgresRole, pgdb.Status.Roles.Reader)
		Expect(err).ToNot(HaveOccurred())
		Expect(isMember).To(BeTrue())

		// Check secret values
		secretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, pguName)
		secretNamespace := pguNamespace
		checkPGUSecretValues(secretName, secretNamespace, "pgu", pgec)
	})

	It("should be ok to regenerate a secret that have been removed", func() {
		// Setup pgec
		pgec, _ := setupPGEC("10s", false)

		// Setup pgdb
		setupPGDB(true)

		// Setup PGU
		item := setupPGU()

		// Retrieve secret controlled by pgu
		pguSecretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, item.Name)
		pguSecretNamespace := item.Namespace
		secret := &corev1.Secret{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      pguSecretName,
			Namespace: pguSecretNamespace},
			secret)
		Expect(err).ToNot(HaveOccurred())

		// Delete secret
		Expect(k8sClient.Delete(ctx, secret)).Should(Succeed())
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguSecretName,
					Namespace: pguSecretNamespace,
				}, secret)

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

		// Retrieve renewed secret
		renewedSecret := &corev1.Secret{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguSecretName,
					Namespace: pguSecretNamespace,
				}, renewedSecret)

				// Check error
				if err != nil {
					return err
				}

				// Check if secret has not been recreated
				if apimachineryErrors.IsNotFound(err) {
					return errors.New("secret has not been recreated")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Check creation times
		Expect(secret.CreationTimestamp.Time.Before(renewedSecret.CreationTimestamp.Time)).To(BeTrue())

		// Check password related fields have changed
		Expect(secret.Data["POSTGRES_URL"]).ToNot(Equal(renewedSecret.Data["POSTGRES_URL"]))
		Expect(secret.Data["POSTGRES_URL_ARGS"]).ToNot(Equal(renewedSecret.Data["POSTGRES_URL_ARGS"]))
		Expect(secret.Data["PASSWORD"]).ToNot(Equal(renewedSecret.Data["PASSWORD"]))

		// Check all secret values
		secretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, pguName)
		secretNamespace := pguNamespace
		checkPGUSecretValues(secretName, secretNamespace, "pgu", pgec)

	})

	It("should be ok to regenerate a secret that have been edited (key removed)", func() {
		// Setup pgec
		pgec, _ := setupPGEC("10s", false)

		// Setup pgdb
		setupPGDB(true)

		// Setup PGU
		item := setupPGU()

		// Retrieve secret controller by pgu
		pguSecretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, item.Name)
		pguSecretNamespace := item.Namespace
		secret := &corev1.Secret{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      pguSecretName,
			Namespace: pguSecretNamespace},
			secret)
		Expect(err).ToNot(HaveOccurred())

		// Remove PASSWORD key from secret and update it
		keyToRemove := "PASSWORD"
		delete(secret.Data, keyToRemove)

		Expect(k8sClient.Update(ctx, secret)).Should(Succeed())

		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguSecretName,
					Namespace: pguSecretNamespace,
				}, secret)
				// Check error
				if err != nil {
					return err
				}

				// Check if secret hasn't been updated
				if secret.Data[keyToRemove] != nil {
					return errors.New("secret hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Then secret should be renewed by operator
		renewedSecret := &corev1.Secret{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguSecretName,
					Namespace: pguSecretNamespace,
				}, renewedSecret)
				// Check error
				if err != nil {
					return err
				}

				// Check if secret hasn't been updated
				if _, found := renewedSecret.Data[keyToRemove]; !found {
					return errors.New("secret hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Removed key should have been recreated
		Expect(renewedSecret.Data[keyToRemove]).ToNot(BeEmpty())

		// Check password related fields have changed
		Expect(secret.Data["POSTGRES_URL"]).ToNot(Equal(renewedSecret.Data["POSTGRES_URL"]))
		Expect(secret.Data["POSTGRES_URL_ARGS"]).ToNot(Equal(renewedSecret.Data["POSTGRES_URL_ARGS"]))
		Expect(secret.Data["PASSWORD"]).ToNot(Equal(renewedSecret.Data["PASSWORD"]))

		// Check all secret values
		secretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, pguName)
		secretNamespace := pguNamespace
		checkPGUSecretValues(secretName, secretNamespace, "pgu", pgec)
	})

	It("should be ok to regenerate a secret that been edited (known field edited)", func() {
		// Setup pgec
		pgec, _ := setupPGEC("10s", false)

		// Setup pgdb
		setupPGDB(true)

		item := setupPGU()

		// Retrieve secret controller by pgu
		pguSecretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, item.Name)
		pguSecretNamespace := item.Namespace
		secret := &corev1.Secret{}

		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      pguSecretName,
			Namespace: pguSecretNamespace},
			secret)
		Expect(err).ToNot(HaveOccurred())

		// Modify PASSWORD value from secret and update it
		fieldToModify := "ROLE"
		fieldNewValue := []byte("superman")
		fieldOldValue := secret.Data[fieldToModify]

		secret.Data[fieldToModify] = fieldNewValue

		Expect(k8sClient.Update(ctx, secret)).Should(Succeed())

		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguSecretName,
					Namespace: pguSecretNamespace,
				}, secret)
				// Check error
				if err != nil {
					return err
				}

				// Check if field has been updated in map
				if bytes.Compare(secret.Data[fieldToModify], fieldNewValue) != 0 {
					return errors.New("secret hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Secret should now get re-updated by controller
		renewedSecret := &corev1.Secret{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguSecretName,
					Namespace: pguSecretNamespace,
				}, renewedSecret)
				// Check error
				if err != nil {
					return err
				}

				// Check if field has been re-updated with old value
				if bytes.Compare(renewedSecret.Data[fieldToModify], fieldOldValue) != 0 {
					return errors.New("secret hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Check password related fields have changed
		Expect(secret.Data["POSTGRES_URL"]).ToNot(Equal(renewedSecret.Data["POSTGRES_URL"]))
		Expect(secret.Data["POSTGRES_URL_ARGS"]).ToNot(Equal(renewedSecret.Data["POSTGRES_URL_ARGS"]))
		Expect(secret.Data["PASSWORD"]).ToNot(Equal(renewedSecret.Data["PASSWORD"]))

		// Check all secret values
		secretName := fmt.Sprintf("%s-%s", item.Spec.GeneratedSecretNamePrefix, pguName)
		secretNamespace := pguNamespace
		checkPGUSecretValues(secretName, secretNamespace, "pgu", pgec)
	})

	It("should be ok to remove a user", func() {
		// Setup pgec
		setupPGEC("10s", false)

		// Setup pgdb
		pgdb := setupPGDB(true)

		// Setup pgu
		item := setupPGU()

		// Then delete it
		Expect(k8sClient.Delete(ctx, item)).Should(Succeed())

		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pguName,
					Namespace: pguNamespace,
				}, item)

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

		// Check if user does not exist anymore
		postgresRoleExists, postgresRoleErr := isSQLRoleExists(item.Status.PostgresRole)
		Expect(postgresRoleErr).ToNot(HaveOccurred())
		Expect(postgresRoleExists).To(BeFalse())

		// Check user is not in group anymore
		_, err := isSQLUserMemberOf(item.Status.PostgresRole, pgdb.Status.Roles.Owner)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("does not exist"))
	})
})
