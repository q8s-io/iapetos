package pv

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	"github.com/q8s-io/iapetos/services"
)

type PVService struct {
	*services.Resource
}

func NewPVService(client client.Client) services.ServiceInf {
	clientMsg:=services.NewResource(client)
	clientMsg.Log.WithName("pv")
	return &PVService{clientMsg}
}

func (pv *PVService)GetName(statefulPod *iapetosapiv1.StatefulPod,index int)*string{

		return &statefulPod.Status.PVCStatusMes[index].PVName
}

func (pv *PVService)DeleteMandatory(ctx context.Context, obj interface{}, statefulPod *iapetosapiv1.StatefulPod)error{
	return nil
}

func (pv *PVService)CreateTemplate(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod,name string,index int)interface{}{
	return nil
}

func (pv *PVService)IsExists(ctx context.Context,nameSpaceName types.NamespacedName)(interface{},bool){
	var pvObj corev1.PersistentVolume
	if err := pv.Get(ctx, nameSpaceName, &pvObj); err != nil {
		if client.IgnoreNotFound(err)!=nil{
			pv.Log.Error(err,"get pv error")
		}
		return nil, false
	}
	return &pvObj, true

}

func (pv *PVService)IsResourceVersionSame(ctx context.Context,obj interface{})bool{
	pvObj:=obj.(*corev1.PersistentVolume)
	if newPv,ok:=pv.IsExists(ctx,types.NamespacedName{
		Namespace: pvObj.Namespace,
		Name:      pvObj.Name,
	});!ok{
		return false
	}else {
		// 判断 resource version 是否一致
		newVersion:=newPv.(*corev1.PersistentVolume).ResourceVersion
		if pvObj.ResourceVersion!=newVersion{
			return false
		}else {
			return true
		}
	}
}

func (pv *PVService)Create(ctx context.Context,obj interface{})(interface{},error){
	return nil, nil
}

func (pv *PVService)Update(ctx context.Context,obj interface{})(interface{},error){
	pvObj:=obj.(*corev1.PersistentVolume)
	if pv.IsResourceVersionSame(ctx,pvObj){
		if err:=pv.Client.Update(ctx,pvObj);err!=nil{
			pv.Log.Error(err,"update pv error")
			return nil, err
		}
	}else {
		pv.Log.Error(errors.New(""),services.ResourceVersionUnSame)
		return nil,errors.New("")
	}
	return pvObj,nil
}

func (pv *PVService)Delete(ctx context.Context,obj interface{})error{
	return nil
}

/*type PVServiceFunc interface {
	IsPVExists(ctx context.Context, namespaceName types.NamespacedName) (*corev1.PersistentVolume, error, bool)
	SetPVRetain(ctx context.Context, pvName *string) error
	SetVolumeAvailable(ctx context.Context, pvName *string) error
	IsPVCanUse(ctx context.Context, pvName string) (*corev1.PersistentVolume, bool)
}

func NewPVService(client client.Client) PVServiceFunc {
	return &PVService{services.NewResource()}
}

// 判断 pv 是否存在
func (p *PVService) IsPVExists(ctx context.Context, namespaceName types.NamespacedName) (*corev1.PersistentVolume, error, bool) {
	var pv corev1.PersistentVolume
	if err := p.Get(ctx, types.NamespacedName{
		Namespace: namespaceName.Namespace,
		Name:      namespaceName.Name,
	}, &pv); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil, false
		}
		p.Log.Error(err, "get pv error")
		return nil, err, false
	}
	return &pv, nil, true
}

// 将 pv 的删除策略设置为 Retain
func (p *PVService) SetPVRetain(ctx context.Context, pvName *string) error {
	if pv, err, ok := p.IsPVExists(ctx, types.NamespacedName{
		Namespace: corev1.NamespaceAll,
		Name:      *pvName,
	}); err == nil && ok { // pv 存在，将 pv 删除策略设置为 Retain，同时将 StorageClassName 设置为空
		pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		pv.Spec.StorageClassName = ""
		if err := p.updatePV(ctx, pv); err != nil { // 修改 pv状态，即设置 pv的删除策略为回收
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

// 等待将 pv 的回收策略设置为 Retain
func (p *PVService) updatePV(ctx context.Context, pv *corev1.PersistentVolume) error {
	if err := p.Update(ctx, pv); err != nil {
		//pvLog.Error(err,"set pv retain error")
		return err
	}
	// 循环等待，直到 pv 的删除策略成功设置为回收
	for {
		pv, _, _ = p.IsPVExists(ctx, types.NamespacedName{
			Namespace: "",
			Name:      pv.Name,
		})
		if pv == nil { // pv 不存在，退出
			break
		}
		// pv 删除策略成功设置为 Retain，退出
		if pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain {
			break
		}
	}
	return nil
}

// 将 pv 状态设置为 Available
func (p *PVService) SetVolumeAvailable(ctx context.Context, pvName *string) error {
	if pv, err, ok := p.IsPVExists(ctx, types.NamespacedName{
		Namespace: corev1.NamespaceAll,
		Name:      *pvName,
	}); err == nil && ok { // pv 存在，将 pv 设置为可用
		pv.Finalizers = nil
		pv.Spec.ClaimRef = nil
		pv.Status.Phase = corev1.VolumeAvailable
		if err := p.changePVStatus(ctx, pv); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

// 修改 pv 状态，防止资源修改冲突
func (p *PVService) changePVStatus(ctx context.Context, pv *corev1.PersistentVolume) error {
	// 循环等待，直到 pv 状态为可用
	for {
		if err := p.Update(ctx, pv, client.DryRunAll); err == nil {
			if err := p.Update(ctx, pv); err == nil {
				return nil
			}
		} else {
			// 更新失败，重新拉取最新状态，然后更新
			newPV, _, _ := p.IsPVExists(ctx, types.NamespacedName{
				Namespace: pv.Namespace,
				Name:      pv.Name,
			})
			if newPV == nil {
				break
			}
			newPV.Finalizers = pv.Finalizers
			newPV.Spec.ClaimRef = nil
			newPV.Status.Phase = corev1.VolumeAvailable
			pv = newPV
		}
	}
	return nil
}

func (p *PVService) SetPVClaimRef(ctx context.Context, pv *corev1.PersistentVolume, nameSpaceName types.NamespacedName) error {
	pv.Spec.StorageClassName = ""
	pv.Spec.ClaimRef.Namespace = nameSpaceName.Namespace
	pv.Spec.ClaimRef.Name = nameSpaceName.Name
	if err := p.Client.Update(ctx, pv); err != nil {
		p.Log.Error(err, "change pv claimref error: ")
		return err
	}
	return nil
}

func (p *PVService) IsPVCanUse(ctx context.Context, pvName string) (*corev1.PersistentVolume, bool) {
	nodeHandler := nodeservice.NewNodeService(p.Client)
	if pv, err, ok := p.IsPVExists(ctx, types.NamespacedName{
		Namespace: corev1.NamespaceAll,
		Name:      pvName,
	}); err == nil && ok {
		var nodeName string
		if name, ok := pv.Annotations["kubevirt.io/provisionOnNode"]; ok {
			nodeName = name
		} else {
			return nil, false
		}
		if nodeHandler.IsNodeReady(ctx, nodeName) {
			return pv, true
		}
	}
	return nil, false
}
*/