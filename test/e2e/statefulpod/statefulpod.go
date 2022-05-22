package statefulpod

import (
	"github.com/onsi/ginkgo"
	"github.com/q8s-io/iapetos/test/e2e/testresource"
)

var TClient *testresource.Client
var TStatefulPodClient *testresource.StatefulPodClient
var TPodClient *testresource.PodClient
var TServiceClient *testresource.ServiceClient
var TPvcClient *testresource.PvcClient
var TPvClient *testresource.PvClient

var _=ginkgo.BeforeSuite(func() {
	var err error
	TClient,err = testresource.NewClient()
	if err!=nil{
		ginkgo.Fail(err.Error())
	}
	TStatefulPodClient = testresource.NewStatefulPodClient(TClient.DynamicClient)
	TPodClient= testresource.NewPodClient(TClient.ClientSet)
	TServiceClient= testresource.NewSvcClient(TClient.ClientSet)
	TPvcClient= testresource.NewPvcClient(TClient.ClientSet)
	TPvClient= testresource.NewPvClient(TClient.ClientSet)
})

var _=ginkgo.AfterSuite(func() {
	_=TStatefulPodClient.DeleteStatefulPod()
})
