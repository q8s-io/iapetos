package pvc

import (
	"context"
	"errors"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	"github.com/q8s-io/iapetos/services"
)


type PVCService struct {
	*services.Resource
}

func NewPVCService(client client.Client) services.ServiceInf {
	clientMsg:=services.NewResource(client)
	clientMsg.Log.WithName("pvc")
	return &PVCService{clientMsg}
}

func (pvc *PVCService)DeleteMandatory(ctx context.Context, obj interface{}, statefulPod *iapetosapiv1.StatefulPod)error{
	pvcObj:=obj.(*corev1.PersistentVolumeClaim)
	if err := pvc.Client.Delete(ctx, pvcObj, client.DeleteOption(client.GracePeriodSeconds(0)), client.DeleteOption(client.PropagationPolicy(metav1.DeletePropagationBackground))); err != nil {
		pvc.Log.Error(err, "delete pvc mandatory error")
		return err
	}
	return nil
}

func (pvc *PVCService)GetName(statefulPod *iapetosapiv1.StatefulPod,index int)*string{
	name:=pvc.SetPVCName(statefulPod,index)
	return &name
}

func (pvc *PVCService)CreateTemplate(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod,name string,index int)interface{}{

	if len(statefulPod.Spec.PVNames) != 0 && len(statefulPod.Spec.PVNames) > index {
		if _, ok := pvc.IsPvCanUse(ctx, &types.NamespacedName{
			Namespace: "",
			Name:      statefulPod.Spec.PVNames[index],
		}); ok {
			name := ""
			statefulPod.Spec.PVCTemplate.StorageClassName = &name
			statefulPod.Spec.PVCTemplate.VolumeName = statefulPod.Spec.PVNames[index]
		}
	}
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: statefulPod.Namespace,
			Annotations: map[string]string{
				iapetosapiv1.GroupVersion.String(): "true",
				services.ParentNmae:                         statefulPod.Name,
				services.Index:                              strconv.Itoa(index),
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulPod, schema.GroupVersionKind{
					Group:   iapetosapiv1.GroupVersion.Group,
					Version: iapetosapiv1.GroupVersion.Version,
					Kind:    services.StatefulPod,
				}),
			},
		},
		Spec: *statefulPod.Spec.PVCTemplate.DeepCopy(),
	}
}

func (pvc *PVCService)IsExists(ctx context.Context,nameSpaceName types.NamespacedName)(interface{},bool){
	var pvcObj corev1.PersistentVolumeClaim
	if err:=pvc.Get(ctx,nameSpaceName,&pvcObj);err!=nil{
		if client.IgnoreNotFound(err)!=nil{
			pvc.Log.Error(err,"get pvc error")
		}
		return nil,false
	}
	return &pvcObj,true
}

func (pvc *PVCService)IsResourceVersionSame(ctx context.Context,obj interface{})bool{
	pvcObj:=obj.(*corev1.PersistentVolumeClaim)
	if newPvc,ok:=pvc.IsExists(ctx,types.NamespacedName{
		Namespace: pvcObj.Namespace,
		Name:      pvcObj.Name,
	});!ok{
		return false
	}else {
		// 判断 resource version 是否一致
		newVersion:=newPvc.(*corev1.PersistentVolumeClaim).ResourceVersion
		if pvcObj.ResourceVersion!=newVersion{
			return false
		}else {
			return true
		}
	}
}

func (pvc *PVCService)Create(ctx context.Context,obj interface{})(interface{},error){
	pvcObj:=obj.(*corev1.PersistentVolumeClaim)
	if err:=pvc.Client.Create(ctx,pvcObj);err!=nil{
		pvc.Log.Error(err,"create pvc error")
		return nil, err
	}
	return pvcObj,nil
}

func (pvc *PVCService)Update(ctx context.Context,obj interface{})(interface{},error){
	pvcObj:=obj.(*corev1.PersistentVolumeClaim)
	if pvc.IsResourceVersionSame(ctx,pvcObj){
		if err:=pvc.Client.Update(ctx,pvcObj);err!=nil{
			pvc.Log.Error(err,"update pvc error")
			return nil, err
		}
	}else {
		pvc.Log.Error(errors.New(""),services.ResourceVersionUnSame)
		return nil,errors.New("")
	}
	return pvcObj,nil
}

func (pvc *PVCService)Delete(ctx context.Context,obj interface{})error{
	pvcObj:=obj.(*corev1.PersistentVolumeClaim)
	if err:=pvc.Client.Delete(ctx,pvcObj);err!=nil{
		pvc.Log.Error(err,"delete pvc error")
		return err
	}
	return nil
}


/*type PVCServiceFunc interface {
	PVCTemplate(ctx context.Context, statefulpod *iapetosapiv1.StatefulPod, pvcName string, index int) (*corev1.PersistentVolumeClaim, error)
	CreatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error
	IsPVCExist(ctx context.Context, name types.NamespacedName) (*corev1.PersistentVolumeClaim, error, bool)
	DeletePVC(ctx context.Context, deletePVC *corev1.PersistentVolumeClaim) error
	SetPVCName(statefulPod *iapetosapiv1.StatefulPod, index int) string
}

func NewPVCService(client client.Client) PVCServiceFunc {
	return &PVCService{services.NewResource()}
}

// 创建 pvc 模板
func (p *PVCService) PVCTemplate(ctx context.Context, statefulpod *iapetosapiv1.StatefulPod, pvcName string, index int) (*corev1.PersistentVolumeClaim, error) {
	pvHandler := pvservice.NewPVService(p.Client)
	if len(statefulpod.Spec.PVNames) != 0 && len(statefulpod.Spec.PVNames) > index {
		if _, ok := pvHandler.IsPVCanUse(ctx, statefulpod.Spec.PVNames[index]); ok {
			name := ""
			statefulpod.Spec.PVCTemplate.StorageClassName = &name
			statefulpod.Spec.PVCTemplate.VolumeName = statefulpod.Spec.PVNames[index]
		}
	}
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: statefulpod.Namespace,
			Annotations: map[string]string{
				iapetosapiv1.GroupVersion.String(): "true",
				ParentNmae:                         statefulpod.Name,
				Index:                              strconv.Itoa(index),
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulpod, schema.GroupVersionKind{
					Group:   iapetosapiv1.GroupVersion.Group,
					Version: iapetosapiv1.GroupVersion.Version,
					Kind:    StatefulPod,
				}),
			},
		},
		Spec: *statefulpod.Spec.PVCTemplate.DeepCopy(),
	}, nil

}

// 判断 pvc 是否存在
func (p *PVCService) IsPVCExist(ctx context.Context, nameSpaceName types.NamespacedName) (*corev1.PersistentVolumeClaim, error, bool) {
	var pvc corev1.PersistentVolumeClaim
	if err := p.Get(ctx, types.NamespacedName{
		Namespace: nameSpaceName.Namespace,
		Name:      nameSpaceName.Name,
	}, &pvc); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil, false
		}
		p.Log.Error(err, "get pvc error")
		return nil, err, false
	}
	return &pvc, nil, true
}
*/
/*
// 创建 pvc
func (p *PVCService) CreatePVC(ctx context.Context, createPvc *corev1.PersistentVolumeClaim) error {
	if err := p.Create(ctx, createPvc); err != nil {
		p.Log.Error(err, "create pvc error")
		return err
	}
	return nil
}

// 删除 pvc
func (p *PVCService) DeletePVC(ctx context.Context, deletePVC *corev1.PersistentVolumeClaim) error {
	if err := p.Delete(ctx, deletePVC); err != nil {
		p.Log.Error(err, "delete pvc error")
		return err
	}
	return nil
}

// 设置 pvc name
func (p *PVCService) SetPVCName(statefulPod *iapetosapiv1.StatefulPod, index int) string {
	if statefulPod.Spec.PVCTemplate == nil { // pvc 不需要创建，返回 none
		return "none"
	}
	if statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName == "" {
		statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName = "data"
	}
	return fmt.Sprintf("%v-%v-%v", statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName, statefulPod.Name, index)
}
*/