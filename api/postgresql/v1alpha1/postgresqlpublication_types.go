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

// PostgresqlPublicationSpec defines the desired state of PostgresqlPublication.
type PostgresqlPublicationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Postgresql Database
	// +required
	// +kubebuilder:validation:Required
	Database *common.CRLink `json:"database"`
	// Postgresql Publication name
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Postgresql replication slot name
	// Default value will the publication name
	// +optional
	ReplicationSlotName string `json:"replicationSlotName,omitempty"`
	// Postgresql replication slot plugin
	// Default value will be "pgoutput"
	// +optional
	ReplicationSlotPlugin string `json:"replicationSlotPlugin,omitempty"`
	// Should drop database on Custom Resource deletion ?
	// +optional
	DropOnDelete bool `json:"dropOnDelete,omitempty"`
	// Publication for all tables
	// Note: This is mutually exclusive with "tablesInSchema" & "tables"
	// +optional
	AllTables bool `json:"allTables,omitempty"`
	// Publication for tables in schema
	// Note: This is a list of schema
	// +optional
	TablesInSchema []string `json:"tablesInSchema,omitempty"`
	// Publication for selected tables
	// +optional
	Tables []*PostgresqlPublicationTable `json:"tables,omitempty"`
	// Publication with parameters
	// +optional
	WithParameters *PostgresqlPublicationWith `json:"withParameters,omitempty"`
}

type PostgresqlPublicationTable struct {
	// Table name to use for publication
	TableName string `json:"tableName"`
	// Columns to export
	Columns *[]string `json:"columns,omitempty"`
	// Additional WHERE for table
	AdditionalWhere *string `json:"additionalWhere,omitempty"`
}

type PostgresqlPublicationWith struct {
	// Publish param
	// See here: https://www.postgresql.org/docs/current/sql-createpublication.html#SQL-CREATEPUBLICATION-PARAMS-WITH-PUBLISH
	Publish string `json:"publish"`
	// Publish via partition root param
	// See here: https://www.postgresql.org/docs/current/sql-createpublication.html#SQL-CREATEPUBLICATION-PARAMS-WITH-PUBLISH
	PublishViaPartitionRoot *bool `json:"publishViaPartitionRoot,omitempty"`
}

type PublicationStatusPhase string

const PublicationNoPhase PublicationStatusPhase = ""
const PublicationFailedPhase PublicationStatusPhase = "Failed"
const PublicationCreatedPhase PublicationStatusPhase = "Created"

// PostgresqlPublicationStatus defines the observed state of PostgresqlPublication.
type PostgresqlPublicationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Current phase of the operator
	Phase PublicationStatusPhase `json:"phase"`
	// Human-readable message indicating details about current operator phase or error.
	// +optional
	Message string `json:"message"`
	// True if all resources are in a ready state and all work is done.
	// +optional
	Ready bool `json:"ready"`
	// Created publication name
	// +optional
	Name string `json:"name,omitempty"`
	// Created replication slot name
	// +optional
	ReplicationSlotName string `json:"replicationSlotName,omitempty"`
	// Created replication slot plugin
	// +optional
	ReplicationSlotPlugin string `json:"replicationSlotPlugin,omitempty"`
	// Marker for save
	// +optional
	AllTables *bool `json:"allTables,omitempty"`
	// Resource Spec hash
	// +optional
	Hash string `json:"hash,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=postgresqlpublications,scope=Namespaced,shortName=pgpublication;pgpub
//+kubebuilder:printcolumn:name="Publication",type=string,description="Publication",JSONPath=".status.name"
//+kubebuilder:printcolumn:name="Replication slot name",type=string,description="Status phase",JSONPath=".status.replicationSlotName"
//+kubebuilder:printcolumn:name="Replication slot plugin",type=string,description="Status phase",JSONPath=".status.replicationSlotPlugin"
//+kubebuilder:printcolumn:name="Phase",type=string,description="Status phase",JSONPath=".status.phase"

// PostgresqlPublication is the Schema for the postgresqlpublications API.
type PostgresqlPublication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlPublicationSpec   `json:"spec,omitempty"`
	Status PostgresqlPublicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgresqlPublicationList contains a list of PostgresqlPublication.
type PostgresqlPublicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresqlPublication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresqlPublication{}, &PostgresqlPublicationList{})
}
