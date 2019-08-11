package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PlexTranscodeJobSpec defines the desired state of PlexTranscodeJob
// +k8s:openapi-gen=true
type PlexTranscodeJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// An array of arguments to pass to the real plex transcode binary
	Args []string `json:"args"`
	Env []string  `json:"env"`
	Cwd string    `json:"cwd"`
}

// PlexTranscodeJobStatus defines the observed state of PlexTranscodeJob
// +k8s:openapi-gen=true
type PlexTranscodeJobStatus struct {
	// Name of the transcoder pod assigned the transcode job
	Transcoder string `json:"transcoder",omitempty`
	// The state of the job, one of: CREATED ASSIGNED STARTED FAILED COMPLETED
	State PlexTranscodeJobState `json:"state",omitempty`
	Error string                `json:"error",omitempty`
}

type PlexTranscodeJobState string

const (
	PlexTranscodeStateCreated PlexTranscodeJobState = "CREATED"
	PlexTranscodeStateAssigned PlexTranscodeJobState = "ASSIGNED"
	PlexTranscodeStateStarted PlexTranscodeJobState = "STARTED"
	PlexTranscodeStateFailed PlexTranscodeJobState = "FAILED"
	PlexTranscodeStateCompleted PlexTranscodeJobState = "COMPLETED"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlexTranscodeJob is the Schema for the plextranscodejobs API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type PlexTranscodeJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlexTranscodeJobSpec   `json:"spec,omitempty"`
	Status PlexTranscodeJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlexTranscodeJobList contains a list of PlexTranscodeJob
type PlexTranscodeJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlexTranscodeJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PlexTranscodeJob{}, &PlexTranscodeJobList{})
}
