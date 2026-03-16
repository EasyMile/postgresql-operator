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
	"strconv"

	"k8s.io/apimachinery/pkg/types"

	//nolint:revive
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimachineryErrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/easymile/postgresql-operator/api/postgresql/common"
	postgresqlv1alpha1 "github.com/easymile/postgresql-operator/api/postgresql/v1alpha1"
)

var _ = Describe("PostgresqlBackup Controller", func() {
	AfterEach(cleanupFunction)

	It("shouldn't accept input without any specs", func() {
		err := k8sClient.Create(ctx, &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
		})

		Expect(err).To(HaveOccurred())

		stErr, ok := err.(*apimachineryErrors.StatusError)
		Expect(ok).To(BeTrue())

		causes := stErr.Status().Details.Causes
		Expect(causes).To(HaveLen(2))

		fields := map[string]bool{
			"spec.database":       false,
			"spec.backupProvider": false,
		}

		for _, cause := range causes {
			fields[cause.Field] = true
		}

		for key, value := range fields {
			if !value {
				Expect(fmt.Errorf("%s should be found in error causes", key)).ToNot(HaveOccurred())
			}
		}
	})

	It("should fail when database is not found", func() {
		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      "fake-database",
					Namespace: pgdbNamespace,
				},
				BackupProvider: &common.CRLink{
					Name:      "fake-provider",
					Namespace: pgbNamespace,
				},
			},
		}

		item = waitForBackupPhase(item)

		Expect(item.Status.Ready).To(BeFalse())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.BackupErrorPhase))
		Expect(item.Status.Message).To(ContainSubstring("fake-database"))
	})

	It("should fail when backup provider is not found", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      "fake-provider",
					Namespace: pgbNamespace,
				},
			},
		}

		item = waitForBackupPhase(item)

		Expect(item.Status.Ready).To(BeFalse())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.BackupErrorPhase))
		Expect(item.Status.Message).To(ContainSubstring("fake-provider"))
	})

	It("should not transition when database is not ready", func() {
		// Create a database pointing to a non-existent PGEC so it enters error state.
		badDB := &postgresqlv1alpha1.PostgresqlDatabase{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgdbName,
				Namespace: pgdbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlDatabaseSpec{
				Database: pgdbDBName,
				EngineConfiguration: &common.CRLink{
					Name:      "nonexistent-pgec",
					Namespace: pgecNamespace,
				},
			},
		}

		Expect(k8sClient.Create(ctx, badDB)).Should(Succeed())

		// Wait for the database to be in error state (not ready).
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgdbName,
					Namespace: pgdbNamespace,
				}, badDB)
				if err != nil {
					return err
				}

				if badDB.Status.Phase == postgresqlv1alpha1.DatabaseNoPhase {
					return errors.New("pgdb hasn't been updated by operator")
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Expect(badDB.Status.Ready).To(BeFalse())

		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "")

		it := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      badDB.Name,
					Namespace: badDB.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		Expect(k8sClient.Create(ctx, it)).Should(Succeed())

		// The backup controller should stay in NoPhase while the database is not ready.
		Consistently(
			func() postgresqlv1alpha1.BackupStatusPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbName,
					Namespace: pgbNamespace,
				}, it)
				if err != nil {
					return postgresqlv1alpha1.BackupErrorPhase
				}

				return it.Status.Phase
			},
			"10s",
			"1s",
		).Should(Equal(postgresqlv1alpha1.BackupNoPhase))
	})

	It("should succeed with required fields only", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)

		Expect(item.Status.Ready).To(BeTrue())
		Expect(item.Status.Phase).To(Equal(postgresqlv1alpha1.BackupValidPhase))
		Expect(item.Status.Message).To(BeEmpty())

		Expect(item.Status.GeneratedName).To(ContainSubstring(pgbName))
		Expect(item.Status.GeneratedName).To(ContainSubstring(pgbNamespace))
	})

	It("should generate the resource name with a prefix from backup provider", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		prefix := "bkp-"
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, prefix)

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)

		Expect(item.Status.Ready).To(BeTrue())

		Expect(item.Status.GeneratedName).To(ContainSubstring(pgbName))
		Expect(item.Status.GeneratedName).To(ContainSubstring(pgbNamespace))
	})

	It("should create a pg_dump secret with the correct env vars", func() {
		pgec, _ := setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		secretName := item.Status.GeneratedName

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: pgbpNamespace,
		}, secret)).To(Succeed())

		Expect(string(secret.Data[pgDumpEnvHost])).To(Equal(pgec.Spec.Host))
		Expect(string(secret.Data[pgDumpEnvPort])).To(Equal(strconv.Itoa(pgec.Spec.Port)))
		Expect(string(secret.Data[pgDumpEnvUser])).To(Equal(postgresUser))
		Expect(string(secret.Data[pgDumpEnvPassword])).To(Equal(postgresPassword))
		Expect(string(secret.Data[pgDumpEnvDatabase])).To(Equal(pgdb.Status.Database))
	})

	It("should include PGSSLMODE in the secret when sslmode is present in pgec URIArgs", func() {
		setupPGEC("10s", false) // URIArgs = "sslmode=disable"
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, secret)).To(Succeed())

		Expect(string(secret.Data[pgDumpEnvSSLMode])).To(Equal("disable"))
	})

	It("should use the backup provider schedule when backup spec.schedule is empty", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "") // schedule = "*/5 * * * *"

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		cronJob := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, cronJob)).To(Succeed())

		Expect(cronJob.Spec.Schedule).To(Equal(pgbp.Spec.CronJobSpec.Schedule))
	})

	It("should override the cronjob schedule from backup spec.schedule", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "") // schedule = "*/5 * * * *"

		customSchedule := "0 3 * * *"

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Schedule: customSchedule,
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		cronJob := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, cronJob)).To(Succeed())

		Expect(cronJob.Spec.Schedule).To(Equal(customSchedule))
	})

	It("should inject the pg_dump secret into cronjob containers via EnvFrom", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		cronJob := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, cronJob)).To(Succeed())

		containers := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers
		Expect(containers).ToNot(BeEmpty())

		for _, container := range containers {
			Expect(hasEnvFromSecret(container, item.Status.GeneratedName)).To(
				BeTrue(),
				"container %q should have EnvFrom referencing secret %q", container.Name, item.Status.GeneratedName,
			)
		}
	})

	It("should inject the pg_dump secret into both init containers and containers", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		// makePgDumpCronJobSpec creates a spec with both init containers and containers
		pgbp := createPGBP(makePgDumpCronJobSpec(), nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		cronJob := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, cronJob)).To(Succeed())

		initContainers := cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers
		Expect(initContainers).ToNot(BeEmpty())

		for _, container := range initContainers {
			Expect(hasEnvFromSecret(container, item.Status.GeneratedName)).To(
				BeTrue(),
				"init container %q should have EnvFrom referencing secret %q", container.Name, item.Status.GeneratedName,
			)
		}

		containers := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers
		Expect(containers).ToNot(BeEmpty())

		for _, container := range containers {
			Expect(hasEnvFromSecret(container, item.Status.GeneratedName)).To(
				BeTrue(),
				"container %q should have EnvFrom referencing secret %q", container.Name, item.Status.GeneratedName,
			)
		}
	})

	It("should not inject the secret twice when containers already have the EnvFrom reference", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)

		cronJobSpec := makeCronJobSpec()
		pgbp := createPGBP(cronJobSpec, nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		cronJob := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, cronJob)).To(Succeed())

		// Each container should contain the secret reference exactly once.
		for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			count := 0
			for _, envFrom := range container.EnvFrom {
				if envFrom.SecretRef != nil && envFrom.SecretRef.Name == item.Status.GeneratedName {
					count++
				}
			}

			Expect(count).To(Equal(1),
				"container %q should reference secret exactly once, got %d", container.Name, count,
			)
		}
	})

	It("should apply labels from backup provider to the secret", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)

		labels := map[string]string{"app": "backup", "env": "test"}
		pgbp := createPGBP(makeCronJobSpec(), labels, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, secret)).To(Succeed())

		for k, v := range labels {
			Expect(secret.Labels).To(HaveKeyWithValue(k, v))
		}
	})

	It("should apply labels from backup provider to the cronjob", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)

		labels := map[string]string{"app": "backup", "env": "test"}
		pgbp := createPGBP(makeCronJobSpec(), labels, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		cronJob := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, cronJob)).To(Succeed())

		for k, v := range labels {
			Expect(cronJob.Labels).To(HaveKeyWithValue(k, v))
		}
	})

	It("should apply annotations from backup provider to the secret", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)

		annotations := map[string]string{"backup.io/type": "pg_dump", "backup.io/retention": "7d"}
		pgbp := createPGBP(makeCronJobSpec(), nil, annotations, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, secret)).To(Succeed())

		for k, v := range annotations {
			Expect(secret.Annotations).To(HaveKeyWithValue(k, v))
		}
	})

	It("should apply annotations from backup provider to the cronjob", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)

		annotations := map[string]string{"backup.io/type": "pg_dump", "backup.io/retention": "7d"}
		pgbp := createPGBP(makeCronJobSpec(), nil, annotations, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		cronJob := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      item.Status.GeneratedName,
			Namespace: pgbpNamespace,
		}, cronJob)).To(Succeed())

		for k, v := range annotations {
			Expect(cronJob.Annotations).To(HaveKeyWithValue(k, v))
		}
	})

	It("should update the cronjob schedule when backup spec.schedule changes", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Schedule: "*/5 * * * *",
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		// Change the schedule
		newSchedule := "0 4 * * *"
		item.Spec.Schedule = newSchedule
		Expect(k8sClient.Update(ctx, item)).To(Succeed())

		Eventually(
			func() error {
				cronJob := &batchv1.CronJob{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      item.Status.GeneratedName,
					Namespace: pgbpNamespace,
				}, cronJob)
				if err != nil {
					return err
				}

				if cronJob.Spec.Schedule != newSchedule {
					return fmt.Errorf("expected schedule %q, got %q", newSchedule, cronJob.Spec.Schedule)
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})

	It("should update cronjob labels when backup provider labels change and backup is reconciled", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)

		initialLabels := map[string]string{"version": "v1"}
		pgbp := createPGBP(makeCronJobSpec(), initialLabels, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		// Refresh pgbp
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pgbp.Name, Namespace: pgbp.Namespace}, pgbp)).To(Succeed())

		// Update the backup provider labels
		newLabels := map[string]string{"version": "v2"}
		pgbp.Spec.Labels = newLabels
		Expect(k8sClient.Update(ctx, pgbp)).To(Succeed())

		// Trigger a backup reconcile by touching an annotation on the backup
		item.Annotations = map[string]string{"reconcile-trigger": "1"}
		Expect(k8sClient.Update(ctx, item)).To(Succeed())

		Eventually(
			func() error {
				cronJob := &batchv1.CronJob{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      item.Status.GeneratedName,
					Namespace: pgbpNamespace,
				}, cronJob)
				if err != nil {
					return err
				}

				v, ok := cronJob.Labels["version"]
				if !ok || v != "v2" {
					return fmt.Errorf("expected label version=v2, got %v", cronJob.Labels)
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})

	It("should update secret annotations when backup provider annotations change and backup is reconciled", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)

		initialAnnotations := map[string]string{"retention": "7d"}
		pgbp := createPGBP(makeCronJobSpec(), nil, initialAnnotations, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		// Refresh pgbp
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pgbp.Name, Namespace: pgbp.Namespace}, pgbp)).To(Succeed())

		// Update backup provider annotations
		newAnnotations := map[string]string{"retention": "30d"}
		pgbp.Spec.Annotations = newAnnotations
		Expect(k8sClient.Update(ctx, pgbp)).To(Succeed())

		// Trigger a backup reconcile
		if item.Annotations == nil {
			item.Annotations = map[string]string{}
		}

		item.Annotations["reconcile-trigger"] = "1"
		Expect(k8sClient.Update(ctx, item)).To(Succeed())

		Eventually(
			func() error {
				secret := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      item.Status.GeneratedName,
					Namespace: pgbpNamespace,
				}, secret)
				if err != nil {
					return err
				}

				v, ok := secret.Annotations["retention"]
				if !ok || v != "30d" {
					return fmt.Errorf("expected annotation retention=30d, got %v", secret.Annotations)
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})

	It("should delete the backup successfully", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())
		generatedName := item.Status.GeneratedName
		Expect(generatedName).NotTo(BeEmpty())

		Expect(k8sClient.Delete(ctx, item)).To(Succeed())

		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbName,
					Namespace: pgbNamespace,
				}, &postgresqlv1alpha1.PostgresqlBackup{})

				if err == nil {
					return errors.New("backup should be deleted but still exists")
				}

				if !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		// Verify that the generated CronJob and Secret are deleted from the backup provider namespace.
		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      generatedName,
					Namespace: pgbpNamespace,
				}, &batchv1.CronJob{})
				if err == nil {
					return errors.New("generated CronJob should be deleted but still exists")
				}
				if !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())

		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      generatedName,
					Namespace: pgbpNamespace,
				}, &corev1.Secret{})
				if err == nil {
					return errors.New("generated Secret should be deleted but still exists")
				}
				if !apimachineryErrors.IsNotFound(err) {
					return err
				}

				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})

	It("should delete the backup successfully when backup provider has been deleted first", func() {
		setupPGEC("10s", false)
		pgdb := setupPGDB(false)
		pgbp := createPGBP(makeCronJobSpec(), nil, nil, "")

		item := &postgresqlv1alpha1.PostgresqlBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      pgbName,
				Namespace: pgbNamespace,
			},
			Spec: postgresqlv1alpha1.PostgresqlBackupSpec{
				Database: &common.CRLink{
					Name:      pgdb.Name,
					Namespace: pgdb.Namespace,
				},
				BackupProvider: &common.CRLink{
					Name:      pgbp.Name,
					Namespace: pgbp.Namespace,
				},
			},
		}

		item = waitForBackupPhase(item)
		Expect(item.Status.Ready).To(BeTrue())

		// Delete the backup provider first.
		Expect(deletePGBP(ctx, k8sClient, pgbpName, pgbpNamespace)).ToNot(HaveOccurred())

		// Now delete the backup CR — the controller must handle the missing provider gracefully.
		Expect(k8sClient.Delete(ctx, item)).To(Succeed())

		Eventually(
			func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      pgbName,
					Namespace: pgbNamespace,
				}, &postgresqlv1alpha1.PostgresqlBackup{})
				if err == nil {
					return errors.New("backup should be deleted but still exists")
				}
				if !apimachineryErrors.IsNotFound(err) {
					return err
				}
				return nil
			},
			generalEventuallyTimeout,
			generalEventuallyInterval,
		).Should(Succeed())
	})
})
