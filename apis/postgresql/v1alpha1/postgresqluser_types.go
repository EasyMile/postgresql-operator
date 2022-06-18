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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type PrivilegesSpecEnum string

const OwnerPrivilege PrivilegesSpecEnum = "OWNER"
const ReaderPrivilege PrivilegesSpecEnum = "READER"
const WriterPrivilege PrivilegesSpecEnum = "WRITER"

// PostgresqlUserSpec defines the desired state of PostgresqlUser.
type PostgresqlUserSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// User role
	// +required
	// +kubebuilder:validation:Required
	RolePrefix string `json:"rolePrefix"`
	// Postgresql Database
	// +required
	// +kubebuilder:validation:Required
	Database *CRLink `json:"database"`
	// Generated secret name prefix
	// +required
	// +kubebuilder:validation:Required
	GeneratedSecretNamePrefix string `json:"generatedSecretNamePrefix"`
	// User privileges
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=OWNER;WRITER;READER
	Privileges PrivilegesSpecEnum `json:"privileges,omitempty"`
	// User password rotation duration
	// +optional
	UserPasswordRotationDuration string `json:"userPasswordRotationDuration,omitempty"`
}

type UserStatusPhase string

const UserNoPhase UserStatusPhase = ""
const UserFailedPhase UserStatusPhase = "Failed"
const UserCreatedPhase UserStatusPhase = "Created"

// PostgresqlUserStatus defines the observed state of PostgresqlUser.
type PostgresqlUserStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Current phase of the operator
	Phase UserStatusPhase `json:"phase"`
	// Human-readable message indicating details about current operator phase or error.
	// +optional
	Message string `json:"message"`
	// True if all resources are in a ready state and all work is done.
	// +optional
	Ready bool `json:"ready"`
	// User role used
	// +optional
	RolePrefix string `json:"rolePrefix"`
	// Postgres role for user
	// +optional
	PostgresRole string `json:"postgresRole"`
	// User login
	// +optional
	PostgresLogin string `json:"postgresLogin"`
	// Postgres group for user
	// +optional
	PostgresGroup string `json:"postgresGroup"`
	// Postgres database name for which user is created
	// +optional
	PostgresDatabaseName string `json:"postgresDatabaseName"`
	// Last password changed time
	// +optional
	LastPasswordChangedTime string `json:"lastPasswordChangedTime"`
}

//+kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=postgresqlusers,scope=Namespaced,shortName=pguser;pgu
// +kubebuilder:printcolumn:name="User role",type=string,description="Generated user role",JSONPath=".status.postgresRole"
// +kubebuilder:printcolumn:name="User group",type=string,description="User group",JSONPath=".status.postgresGroup"
// +kubebuilder:printcolumn:name="Database",type=string,description="Database",JSONPath=".status.postgresDatabaseName"
// +kubebuilder:printcolumn:name="Last Password Change",type=date,description="Last time the password was changed",JSONPath=".status.lastPasswordChangedTime"
// +kubebuilder:printcolumn:name="Phase",type=string,description="Status phase",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PostgresqlUser is the Schema for the postgresqlusers API.
type PostgresqlUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlUserSpec   `json:"spec,omitempty"`
	Status PostgresqlUserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgresqlUserList contains a list of PostgresqlUser.
type PostgresqlUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlUser{}, &PostgresqlUserList{})
}
