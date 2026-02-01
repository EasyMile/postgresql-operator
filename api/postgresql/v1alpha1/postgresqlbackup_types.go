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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/easymile/postgresql-operator/api/postgresql/common"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresqlBackupSpec defines the desired state of PostgresqlBackup.
type PostgresqlBackupSpec struct {
	// Schedule for cronjob
	Schedule string `json:"schedule"`
	// One shot backup ?
	OneShot bool `json:"oneShot"`
	// Postgresql Database
	// +required
	// +kubebuilder:validation:Required
	Database *common.CRLink `json:"database"`
	// Postgresql backup provider
	// +required
	// +kubebuilder:validation:Required
	BackupProvider *common.CRLink `json:"backupProvider"`
}

type BackupStatusPhase string

const (
	BackupNoPhase    BackupStatusPhase = ""
	BackupErrorPhase BackupStatusPhase = "Error"
	BackupValidPhase BackupStatusPhase = "Valid"
)

// PostgresqlBackupStatus defines the observed state of PostgresqlBackup.
type PostgresqlBackupStatus struct {
	// Current phase of the operator
	Phase BackupProviderStatusPhase `json:"phase,omitempty"`
	// Human-readable message indicating details about current operator phase or error.
	// +optional
	Message string `json:"message,omitempty"`
	// True if all resources are in a ready state and all work is done.
	// +optional
	Ready bool `json:"ready,omitempty"`
	// Resource Spec hash
	Hash string `json:"hash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PostgresqlBackup is the Schema for the postgresqlbackups API.
type PostgresqlBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlBackupSpec   `json:"spec,omitempty"`
	Status PostgresqlBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgresqlBackupList contains a list of PostgresqlBackup.
type PostgresqlBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlBackup{}, &PostgresqlBackupList{})
}
