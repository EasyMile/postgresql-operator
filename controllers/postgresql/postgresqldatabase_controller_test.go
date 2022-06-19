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

var _ = Describe("Postgresql Engine Configuration tests", func() {
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
})
