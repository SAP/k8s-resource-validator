package fake

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.tools.sap/I034929/k8s-resource-validator/pkg/common"
	"github.tools.sap/I034929/k8s-resource-validator/pkg/test_utils"
)

const (
	podName       = "name"
	namespace     = "namespace"
	containerName = "test-container"

	replicaSetName = "rs1"
)

var (
	configDirectory string
	appFs           afero.Fs
	logger          logr.Logger
	ctx             context.Context
)

func TestFakeValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = "tests.xml"
	RunSpecs(t, "Fake Validator Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("Fake Validator", func() {
	var (
		podUnstructuredResource unstructured.Unstructured
	)

	BeforeEach(func() {
		podUnstructuredResource = test_utils.CreateUnstructuredPodResource(false, podName, namespace, containerName)
		configDirectory = "/config/"
	})

	Describe("Fake", func() {
		BeforeEach(func() {
			ctx = context.Background()
			appFs = afero.NewMemMapFs()
			logger = testr.New(&testing.T{})
			ctx = logr.NewContext(ctx, logger)
			ctx = context.WithValue(ctx, common.FileSystemContextKey, appFs)
		})

		It("violations", func() {
			violationsCount := 2
			allowedPodsValidator := NewFakeValidator(ctx, violationsCount)
			violationsArray, err := allowedPodsValidator.Validate(ctx, []unstructured.Unstructured{podUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(violationsCount))

		})
	})
})
