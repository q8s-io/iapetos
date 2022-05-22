package withpvc

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/q8s-io/iapetos/test/e2e/statefulpod"
	"github.com/q8s-io/iapetos/test/e2e/testresource"
)

var _ = ginkgo.Describe("create statefulPod with pvc", func() {
	ginkgo.Context("Dynamic binding pv", func() {
		ginkgo.It("create statefulPod with pvc", func() {
			testStatefulPod := testresource.BasicStatefulPod(testresource.WithPVC)
			err := TStatefulPodClient.CreateStatefulPod(testStatefulPod)
			gomega.Expect(err).To(gomega.Succeed(), "%v create error", testresource.BasicName)
			time.Sleep(time.Second * 20)
			// is pod0 ok
			podName0 := fmt.Sprintf("%v-%v", testresource.BasicName, 0)
			pod, err := TPodClient.IsPodExists(podName0)
			// is pod0 create
			gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", podName0)
			// is pod0 running
			gomega.Expect(pod.Status.Phase).To(gomega.Equal(corev1.PodRunning))

			// is pvc0 ok
			pvcName0 := fmt.Sprintf("%v-%v-%v", "data", testresource.BasicName, 0)
			// is pvc0 create
			pvc0, err := TPvcClient.IsPvcExits(pvcName0)
			gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", pvcName0)
			// is pvc0 bound
			gomega.Expect(pvc0.Status.Phase).To(gomega.Equal(corev1.ClaimBound))
			time.Sleep(time.Second * 20)

			// is pod1 ok
			podName1 := fmt.Sprintf("%v-%v", testresource.BasicName, 0)
			pod, err = TPodClient.IsPodExists(podName1)
			// is pod1 create
			gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", podName1)
			// is pod1 running
			gomega.Expect(pod.Status.Phase).To(gomega.Equal(corev1.PodRunning))

			// is pvc0 ok
			pvcName1 := fmt.Sprintf("%v-%v-%v", "data", testresource.BasicName, 1)
			// is pvc0 create
			pvc1, err := TPvcClient.IsPvcExits(pvcName1)
			gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", pvcName0)
			// is pvc0 bound
			gomega.Expect(pvc1.Status.Phase).To(gomega.Equal(corev1.ClaimBound))
			time.Sleep(time.Second * 20)

			// is pod2 ok
			podName2 := fmt.Sprintf("%v-%v", testresource.BasicName, 0)
			pod, err = TPodClient.IsPodExists(podName2)
			// is pod2 create
			gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", podName2)
			// is pod2 running
			gomega.Expect(pod.Status.Phase).To(gomega.Equal(corev1.PodRunning))

			// is pvc0 ok
			pvcName2 := fmt.Sprintf("%v-%v-%v", "data", testresource.BasicName, 2)
			// is pvc0 create
			pvc2, err := TPvcClient.IsPvcExits(pvcName2)
			gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", pvcName0)
			// is pvc0 bound
			gomega.Expect(pvc2.Status.Phase).To(gomega.Equal(corev1.ClaimBound))
		})

		ginkgo.It("test expansion statefulPod with pvc", func() {
			size, err := TPodClient.GetNumOfSamePod()
			gomega.Expect(err).To(gomega.Succeed(), "list pod error")
			gomega.Expect(size).To(gomega.BeEquivalentTo(testresource.BasicSize), "size is not equal 3")

			// change size to 5
			var newSize int32 = 5
			err = TStatefulPodClient.Expansion(&newSize)
			gomega.Expect(err).To(gomega.Succeed(), "expansion statefulPod error")
			time.Sleep(time.Second * 20)

			// get size
			size, err = TPodClient.GetNumOfSamePod()
			gomega.Expect(err).To(gomega.Succeed(), "list pod error")
			gomega.Expect(size).To(gomega.BeEquivalentTo(newSize), "pod size is not equal 5")
			pvcSize, err := TPvcClient.GetNumOfSamePvc()
			gomega.Expect(err).To(gomega.Succeed(), "list pvc error")
			gomega.Expect(pvcSize).To(gomega.BeEquivalentTo(newSize), "pvc size is not equal 5")
		})

		ginkgo.It("test shrink statefulPod with pvc", func() {
			var newSize int32 = 2
			err := TStatefulPodClient.Expansion(&newSize)
			gomega.Expect(err).To(gomega.Succeed(), "expansion statefulPod error")
			time.Sleep(time.Second * 30)

			// get size
			size, err := TPodClient.GetNumOfSamePod()
			gomega.Expect(err).To(gomega.Succeed(), "list pod error")
			gomega.Expect(size).To(gomega.BeEquivalentTo(newSize), "pod size is not equal 2")
			pvcSize, err := TPvcClient.GetNumOfSamePvc()
			gomega.Expect(err).To(gomega.Succeed(), "list pvc error")
			gomega.Expect(pvcSize).To(gomega.BeEquivalentTo(newSize), "pvc size is not equal 2")
			_ = TStatefulPodClient.DeleteStatefulPod()
			time.Sleep(time.Second * 120)
		})
	})

	ginkgo.Context("Static binding", func() {
		// Make sure the PvName0 PvName1 PvName2 exists before testing
		ginkgo.It("create statefulPod with static pvc", func() {
			testStatefulPod := testresource.BasicStatefulPod(testresource.WithStaticPvc)
			err := TStatefulPodClient.CreateStatefulPod(testStatefulPod)
			gomega.Expect(err).To(gomega.Succeed(), "%v create error", testresource.BasicName)
			time.Sleep(time.Second * 30)

			pv0Status, err := TPvClient.GetPvStatus(testresource.PvName0)
			gomega.Expect(err).To(gomega.Succeed(), "get pv %s error", testresource.PvName0)
			gomega.Expect(pv0Status).To(gomega.Equal(corev1.VolumeBound), "%s is not bound", testresource.PvName0)

			pv1Status, err := TPvClient.GetPvStatus(testresource.PvName1)
			gomega.Expect(err).To(gomega.Succeed(), "get pv %s error", testresource.PvName1)
			gomega.Expect(pv1Status).To(gomega.Equal(corev1.VolumeBound), "%s is not bound", testresource.PvName1)

			pv2Status, err := TPvClient.GetPvStatus(testresource.PvName2)
			gomega.Expect(err).To(gomega.Succeed(), "get pv %s error", testresource.PvName2)
			gomega.Expect(pv2Status).To(gomega.Equal(corev1.VolumeBound), "%s is not bound", testresource.PvName2)
		})
	})
})
