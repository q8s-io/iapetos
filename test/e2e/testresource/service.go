package testresource

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ServiceClient struct {
	*kubernetes.Clientset
}

func NewSvcClient(clientset *kubernetes.Clientset)*ServiceClient{
	return &ServiceClient{clientset}
}

func (client *ServiceClient)IsServiceOK()bool{
	endpoint,err:=client.CoreV1().Endpoints(BasicNameSpace).Get(fmt.Sprintf("%v-%v",BasicName,"service"),metav1.GetOptions{})
	if err!=nil{
		return false
	}
	pod,err:=NewPodClient(client.Clientset).IsPodExists(fmt.Sprintf("%s-%d",BasicName,0))
	if err!=nil{
		return false
	}
	if pod.Status.PodIP==endpoint.Subsets[0].Addresses[0].IP{
		return true
	}
	return false
}
