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

// PostgresqlDatabaseSpec defines the desired state of PostgresqlDatabase.
type PostgresqlDatabaseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Database name
	// +required
	// +kubebuilder:validation:Required
	Database string `json:"database"`
	// Master role name will be used to create top group role.
	// Database owner and users will be in this group role.
	// +optional
	MasterRole string `json:"masterRole,omitempty"`
	// Should drop database on Custom Resource deletion ?
	// +optional
	DropOnDelete bool `json:"dropOnDelete,omitempty"`
	// Wait for linked resource to be deleted
	// +optional
	WaitLinkedResourcesDeletion bool `json:"waitLinkedResourcesDeletion,omitempty"`
	// Schema to create in database
	// +optional
	Schemas DatabaseModulesList `json:"schemas,omitempty"`
	// Extensions to enable
	// +optional
	Extensions DatabaseModulesList `json:"extensions,omitempty"`
	// Postgresql Engine Configuration link
	// +required
	// +kubebuilder:validation:Required
	EngineConfiguration *CRLink `json:"engineConfiguration"`
}

type DatabaseModulesList struct {
	// Modules list
	// +optional
	// +listType=set
	List []string `json:"list,omitempty"`
	// Should drop on delete ?
	// +optional
	DropOnOnDelete bool `json:"dropOnDelete,omitempty"`
	// Should drop with cascade ?
	// +optional
	DeleteWithCascade bool `json:"deleteWithCascade,omitempty"`
}

type DatabaseStatusPhase string

const DatabaseNoPhase DatabaseStatusPhase = ""
const DatabaseFailedPhase DatabaseStatusPhase = "Failed"
const DatabaseCreatedPhase DatabaseStatusPhase = "Created"

// PostgresqlDatabaseStatus defines the observed state of PostgresqlDatabase.
type PostgresqlDatabaseStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Current phase of the operator
	Phase DatabaseStatusPhase `json:"phase"`
	// Human-readable message indicating details about current operator phase or error.
	// +optional
	Message string `json:"message"`
	// True if all resources are in a ready state and all work is done.
	// +optional
	Ready bool `json:"ready"`
	// Created database
	// +optional
	Database string `json:"database"`
	// Already created roles for database
	// +optional
	Roles StatusPostgresRoles `json:"roles"`
	// Already created schemas
	// +optional
	// +listType=set
	Schemas []string `json:"schemas,omitempty"`
	// Already extensions added
	// +optional
	// +listType=set
	Extensions []string `json:"extensions,omitempty"`
}

// StatusPostgresRoles stores the different group roles already created for database
// +k8s:openapi-gen=true
type StatusPostgresRoles struct {
	Owner  string `json:"owner"`
	Reader string `json:"reader"`
	Writer string `json:"writer"`
}

//+kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=postgresqldatabases,scope=Namespaced,shortName=pgdb
// +kubebuilder:printcolumn:name="Database",type=string,description="Database name",JSONPath=".status.database"
// +kubebuilder:printcolumn:name="Schemas",type=string,description="Schemas",JSONPath=".status.schemas"
// +kubebuilder:printcolumn:name="Extensions",type=string,description="Extensions",JSONPath=".status.extensions"
// +kubebuilder:printcolumn:name="Phase",type=string,description="Status phase",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PostgresqlDatabase is the Schema for the postgresqldatabases API.
type PostgresqlDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlDatabaseSpec   `json:"spec,omitempty"`
	Status PostgresqlDatabaseStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgresqlDatabaseList contains a list of PostgresqlDatabase.
type PostgresqlDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlDatabase{}, &PostgresqlDatabaseList{})
}
