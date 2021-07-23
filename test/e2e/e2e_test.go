package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestRunE2E(t *testing.T){
	runE2E()
	RegisterFailHandler(Fail)
	RunSpecs(t,"statefulpod e2e test suites")
}
