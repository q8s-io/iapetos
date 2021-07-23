package testresource

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

type PodClient struct {
	*kubernetes.Clientset
}

func NewPodClient(clientset *kubernetes.Clientset)*PodClient{
	return &PodClient{clientset}
}

func (client *PodClient)IsPodExists(name string)(*corev1.Pod,error){
	pod,err:=client.CoreV1().Pods(BasicNameSpace).Get(name,metav1.GetOptions{})
	if err!=nil{
		return nil,err
	}
	return pod,nil
}

func getLabelSelector()string {
	labelSelector:=metav1.LabelSelector{MatchLabels: map[string]string{"father":BasicName}}
	return labels.Set(labelSelector.MatchLabels).String()
}

func (client *PodClient)GetNumOfSamePod()(int32,error){
	pods,err:=client.CoreV1().Pods(BasicNameSpace).List(metav1.ListOptions{
		TypeMeta:            metav1.TypeMeta{},
		LabelSelector:       getLabelSelector(),
	})
	if err!=nil{
		return 0,err
	}
	size:=len(pods.Items)
	return int32(size),nil
}