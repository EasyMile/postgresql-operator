/*
Copyright 2026.

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
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	//nolint:revive
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive
	. "github.com/onsi/gomega"

	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/easymile/postgresql-operator/api/postgresql/common"
	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
	"github.com/easymile/postgresql-operator/internal/controller/config"
)

var _ = Describe("PostgresqlBackupProvider Controller", func() {
	AfterEach(cleanupFunction)

	It("shouldn't accept input without any specs", func() {
		err := k8sClient.Create(ctx, &postgresqlv1alpha1.PostgresqlBackupProvider{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbpName,
				Namespace: pgbpNamespace,
			},
		})

		Expect(err).To(HaveOccurred())

		// Cast error
		stErr, ok := err.(*apimachineryErrors.StatusError)
		Expect(ok).To(BeTrue())

		// Check that content is correct
		causes := stErr.Status().Details.Causes
		Expect(causes).To(HaveLen(1))

		fields := map[string]bool{
			"spec.cronJobSpec": false,
		}

		for _, cause := range causes {
			fields[cause.Field] = true
		}

		for key, value := range fields {
			if !value {
				err := fmt.Errorf("%s found be found in error causes", key)
				Expect(err).ToNot(HaveOccurred())
			}
		}
	})

	It("should fail when generated name prefix is too long", func() {
		it := &postgresqlv1alpha1.PostgresqlBackupProvider{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbpName,
				Namespace: pgbpNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupProviderSpec{
				CronJobSpec:         makeCronJobSpec(),
				GeneratedNamePrefix: strings.Repeat("b", maxGeneratedNamePrefixLength+1),
			},
		}

		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlBackupProvider{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				}, item)
				if err != nil {
					return err
				}
				if item.Status.Phase == postgresqlv1alpha1.BackupProviderNoPhase {
					return errors.New("pgbp hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Expect(item.Status.Ready).To(BeFalse())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.BackupProviderErrorPhase))
		Expect(item.Status.Message).To(Equal("GeneratedNamePrefix length is greater than supported"))
	})

	It("should be ok to set required values and auto-generate prefix", func() {
		it := &postgresqlv1alpha1.PostgresqlBackupProvider{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbpName,
				Namespace: pgbpNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupProviderSpec{
				CronJobSpec: makeCronJobSpec(),
			},
		}

		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlBackupProvider{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				}, item)
				if err != nil {
					return err
				}
				if item.Status.Phase == postgresqlv1alpha1.BackupProviderNoPhase {
					return errors.New("pgbp hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.BackupProviderValidPhase))
		Expect(item.Status.Message).To(BeEmpty())
		Expect(controllerutil.ContainsFinalizer(item, config.Finalizer)).To(BeTrue())
		Expect(item.Spec.GeneratedNamePrefix).ToNot(BeEmpty())
		Expect(len(item.Spec.GeneratedNamePrefix)).To(BeNumerically("<=", maxGeneratedNamePrefixLength))
	})

	It("should block deletion when wait linked resources deletion is enabled and a backup exists", func() {
		provider := &postgresqlv1alpha1.PostgresqlBackupProvider{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbpName,
				Namespace: pgbpNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupProviderSpec{
				CronJobSpec:                 makeCronJobSpec(),
				GeneratedNamePrefix:         "prefix",
				WaitLinkedResourcesDeletion: true,
			},
		}
		Expect(k8sClient.Create(ctx, provider)).Should(Succeed())

		backup := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Schedule: "*/10 * * * *",
				Database: &common.CRLink{
					Name:      pgdbName,
					Namespace: pgbpNamespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

		backupItem := &postgresqlv1alpha1.PostgresqlBackup{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbName,
					Namespace: pgbNamespace,
				}, backupItem)
				if err != nil {
					return err
				}

				if backupItem.Status.Phase != postgresqlv1alpha1.BackupErrorPhase {
					return errors.New("backup not updated")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Expect(k8sClient.Delete(ctx, provider)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlBackupProvider{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				}, item)
				if err != nil {
					return err
				}
				if item.DeletionTimestamp.IsZero() {
					return errors.New("pgbp hasn't started deletion yet")
				}
				if !controllerutil.ContainsFinalizer(item, config.Finalizer) {
					return errors.New("finalizer removed before linked backup deletion")
				}
				if item.Status.Phase != postgresqlv1alpha1.BackupProviderErrorPhase {
					return errors.New("pgbp hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Expect(item.Status.Ready).To(BeFalse())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.BackupProviderErrorPhase))
		Expect(item.Status.Message).To(ContainSubstring("cannot remove resource because found backup"))
	})

	It("should block deletion when wait linked resources deletion is disabled and a backup exists", func() {
		provider := &postgresqlv1alpha1.PostgresqlBackupProvider{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbpName,
				Namespace: pgbpNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupProviderSpec{
				CronJobSpec:                 makeCronJobSpec(),
				GeneratedNamePrefix:         "prefix",
				WaitLinkedResourcesDeletion: false,
			},
		}
		Expect(k8sClient.Create(ctx, provider)).Should(Succeed())

		backup := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Schedule: "*/10 * * * *",
				Database: &common.CRLink{
					Name:      pgdbName,
					Namespace: pgbpNamespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, backup)).Should(Succeed())

		backupItem := &postgresqlv1alpha1.PostgresqlBackup{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbName,
					Namespace: pgbNamespace,
				}, backupItem)
				if err != nil {
					return err
				}

				if backupItem.Status.Phase != postgresqlv1alpha1.BackupErrorPhase {
					return errors.New("backup not updated")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Expect(k8sClient.Delete(ctx, provider)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlBackupProvider{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				}, item)
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}
				if err != nil && apimachineryErrors.IsNotFound(err) {
					return nil
				}
				if item.DeletionTimestamp.IsZero() {
					return errors.New("pgbp hasn't started deletion yet")
				}
				if !controllerutil.ContainsFinalizer(item, config.Finalizer) {
					return errors.New("finalizer removed before linked backup deletion")
				}

				return errors.New("not cleaned")
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})

	It("should block deletion when wait linked resources deletion is enabled and no backup exists", func() {
		provider := &postgresqlv1alpha1.PostgresqlBackupProvider{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbpName,
				Namespace: pgbpNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupProviderSpec{
				CronJobSpec:                 makeCronJobSpec(),
				GeneratedNamePrefix:         "prefix",
				WaitLinkedResourcesDeletion: true,
			},
		}
		Expect(k8sClient.Create(ctx, provider)).Should(Succeed())

		backupProviderItem := &postgresqlv1alpha1.PostgresqlBackupProvider{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				}, backupProviderItem)
				if err != nil {
					return err
				}

				if backupProviderItem.Status.Phase == postgresqlv1alpha1.BackupProviderNoPhase {
					return errors.New("backup not updated")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Expect(k8sClient.Delete(ctx, provider)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlBackupProvider{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				}, item)
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}
				if err != nil && apimachineryErrors.IsNotFound(err) {
					return nil
				}
				if item.DeletionTimestamp.IsZero() {
					return errors.New("pgbp hasn't started deletion yet")
				}
				if !controllerutil.ContainsFinalizer(item, config.Finalizer) {
					return errors.New("finalizer removed before linked backup deletion")
				}

				return errors.New("not cleaned")
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})

	It("should block deletion when wait linked resources deletion is disabled and no backup exists", func() {
		provider := &postgresqlv1alpha1.PostgresqlBackupProvider{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbpName,
				Namespace: pgbpNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupProviderSpec{
				CronJobSpec:                 makeCronJobSpec(),
				GeneratedNamePrefix:         "prefix",
				WaitLinkedResourcesDeletion: false,
			},
		}
		Expect(k8sClient.Create(ctx, provider)).Should(Succeed())

		backupProviderItem := &postgresqlv1alpha1.PostgresqlBackupProvider{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				}, backupProviderItem)
				if err != nil {
					return err
				}

				if backupProviderItem.Status.Phase == postgresqlv1alpha1.BackupProviderNoPhase {
					return errors.New("backup not updated")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Expect(k8sClient.Delete(ctx, provider)).Should(Succeed())

		item := &postgresqlv1alpha1.PostgresqlBackupProvider{}
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbpName,
					Namespace: pgbpNamespace,
				}, item)
				if err != nil && !apimachineryErrors.IsNotFound(err) {
					return err
				}
				if err != nil && apimachineryErrors.IsNotFound(err) {
					return nil
				}
				if item.DeletionTimestamp.IsZero() {
					return errors.New("pgbp hasn't started deletion yet")
				}
				if !controllerutil.ContainsFinalizer(item, config.Finalizer) {
					return errors.New("finalizer removed before linked backup deletion")
				}

				return errors.New("not cleaned")
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})
})
