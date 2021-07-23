package testresource

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PvClient struct {
	*kubernetes.Clientset
}

func NewPvClient(clientset *kubernetes.Clientset)*PvClient{
	return &PvClient{clientset}
}

func (client *PvClient)GetPvStatus(name string)(*corev1.PersistentVolumePhase,error){
	pv,err:=client.CoreV1().PersistentVolumes().Get(name,metav1.GetOptions{})
	if err!=nil{
		return nil, err
	}
	return &pv.Status.Phase,nil
}
