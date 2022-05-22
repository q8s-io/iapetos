package basic

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/q8s-io/iapetos/test/e2e/statefulpod"
	"github.com/q8s-io/iapetos/test/e2e/testresource"
)

var _ = ginkgo.Describe("statefulPod resource test", func() {

	ginkgo.It("create basic statefulPod", func() {
		// create statefulPod
		testStatefulPod := testresource.BasicStatefulPod(testresource.Basic)
		err := TStatefulPodClient.CreateStatefulPod(testStatefulPod)
		gomega.Expect(err).To(gomega.Succeed(), "%v create error", testresource.BasicName)
		time.Sleep(time.Second * 10)

		// is pod0 ok
		podName0 := fmt.Sprintf("%v-%v", testresource.BasicName, 0)
		pod, err := TPodClient.IsPodExists(podName0)
		// is pod0 create
		gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", podName0)
		// is pod0 running
		gomega.Expect(pod.Status.Phase).To(gomega.Equal(corev1.PodRunning))
		time.Sleep(time.Second * 10)

		// is pod1 ok
		podName1 := fmt.Sprintf("%v-%v", testresource.BasicName, 0)
		pod, err = TPodClient.IsPodExists(podName1)
		// is pod1 create
		gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", podName1)
		// is pod1 running
		gomega.Expect(pod.Status.Phase).To(gomega.Equal(corev1.PodRunning))
		time.Sleep(time.Second * 10)

		// is pod2 ok
		podName2 := fmt.Sprintf("%v-%v", testresource.BasicName, 0)
		pod, err = TPodClient.IsPodExists(podName2)
		// is pod2 create
		gomega.Expect(err).To(gomega.Succeed(), "%v is not exists", podName2)
		// is pod2 running
		gomega.Expect(pod.Status.Phase).To(gomega.Equal(corev1.PodRunning))
	})

	ginkgo.It("test expansion", func() {
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
		gomega.Expect(size).To(gomega.BeEquivalentTo(newSize), "size is not equal 5")
	})

	ginkgo.It("test shrink", func() {
		var newSize int32 = 2
		err := TStatefulPodClient.Expansion(&newSize)
		gomega.Expect(err).To(gomega.Succeed(), "expansion statefulPod error")
		time.Sleep(time.Second * 30)

		// get size
		size, err := TPodClient.GetNumOfSamePod()
		gomega.Expect(err).To(gomega.Succeed(), "list pod error")
		gomega.Expect(size).To(gomega.BeEquivalentTo(newSize), "size is not equal 2")
	})
})
