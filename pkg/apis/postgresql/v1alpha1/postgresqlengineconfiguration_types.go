package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ProviderType string

const NoProvider ProviderType = ""
const AWSProvider ProviderType = "AWS"
const AzureProvider ProviderType = "AZURE"

// PostgresqlEngineConfigurationSpec defines the desired state of PostgresqlEngineConfiguration
type PostgresqlEngineConfigurationSpec struct {
	// Provider
	// +kubebuilder:validation:Enum="";AWS;AZURE
	Provider ProviderType `json:"provider,omitempty"`
	// Hostname
	// +required
	// +kubebuilder:validation:Required
	Host string `json:"host"`
	// Port
	Port int `json:"port,omitempty"`
	// URI args like sslmode, ...
	UriArgs string `json:"uriArgs,omitempty"`
	// Default database
	DefaultDatabase string `json:"defaultDatabase,omitempty"`
	// Duration between two checks for valid engine
	CheckInterval string `json:"checkDuration,omitempty"`
	// Wait for linked resource to be deleted
	WaitLinkedResourcesDeletion bool `json:"waitLinkedResourcesDeletion,omitempty"`
	// User and password secret
	// +required
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`
}

type StatusPhase string

const NoPhase StatusPhase = ""
const FailedPhase StatusPhase = "failed"
const ValidatedPhase StatusPhase = "validated"

// PostgresqlEngineConfigurationStatus defines the observed state of PostgresqlEngineConfiguration
type PostgresqlEngineConfigurationStatus struct {
	// Current phase of the operator
	Phase StatusPhase `json:"phase"`
	// Human-readable message indicating details about current operator phase or error.
	Message string `json:"message"`
	// True if all resources are in a ready state and all work is done.
	Ready bool `json:"ready"`
	// Last validated time
	LastValidatedTime string `json:"lastValidatedDate"`
	// Resource Spec hash
	Hash string `json:"hash"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlEngineConfiguration is the Schema for the postgresqlengineconfigurations API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=postgresqlengineconfigurations,scope=Namespaced
type PostgresqlEngineConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlEngineConfigurationSpec   `json:"spec,omitempty"`
	Status PostgresqlEngineConfigurationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlEngineConfigurationList contains a list of PostgresqlEngineConfiguration
type PostgresqlEngineConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlEngineConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlEngineConfiguration{}, &PostgresqlEngineConfigurationList{})
}
