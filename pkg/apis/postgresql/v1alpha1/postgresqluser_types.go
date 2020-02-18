package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresqlUserSpec defines the desired state of PostgresqlUser
type PostgresqlUserSpec struct {
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// User role
	// +required
	// +kubebuilder:validation:Required
	RolePrefix string `json:"rolePrefix"`
	// Postgresql Database
	// +required
	// +kubebuilder:validation:Required
	Database CRLink `json:"database"`
	// Generated secret name prefix
	// +required
	// +kubebuilder:validation:Required
	GeneratedSecretNamePrefix string `json:"generatedSecretNamePrefix"`
	// User privileges
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=OWNER;WRITE;READ
	Privileges string `json:"privileges,omitempty"`
	// User password rotation duration
	// +optional
	UserPasswordRotationDuration string `json:"userPasswordRotationDuration,omitempty"`
}

type UserStatusPhase string

const UserNoPhase UserStatusPhase = ""
const UserFailedPhase UserStatusPhase = "failed"
const UserCreatedPhase UserStatusPhase = "created"

// PostgresqlUserStatus defines the observed state of PostgresqlUser
type PostgresqlUserStatus struct {
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Current phase of the operator
	Phase UserStatusPhase `json:"phase"`
	// Human-readable message indicating details about current operator phase or error.
	Message string `json:"message"`
	// True if all resources are in a ready state and all work is done.
	Ready bool `json:"ready"`
	// User role used
	RolePrefix string `json:"rolePrefix"`
	// Postgres role for user
	PostgresRole string `json:"postgresRole"`
	// User login
	PostgresLogin string `json:"postgresLogin"`
	// Postgres group for user
	PostgresGroup string `json:"postgresGroup"`
	// Postgres database name for which user is created
	PostgresDatabaseName string `json:"postgresDatabaseName"`
	// Resource Spec hash
	Hash string `json:"hash"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlUser is the Schema for the postgresqlusers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=postgresqlusers,scope=Namespaced
type PostgresqlUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlUserSpec   `json:"spec,omitempty"`
	Status PostgresqlUserStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlUserList contains a list of PostgresqlUser
type PostgresqlUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlUser{}, &PostgresqlUserList{})
}
