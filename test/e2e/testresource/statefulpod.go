package testresource

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)


var  (
	statefulpodRes=schema.GroupVersionResource{
		Group:    StatefulPodGroup,
		Version:  StatefulPodVersion,
		Resource: StatefulPodResource,
	}
)

type StatefulPodClient struct {
	dynamic.Interface
}


func NewStatefulPodClient(p dynamic.Interface)*StatefulPodClient{
	return &StatefulPodClient{p}
}

func BasicStatefulPod(statefulPodType int)*unstructured.Unstructured{

	var template string
	switch statefulPodType {
	case Basic:
		template=BasicTemplate
	case WithService:
		template=WithServiceTemplate
	case WithPVC:
		template=WithPvcTemplate
	case WithStaticPvc:
		template=WithStaticPvcTemplate
	}
	obj:=&unstructured.Unstructured{}
	dec:=yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_,_,_=dec.Decode([]byte(template),nil,obj)
	return obj
}

func (client *StatefulPodClient)CreateStatefulPod(statefulPod *unstructured.Unstructured)error{
	_,err:=client.Resource(statefulpodRes).Namespace(BasicNameSpace).Create(statefulPod,metav1.CreateOptions{})
	if err!=nil{
		return err
	}
	return nil
}

func(client *StatefulPodClient)Expansion(size *int32)error{
	retryErr:=retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result,getErr:=client.Resource(statefulpodRes).Namespace(BasicNameSpace).Get(BasicName,metav1.GetOptions{})
		if getErr!=nil{
			panic(fmt.Errorf("failed get last version of statefulpod error: %v",getErr))
		}
		if err:=unstructured.SetNestedField(result.Object,int64(*size),"spec","size");err!=nil{
			panic(fmt.Errorf("failed to set size value: %v",err))
		}
		_,updateErr:=client.Resource(statefulpodRes).Namespace(BasicNameSpace).Update(result,metav1.UpdateOptions{})
		return updateErr
	})
	return retryErr
}

func (client *StatefulPodClient)DeleteStatefulPod()error{
	if err:=client.Resource(statefulpodRes).Namespace(BasicNameSpace).Delete(BasicName,&metav1.DeleteOptions{});err!=nil{
		return err
	}
	return nil
}

