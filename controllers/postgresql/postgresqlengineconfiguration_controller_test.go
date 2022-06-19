package postgresql

import (
	"errors"
	"fmt"

	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/apis/postgresql/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Postgresql Engine Configuration tests", func() {
	AfterEach(cleanupFunction)

	It("shouldn't accept input without any specs", func() {
		err := k8sClient.Create(ctx, &postgresqlv1alpha1.PostgresqlEngineConfiguration{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgecName,
				Namespace: pgecNamespace,
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
			"spec.host":       false,
			"spec.secretName": false,
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

	It("should fail to look a not found secret", func() {
		// Create pgec
		it := &postgresqlv1alpha1.PostgresqlEngineConfiguration{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgecName,
				Namespace: pgecNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlEngineConfigurationSpec{
				Provider:        "",
				Host:            "localhost",
				Port:            5432,
				URIArgs:         "sslmode=disable",
				DefaultDatabase: "postgres",
				SecretName:      pgecSecretName,
			},
		}

		// Create provider
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		updatedPgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get updated pgec
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, updatedPgec)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedPgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedPgec.Status.Ready).To(BeFalse())
		Expect(updatedPgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineFailedPhase))
		Expect(updatedPgec.Status.LastValidatedTime).To(BeEquivalentTo(""))
		Expect(updatedPgec.Status.Message).To(ContainSubstring(pgecSecretName))
		Expect(updatedPgec.Status.Message).To(ContainSubstring("not found"))
	})

	It("should fail to look a malformed secret (no username)", func() {
		// Create secret
		sec := &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgecSecretName,
				Namespace: pgecNamespace,
			},
			StringData: map[string]string{
				"MALFORMED": "MALFORMED",
			},
		}

		Expect(k8sClient.Create(ctx, sec)).To(Succeed())

		// Get secret to be sure
		Eventually(
			func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      sec.Name,
					Namespace: sec.Namespace,
				}, sec)
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Create pgec
		it := &postgresqlv1alpha1.PostgresqlEngineConfiguration{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgecName,
				Namespace: pgecNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlEngineConfigurationSpec{
				Provider:        "",
				Host:            "localhost",
				Port:            5432,
				URIArgs:         "sslmode=disable",
				DefaultDatabase: "postgres",
				SecretName:      pgecSecretName,
			},
		}

		// Create pgec
		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		updatedPgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get updated pgec
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, updatedPgec)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedPgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedPgec.Status.Ready).To(BeFalse())
		Expect(updatedPgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineFailedPhase))
		Expect(updatedPgec.Status.LastValidatedTime).To(BeEquivalentTo(""))
		Expect(updatedPgec.Status.Message).To(BeEquivalentTo(`secret ` + pgecSecretName + ` must contain "user" and "password" values`))
	})

	It("should fail to look a malformed secret (no password)", func() {
		// Create secret
		sec := &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgecSecretName,
				Namespace: pgecNamespace,
			},
			StringData: map[string]string{
				"user":      "postgres",
				"MALFORMED": "MALFORMED",
			},
		}

		Expect(k8sClient.Create(ctx, sec)).To(Succeed())

		// Get secret to be sure
		Eventually(
			func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      sec.Name,
					Namespace: sec.Namespace,
				}, sec)
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Create pgec
		prov := &postgresqlv1alpha1.PostgresqlEngineConfiguration{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgecName,
				Namespace: pgecNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlEngineConfigurationSpec{
				Provider:        "",
				Host:            "localhost",
				Port:            5432,
				URIArgs:         "sslmode=disable",
				DefaultDatabase: "postgres",
				SecretName:      pgecSecretName,
			},
		}

		// Create pgec
		Expect(k8sClient.Create(ctx, prov)).Should(Succeed())

		updatedPgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get updated pgec
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, updatedPgec)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedPgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedPgec.Status.Ready).To(BeFalse())
		Expect(updatedPgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineFailedPhase))
		Expect(updatedPgec.Status.LastValidatedTime).To(BeEquivalentTo(""))
		Expect(updatedPgec.Status.Message).To(BeEquivalentTo(`secret ` + pgecSecretName + ` must contain "user" and "password" values`))
	})

	It("should be ok to set default values", func() {
		// Create secret
		sec := setupPGECSecret()

		// Get secret to be sure
		Eventually(
			func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      sec.Name,
					Namespace: sec.Namespace,
				}, sec)
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Create pgec
		prov := &postgresqlv1alpha1.PostgresqlEngineConfiguration{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgecName,
				Namespace: pgecNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlEngineConfigurationSpec{
				Provider:        "",
				Host:            "localhost",
				Port:            0,
				URIArgs:         "sslmode=disable",
				DefaultDatabase: "",
				CheckInterval:   "",
				SecretName:      pgecSecretName,
			},
		}

		// Create pgec
		Expect(k8sClient.Create(ctx, prov)).Should(Succeed())

		updatedPgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get updated pgec
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, updatedPgec)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedPgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedPgec.Status.Ready).To(BeTrue())
		Expect(updatedPgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineValidatedPhase))
		Expect(updatedPgec.Status.LastValidatedTime).NotTo(BeEquivalentTo(""))
		Expect(updatedPgec.Status.Message).To(BeEquivalentTo(""))
		Expect(updatedPgec.Spec.CheckInterval).To(BeEquivalentTo("30s"))
		Expect(updatedPgec.Spec.Port).To(BeEquivalentTo(5432))
		Expect(updatedPgec.Spec.DefaultDatabase).To(BeEquivalentTo("postgres"))
	})

	It("should be ok to set everything", func() {
		// Create pgec
		setupPGEC("10s", false)

		updatedPgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get updated pgec
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, updatedPgec)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedPgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedPgec.Status.Ready).To(BeTrue())
		Expect(updatedPgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineValidatedPhase))
		Expect(updatedPgec.Status.LastValidatedTime).NotTo(BeEquivalentTo(""))
	})

	It("should fail when pg instance cannot be reached", func() {
		// Create secret
		sec := setupPGECSecret()

		// Get secret to be sure
		Eventually(
			func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      sec.Name,
					Namespace: sec.Namespace,
				}, sec)
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Create pgec
		prov := &postgresqlv1alpha1.PostgresqlEngineConfiguration{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgecName,
				Namespace: pgecNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlEngineConfigurationSpec{
				Provider:   "",
				Host:       "cannotwork",
				URIArgs:    "sslmode=disable",
				SecretName: pgecSecretName,
			},
		}

		// Create pgec
		Expect(k8sClient.Create(ctx, prov)).Should(Succeed())

		updatedPgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get updated pgec
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, updatedPgec)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedPgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(updatedPgec.Status.Ready).To(BeFalse())
		Expect(updatedPgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineFailedPhase))
		Expect(updatedPgec.Status.LastValidatedTime).To(BeEquivalentTo(""))
	})

	It("should fail when secret is updated with wrong password", func() {
		// Create pgec
		_, sec := setupPGEC("10s", false)

		pgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get pgec
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
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(pgec.Status.Ready).To(BeTrue())
		Expect(pgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineValidatedPhase))

		// Update sec password
		sec.Data["password"] = []byte("cannotwork")

		// Update secret
		Expect(k8sClient.Update(ctx, sec)).NotTo(HaveOccurred())
		// Get pgec
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
				if pgec.Status.Phase == postgresqlv1alpha1.EngineValidatedPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(pgec.Status.Ready).To(BeFalse())
		Expect(pgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineFailedPhase))
		Expect(pgec.Status.LastValidatedTime).NotTo(BeEquivalentTo(""))
	})

	It("should fail when secret is updated with wrong user", func() {
		// Create pgec
		_, sec := setupPGEC("10s", false)

		pgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get pgec
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
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(pgec.Status.Ready).To(BeTrue())
		Expect(pgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineValidatedPhase))

		// Update sec user
		sec.Data["user"] = []byte("cannotwork")

		// Update secret
		Expect(k8sClient.Update(ctx, sec)).NotTo(HaveOccurred())
		// Get pgec
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
				if pgec.Status.Phase == postgresqlv1alpha1.EngineValidatedPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		// Checks
		Expect(pgec.Status.Ready).To(BeFalse())
		Expect(pgec.Status.Phase).To(BeEquivalentTo(postgresqlv1alpha1.EngineFailedPhase))
		Expect(pgec.Status.LastValidatedTime).NotTo(BeEquivalentTo(""))
	})

	It("should be ok to delete it without wait and nothing linked", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", false)

		updatedPgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get updated pgec
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, updatedPgec)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedPgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(k8sClient.Delete(ctx, prov)).NotTo(HaveOccurred())

		// Checks
		Expect(updatedPgec.Status.Ready).To(BeTrue())

		pgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		Eventually(
			func() error {
				// Get pgec to be sure
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, pgec)
				// Check if error isn't present
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

	It("should be ok to delete it with wait and nothing linked", func() {
		// Create pgec
		prov, _ := setupPGEC("10s", true)

		updatedPgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		// Get updated pgec
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, updatedPgec)
				// Check error
				if err != nil {
					return err
				}

				// Check if status hasn't been updated
				if updatedPgec.Status.Phase == postgresqlv1alpha1.EngineNoPhase {
					return errors.New("pgec hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).
			Should(Succeed())

		Expect(k8sClient.Delete(ctx, prov)).NotTo(HaveOccurred())

		// Checks
		Expect(updatedPgec.Status.Ready).To(BeTrue())

		pgec := &postgresqlv1alpha1.PostgresqlEngineConfiguration{}
		Eventually(
			func() error {
				// Get pgec to be sure
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgecName,
					Namespace: pgecNamespace,
				}, pgec)
				// Check if error isn't present
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
})
