package pod

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	Prepared = corev1.PodPhase("preparing")
	Deleted  = corev1.PodPhase("deleting")
)

/*type podHndles struct {
	*corev1.Pod
	client client.Client
}

func NewPodHandles(pod *corev1.Pod) HandlePodIntf {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Port:               9443,
		LeaderElectionID:   "3118b9d6.iapetos.foundary-cloud.io",
	})
	if err!=nil{
		log.Fatal(err)
	}
	return &podHndles{pod,mgr.GetClient()}
}

type HandlePodIntf interface {
	IsPodRunning()bool
}

func (p *podHndles)IsPodRunning()bool{
	if p.CreationTimestamp.Sub(time.Now())==time.Minute*5{
		if err:=p.client.Delete(context.Background(),p);err!=nil{
			log.Fatalf(err.Error())
		}
		return true
	}
	if p.Status.Phase==corev1.PodRunning{
		return true
	}
	return false
}*/
