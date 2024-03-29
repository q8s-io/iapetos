/*


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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StatefulPodSpec defines the desired state of StatefulPod
type StatefulPodSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Minimum=1
	Size            *int32                               `json:"size"`
	Selector        *metav1.LabelSelector                `json:"selector,omitempty"`
	PVRecyclePolicy corev1.PersistentVolumeReclaimPolicy `json:"pvRecyclePolicy,omitempty"`
	ServiceTemplate *corev1.ServiceSpec                  `json:"serviceTemplate,omitempty"`
	PodTemplate     corev1.PodSpec                       `json:"podTemplate"`
	PVCTemplate     *corev1.PersistentVolumeClaimSpec    `json:"pvcTemplate,omitempty"`
	PVNames         []string                             `json:"pvNames,omitempty"`
}

// StatefulPodStatus defines the observed state of StatefulPod
type StatefulPodStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	PodStatusMes []PodStatus `json:"podStatus,omitempty"`
	PVCStatusMes []PVCStatus `json:"pvcStatus,omitempty"`
}

// pod 状态
type PodStatus struct {
	PodName  string          `json:"podName"`
	Status   corev1.PodPhase `json:"status"`
	Index    *int32          `json:"index"`
	NodeName string          `json:"nodeName"`
}

// pvc 状态
type PVCStatus struct {
	Index        *int32                              `json:"index"`
	PVCName      string                              `json:"pvcName"`
	Status       corev1.PersistentVolumeClaimPhase   `json:"status"`
	Capacity     string                              `json:"capacity"`
	AccessModes  []corev1.PersistentVolumeAccessMode `json:"accessModes"`
	StorageClass string                              `json:"storageClass"`
	PVName       string                              `json:"pvName"`
}

// +kubebuilder:object:root=true

// StatefulPod is the Schema for the statefulpods API
type StatefulPod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StatefulPodSpec   `json:"spec,omitempty"`
	Status StatefulPodStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StatefulPodList contains a list of StatefulPod
type StatefulPodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StatefulPod `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StatefulPod{}, &StatefulPodList{})
}
