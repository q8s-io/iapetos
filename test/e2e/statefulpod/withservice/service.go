package withservice

import (
	 "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "github.com/q8s-io/iapetos/test/e2e/statefulpod"
	"github.com/q8s-io/iapetos/test/e2e/testresource"
	"time"
)


var _=ginkgo.Describe("create statefulPod with headless service", func() {
	ginkgo.It("", func() {
		testStatefulPod:= testresource.BasicStatefulPod(testresource.WithService)
		err:=TStatefulPodClient.CreateStatefulPod(testStatefulPod)
		gomega.Expect(err).To(gomega.Succeed(),"%v create error", testresource.BasicName)
		time.Sleep(time.Second*30)

		ok:=TServiceClient.IsServiceOK()
		gomega.Expect(ok).To(gomega.Equal(true))
	})
})

