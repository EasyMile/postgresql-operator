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

package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresqlBackupProviderSpec defines the desired state of PostgresqlBackupProvider.
type PostgresqlBackupProviderSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Cron job spec for backup runs
	// +required
	CronJobSpec *batchv1.CronJobSpec `json:"cronJobSpec,omitempty"`
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// Generated name prefix used to populate secret pg_dump information and to create cronjobs.
	// Must be limited to 20 characters.
	// +optional
	GeneratedNamePrefix string `json:"generatedNamePrefix,omitempty"`
	// Wait for linked resource to be deleted
	// +optional
	WaitLinkedResourcesDeletion bool `json:"waitLinkedResourcesDeletion,omitempty"`
}

type BackupProviderStatusPhase string

const (
	BackupProviderNoPhase    BackupProviderStatusPhase = ""
	BackupProviderErrorPhase BackupProviderStatusPhase = "Error"
	BackupProviderValidPhase BackupProviderStatusPhase = "Valid"
)

// PostgresqlBackupProviderStatus defines the observed state of PostgresqlBackupProvider.
type PostgresqlBackupProviderStatus struct {
	// Current phase of the operator
	Phase BackupProviderStatusPhase `json:"phase,omitempty"`
	// Human-readable message indicating details about current operator phase or error.
	// +optional
	Message string `json:"message,omitempty"`
	// True if all resources are in a ready state and all work is done.
	// +optional
	Ready bool `json:"ready,omitempty"`
	// Generated secret name
	GeneratedSecretNamePrefix string `json:"generatedSecretNamePrefix,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PostgresqlBackupProvider is the Schema for the postgresqlbackupproviders API.
type PostgresqlBackupProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlBackupProviderSpec   `json:"spec,omitempty"`
	Status PostgresqlBackupProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgresqlBackupProviderList contains a list of PostgresqlBackupProvider.
type PostgresqlBackupProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlBackupProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlBackupProvider{}, &PostgresqlBackupProviderList{})
}
