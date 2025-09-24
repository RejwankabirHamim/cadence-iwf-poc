/*
Copyright 2024.

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

// CapiVersionSpec defines the desired state of CapiVersion
type CapiVersionSpec struct {
	CAPA CAPAVersionMatrix    `json:"capa"`
	CAPG GenericVersionMatrix `json:"capg"`
	CAPZ GenericVersionMatrix `json:"capz"`
}

type DockerMachine struct {
	MachineType string `json:"type,omitempty"`
	//+optional
	MachineOS string `json:"os,omitempty"`
}

type GenericVersionMatrix struct {
	Kind           string        `json:"kind"`
	KindImage      string        `json:"kindImage"`
	Clusterctl     string        `json:"clusterctl"`
	Capi           string        `json:"capi"`
	Infrastructure string        `json:"infrastructure"`
	DockerMachine  DockerMachine `json:"dockerMachine,omitempty"`
}

type CAPAVersionMatrix struct {
	GenericVersionMatrix `json:",inline"`
	Clusterawsadm        string `json:"clusterawsadm"`
	IAMAuthenticator     string `json:"iamAuthenticator"`
}

// CapiVersionStatus defines the observed state of CapiVersion
type CapiVersionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// CapiVersion is the Schema for the capiversions API
type CapiVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CapiVersionSpec   `json:"spec,omitempty"`
	Status CapiVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CapiVersionList contains a list of CapiVersion
type CapiVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CapiVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CapiVersion{}, &CapiVersionList{})
}
