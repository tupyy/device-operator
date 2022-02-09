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

package v1

import (
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeviceDeploymentSpec defines the desired state of DeviceDeployment
type DeviceDeploymentSpec struct {
	// id of the first device
	StartDeviceID int32 `json:"startDeviceID"`

	// id of the last device
	// +optional
	EndDeviceID *int32 `json:"endDeviceID"`

	// +optional
	Count *int32 `json:"count"`

	// devices per deployment
	//+kubebuilder:validation:Minimum=1
	DeviceCount int32 `json:"deviceCount"`

	//+kubebuilder:validation:Minimum=1
	MessagesFrequency int32 `json:"messagesFrequency"`

	DeploymentTemplate appv1.DeploymentSpec `json:"deploymentTemplate"`
}

// DeviceDeploymentStatus defines the observed state of DeviceDeployment
type DeviceDeploymentStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DeviceDeployment is the Schema for the devicedeployments API
type DeviceDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceDeploymentSpec   `json:"spec,omitempty"`
	Status DeviceDeploymentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeviceDeploymentList contains a list of DeviceDeployment
type DeviceDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeviceDeployment{}, &DeviceDeploymentList{})
}
