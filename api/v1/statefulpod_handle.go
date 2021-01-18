package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Pod        = "Pod"
	PodVersion = "v1"
	Kind       = "StatefulPod"
	ParentNmae="parentName"
)

func (handle *StatefulPod) CreatePod(podName string) *corev1.Pod {
	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       Pod,
			APIVersion: PodVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: handle.Namespace,
			Annotations: map[string]string{
				GroupVersion.String(): "true",
				ParentNmae: handle.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(handle, schema.GroupVersionKind{
					Group:   GroupVersion.Group,
					Version: GroupVersion.Version,
					Kind:    Kind,
				}),
			},
		},
		Spec: *handle.Spec.PodTemplate.DeepCopy(),
	}
	return &pod
}

func (handle *StatefulPod) IsRunning(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodRunning {
		return true
	}
	return false
}
