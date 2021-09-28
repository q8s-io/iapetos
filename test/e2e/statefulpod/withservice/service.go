package withservice

import (
	 "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	. "w.src.corp.qihoo.net/data-platform/infra/iapetos.git/test/e2e/statefulpod"
	"w.src.corp.qihoo.net/data-platform/infra/iapetos.git/test/e2e/testresource"
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

