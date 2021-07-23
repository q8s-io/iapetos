package testresource

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type PvcClient struct {
	*kubernetes.Clientset
}

func NewPvcClient(clientset *kubernetes.Clientset)*PvcClient{
	return &PvcClient{clientset}
}

func (client *PvcClient)IsPvcExits(name string)(*corev1.PersistentVolumeClaim,error){
	pvc,err:=client.CoreV1().PersistentVolumeClaims(BasicNameSpace).Get(name,metav1.GetOptions{})
	if err!=nil{
		return nil, err
	}
	return pvc ,nil
}

func getPvcLabelSelector()string{
	labelSelector:=metav1.LabelSelector{MatchLabels: map[string]string{"parentName":BasicName}}
	return labels.Set(labelSelector.MatchLabels).String()
}

func(client *PvcClient)GetNumOfSamePvc()(int32,error){
	pvces,err:=client.CoreV1().PersistentVolumeClaims(BasicNameSpace).List(metav1.ListOptions{
		LabelSelector:       getPvcLabelSelector(),
	})
	if err!=nil{
		return 0, err
	}
	size:=len(pvces.Items)
	return int32(size),nil
}
