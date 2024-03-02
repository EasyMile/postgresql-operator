/*
Copyright 2022.

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
	"github.com/easymile/postgresql-operator/api/postgresql/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type PrivilegesSpecEnum string

const OwnerPrivilege PrivilegesSpecEnum = "OWNER"
const ReaderPrivilege PrivilegesSpecEnum = "READER"
const WriterPrivilege PrivilegesSpecEnum = "WRITER"

type ConnectionTypesSpecEnum string

const PrimaryConnectionType ConnectionTypesSpecEnum = "PRIMARY"
const BouncerConnectionType ConnectionTypesSpecEnum = "BOUNCER"

type PostgresqlUserRolePrivilege struct {
	// User Connection type.
	// This is referring to the user connection type needed for this user.
	// +optional
	// +kubebuilder:default=PRIMARY
	// +kubebuilder:validation:Enum=PRIMARY;BOUNCER
	ConnectionType ConnectionTypesSpecEnum `json:"connectionType,omitempty"`
	// User privileges
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=OWNER;WRITER;READER
	Privilege PrivilegesSpecEnum `json:"privilege"`
	// Postgresql Database
	// +required
	// +kubebuilder:validation:Required
	Database *common.CRLink `json:"database"`
	// Generated secret name prefix
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	GeneratedSecretName string `json:"generatedSecretName"`
}

type ModeEnum string

const ProvidedMode ModeEnum = "PROVIDED"
const ManagedMode ModeEnum = "MANAGED"

// PostgresqlUserRoleSpec defines the desired state of PostgresqlUserRole.
type PostgresqlUserRoleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// User mode
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=PROVIDED;MANAGED
	Mode ModeEnum `json:"mode,omitempty"`
	// Privileges
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Min=1
	Privileges []*PostgresqlUserRolePrivilege `json:"privileges"`
	// User role prefix
	// +optional
	RolePrefix string `json:"rolePrefix,omitempty"`
	// User password rotation duration
	// +optional
	UserPasswordRotationDuration string `json:"userPasswordRotationDuration,omitempty"`
	// Simple user password tuple generated secret name
	// +optional
	WorkGeneratedSecretName string `json:"workGeneratedSecretName"`
	// Import secret name
	// +optional
	ImportSecretName string `json:"importSecretName,omitempty"`
}

type UserRoleStatusPhase string

const UserRoleNoPhase UserRoleStatusPhase = ""
const UserRoleFailedPhase UserRoleStatusPhase = "Failed"
const UserRoleCreatedPhase UserRoleStatusPhase = "Created"

// PostgresqlUserRoleStatus defines the observed state of PostgresqlUserRole.
type PostgresqlUserRoleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Current phase of the operator
	Phase UserRoleStatusPhase `json:"phase"`
	// Human-readable message indicating details about current operator phase or error.
	// +optional
	Message string `json:"message"`
	// True if all resources are in a ready state and all work is done.
	// +optional
	Ready bool `json:"ready"`
	// User role
	// +optional
	RolePrefix string `json:"roleName"`
	// Postgres role for user
	// +optional
	PostgresRole string `json:"postgresRole"`
	// Postgres old roles to cleanup
	// +optional
	OldPostgresRoles []string `json:"oldPostgresRoles"`
	// Last password changed time
	// +optional
	LastPasswordChangedTime string `json:"lastPasswordChangedTime"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=postgresqluserroles,scope=Namespaced,shortName=pguserrole;pgur
//+kubebuilder:printcolumn:name="User role",type=string,description="User role",JSONPath=".status.postgresRole"
//+kubebuilder:printcolumn:name="Last Password Change",type=date,description="Last time the password was changed",JSONPath=".status.lastPasswordChangedTime"
//+kubebuilder:printcolumn:name="Phase",type=string,description="Status phase",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PostgresqlUserRole is the Schema for the postgresqluserroles API.
type PostgresqlUserRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlUserRoleSpec   `json:"spec,omitempty"`
	Status PostgresqlUserRoleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgresqlUserRoleList contains a list of PostgresqlUserRole.
type PostgresqlUserRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlUserRole `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlUserRole{}, &PostgresqlUserRoleList{})
}
