package v1alpha1

type CRLink struct {
	// Custom resource name
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Custom resource namespace
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
