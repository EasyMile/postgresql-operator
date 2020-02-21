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
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

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
	URIArgs string `json:"uriArgs,omitempty"`
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

type EngineStatusPhase string

const EngineNoPhase EngineStatusPhase = ""
const EngineFailedPhase EngineStatusPhase = "Failed"
const EngineValidatedPhase EngineStatusPhase = "Validated"

// PostgresqlEngineConfigurationStatus defines the observed state of PostgresqlEngineConfiguration
type PostgresqlEngineConfigurationStatus struct {
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Current phase of the operator
	Phase EngineStatusPhase `json:"phase"`
	// Human-readable message indicating details about current operator phase or error.
	// +optional
	Message string `json:"message"`
	// True if all resources are in a ready state and all work is done.
	// +optional
	Ready bool `json:"ready"`
	// Last validated time
	// +optional
	LastValidatedTime string `json:"lastValidatedTime"`
	// Resource Spec hash
	// +optional
	Hash string `json:"hash"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PostgresqlEngineConfiguration is the Schema for the postgresqlengineconfigurations API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=postgresqlengineconfigurations,scope=Namespaced,shortName=pgengcfg;pgec
// +kubebuilder:printcolumn:name="Last Validation",type=date,description="Last time validated",JSONPath=".status.lastValidatedTime"
// +kubebuilder:printcolumn:name="Phase",type=string,description="Status phase",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
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
