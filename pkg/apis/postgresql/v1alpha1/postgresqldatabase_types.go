package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresqlDatabaseSpec defines the desired state of PostgresqlDatabase
type PostgresqlDatabaseSpec struct {
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
const DatabaseFailedPhase DatabaseStatusPhase = "failed"
const DatabaseCreatedPhase DatabaseStatusPhase = "created"

// PostgresqlDatabaseStatus defines the observed state of PostgresqlDatabase
type PostgresqlDatabaseStatus struct {
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Current phase of the operator
	Phase DatabaseStatusPhase `json:"phase"`
	// Human-readable message indicating details about current operator phase or error.
	Message string `json:"message"`
	// True if all resources are in a ready state and all work is done.
	Ready bool `json:"ready"`
	// Already created roles for database
	// +optional
	Roles PostgresRoles `json:"roles"`
	// Already created schemas
	// +optional
	// +listType=set
	Schemas []string `json:"schemas,omitempty"`
	// Already extensions added
	// +optional
	// +listType=set
	Extensions []string `json:"extensions,omitempty"`
}

// PostgresRoles stores the different group roles already created for database
// +k8s:openapi-gen=true
type PostgresRoles struct {
	Owner  string `json:"owner"`
	Reader string `json:"reader"`
	Writer string `json:"writer"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlDatabase is the Schema for the postgresqldatabases API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=postgresqldatabases,scope=Namespaced
type PostgresqlDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlDatabaseSpec   `json:"spec,omitempty"`
	Status PostgresqlDatabaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlDatabaseList contains a list of PostgresqlDatabase
type PostgresqlDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlDatabase{}, &PostgresqlDatabaseList{})
}
