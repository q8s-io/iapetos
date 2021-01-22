package statefulpod

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
	nodecontrl "iapetos/controllers/node"
	podcontrl "iapetos/controllers/pod"
	resourcecfg "iapetos/initconfig"
	"iapetos/tools"
)


type StatefulPodController struct {
	client.Client
}

type StatefulPodContrlInf interface {
	StatefulPodContrl(ctx context.Context,statefulPod *statefulpodv1.StatefulPod)error
	MonitorPodStatus(ctx context.Context,pod *corev1.Pod)error
}


func NewStatefulPodController(client client.Client)StatefulPodContrlInf{
	return &StatefulPodController{client}
}

func (s *StatefulPodController)StatefulPodContrl(ctx context.Context,statefulPod *statefulpodv1.StatefulPod)error{
	lenStatus:=len(statefulPod.Status.PodStatusMes)
	if len(statefulPod.Status.PodStatusMes) < int(*statefulPod.Spec.Size) {
		return s.expansionPod(ctx,statefulPod,lenStatus)
	} else if len(statefulPod.Status.PodStatusMes) > int(*statefulPod.Spec.Size) {
		return s.shrinkPod(ctx,statefulPod,lenStatus)
	} else {
		return s.maintainPod(ctx,statefulPod)
	}
}

func (s *StatefulPodController)changeSpecPodStatus(ctx context.Context,statefulPod *statefulpodv1.StatefulPod)error{
	for {
		if err := s.Update(ctx, statefulPod, client.DryRunAll); err == nil {
			_ = s.Update(ctx, statefulPod)
			break
		}
	}
	return nil
}

// 维护pod状态
func (s *StatefulPodController)maintainPod(ctx context.Context,statefulPod *statefulpodv1.StatefulPod)error{
	for i, pod := range statefulPod.Status.PodStatusMes {
		if pod.PodName==""{
			return s.expansionPod(ctx,statefulPod,int(*pod.Index))
		}
		if pod.Status == podcontrl.Deleting {
			if _, err, ok := podcontrl.NewPodController(s.Client).IsPodExist(ctx, types.NamespacedName{Namespace: statefulPod.Namespace, Name: pod.PodName}); err == nil && !ok {
				statefulPod.Status.PodStatusMes[i]=statefulpodv1.PodStatus{
					PodName:  "",
					Status:   "",
					Index:    statefulPod.Status.PodStatusMes[i].Index,
					NodeName: "",
				}
				return s.changeSpecPodStatus(ctx,statefulPod)
			}
		}
	}
	return nil
}

// 扩容
func (s *StatefulPodController)expansionPod(ctx context.Context,statefulPod *statefulpodv1.StatefulPod,index int)error{
	podIndex:=int32(index)
	podName:=fmt.Sprintf("%v%v",statefulPod.Name,index)
	if _, err, ok := podcontrl.NewPodController(s.Client).IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); !ok && err == nil {
		pod := podcontrl.NewPodController(s.Client).PodTempale(ctx, statefulPod, podName, index)
		if err := podcontrl.NewPodController(s.Client).CreatePod(ctx, pod); err != nil {
			return err
		}
		if len(statefulPod.Status.PodStatusMes) == index {
			statefulPod.Status.PodStatusMes = append(statefulPod.Status.PodStatusMes, statefulpodv1.PodStatus{
				PodName: pod.Name,
				Status:  podcontrl.Preparing,
				Index:   &podIndex,
			})
			return s.changeSpecPodStatus(ctx, statefulPod)
		} else {
			statefulPod.Status.PodStatusMes[index] = statefulpodv1.PodStatus{
				PodName:  pod.Name,
				Status:   podcontrl.Preparing,
				Index:    &podIndex,
				NodeName: "",
			}
			return s.changeSpecPodStatus(ctx, statefulPod)
		}
	}
	return nil
}

// 缩容
func (s *StatefulPodController)shrinkPod(ctx context.Context,statefulPod *statefulpodv1.StatefulPod,index int)error{
	podName:=fmt.Sprintf("%v%v",statefulPod.Name,index-1)
	if pod, err, ok := podcontrl.NewPodController(s.Client).IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); err == nil && ok{
		if podcontrl.NewPodController(s.Client).JudgmentPodDel(pod) {
			return nil
		}
		if err:=podcontrl.NewPodController(s.Client).DeletePod(ctx,pod);err!=nil{
			return err
		}
	}else if err == nil && !ok {
		statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:index-1]
		if err := s.changeSpecPodStatus(ctx, statefulPod); err != nil {
			return err
		}
	}
	return nil
}

func (s *StatefulPodController)getStatefulPod(ctx context.Context,namespaceName *types.NamespacedName)*statefulpodv1.StatefulPod{
	var statefulPod statefulpodv1.StatefulPod
	if err := s.Get(ctx, types.NamespacedName{
		Namespace: namespaceName.Namespace,
		Name:     namespaceName.Name,
	}, &statefulPod); err != nil {
		return nil
	}
	return &statefulPod
}

func (s *StatefulPodController)MonitorPodStatus(ctx context.Context,pod *corev1.Pod)error{
	statefulPod:=s.getStatefulPod(ctx,&types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Annotations[podcontrl.ParentNmae],
	})
	if statefulPod==nil{
		return nil
	}
	index := tools.StrToInt(pod.Annotations["index"])
	if podcontrl.NewPodController(s.Client).JudgmentPodDel(pod) {
		if statefulPod.Status.PodStatusMes[index].Status == podcontrl.Deleting {
			return nil
		}
		statefulPod.Status.PodStatusMes[index].Status = podcontrl.Deleting
		if err:=s.changeSpecPodStatus(ctx,statefulPod);err!=nil{
			return err
		}
		return nil
	}
	if !nodecontrl.NewNodeContrl(s.Client).IsNodeReady(ctx,pod.Spec.NodeName){
		if podcontrl.NewPodController(s.Client).JudgmentPodDel(pod){
			return nil
		}
		if err:=podcontrl.NewPodController(s.Client).DeletePodMandatory(ctx,pod);err!=nil{
			return err
		}else{
			statefulPod.Status.PodStatusMes[index].Status = podcontrl.Deleting
			if err:=s.changeSpecPodStatus(ctx,statefulPod);err!=nil{
				return err
			}
		}
		return nil
	}
	if pod.Status.Phase == corev1.PodRunning {
		if statefulPod.Status.PodStatusMes[index].Status == corev1.PodRunning {
			return nil
		}
		statefulPod.Status.PodStatusMes[index].Status = corev1.PodRunning
		statefulPod.Status.PodStatusMes[index].NodeName = pod.Spec.NodeName
		if err :=s.changeSpecPodStatus(ctx,statefulPod);err!=nil{
			return err
		}
	}
	if time.Now().Sub(pod.CreationTimestamp.Time) >= time.Second*time.Duration(resourcecfg.StatefulPodResourceCfg.Pod.Timeout) && pod.Status.Phase == podcontrl.Preparing {

	}
	return nil
}