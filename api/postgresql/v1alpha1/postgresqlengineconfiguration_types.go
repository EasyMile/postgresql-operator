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

type ProviderType string

const NoProvider ProviderType = ""
const AWSProvider ProviderType = "AWS"
const AzureProvider ProviderType = "AZURE"

// PostgresqlEngineConfigurationSpec defines the desired state of PostgresqlEngineConfiguration.
type PostgresqlEngineConfigurationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Provider
	// +kubebuilder:validation:Enum="";AWS;AZURE
	Provider ProviderType `json:"provider,omitempty"`
	// Hostname
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`
	// Port
	Port int `json:"port,omitempty"`
	// URI args like sslmode, ...
	URIArgs string `json:"uriArgs,omitempty"`
	// Default database
	DefaultDatabase string `json:"defaultDatabase,omitempty"`
	// Duration between two checks for valid engine
	CheckInterval string `json:"checkInterval,omitempty"`
	// Allow grant admin on every created roles (group or user) for provided PGEC user in order to
	// have power to administrate those roles even with a less powered "admin" user.
	// Operator will create role and after grant PGEC provided user on those roles with admin option if enabled.
	AllowGrantAdminOption bool `json:"allowGrantAdminOption,omitempty"`
	// Wait for linked resource to be deleted
	WaitLinkedResourcesDeletion bool `json:"waitLinkedResourcesDeletion,omitempty"`
	// User and password secret
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secretName"`
	// User connections used for secret generation
	// That will be used to generate secret with primary server as url or
	// to use the pg bouncer one.
	// Note: Operator won't check those values.
	// +optional
	UserConnections *UserConnections `json:"userConnections"`
}

type UserConnections struct {
	// Primary connection is referring to the primary node connection.
	// +optional
	PrimaryConnection *GenericUserConnection `json:"primaryConnection,omitempty"`
	// Bouncer connection is referring to a pg bouncer node.
	// +optional
	BouncerConnection *GenericUserConnection `json:"bouncerConnection,omitempty"`
}

type GenericUserConnection struct {
	// Hostname
	// +required
	// +kubebuilder:validation:Required
	Host string `json:"host"`
	// URI args like sslmode, ...
	URIArgs string `json:"uriArgs"`
	// Port
	Port int `json:"port,omitempty"`
}

type EngineStatusPhase string

const EngineNoPhase EngineStatusPhase = ""
const EngineFailedPhase EngineStatusPhase = "Failed"
const EngineValidatedPhase EngineStatusPhase = "Validated"

// PostgresqlEngineConfigurationStatus defines the observed state of PostgresqlEngineConfiguration.
type PostgresqlEngineConfigurationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

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

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:path=postgresqlengineconfigurations,scope=Namespaced,shortName=pgengcfg;pgec
// +kubebuilder:printcolumn:name="Last Validation",type=date,description="Last time validated",JSONPath=".status.lastValidatedTime"
// +kubebuilder:printcolumn:name="Phase",type=string,description="Status phase",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PostgresqlEngineConfiguration is the Schema for the postgresqlengineconfigurations API.
type PostgresqlEngineConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlEngineConfigurationSpec   `json:"spec,omitempty"`
	Status PostgresqlEngineConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgresqlEngineConfigurationList contains a list of PostgresqlEngineConfiguration.
type PostgresqlEngineConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlEngineConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlEngineConfiguration{}, &PostgresqlEngineConfigurationList{})
}
