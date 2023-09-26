package freshness

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.tools.sap/I034929/k8s-resource-validator/pkg/common"
	"github.tools.sap/I034929/k8s-resource-validator/pkg/test_utils"
)

const (
	podName       = "name"
	namespace     = "namespace"
	containerName = "test-container"
)

var (
	logger logr.Logger
	ctx    context.Context
)

func TestFreshnessValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = "tests.xml"
	RunSpecs(t, "Freshness Validator Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("Freshness", func() {
	var (
		freshnessUnstructuredResource unstructured.Unstructured
	)

	BeforeEach(func() {
		freshnessUnstructuredResource = test_utils.CreateUnstructuredPodResource(false, podName, namespace, containerName)
	})

	Describe("Fresh Resources", func() {
		BeforeEach(func() {
			ctx = context.Background()
			logger = testr.New(&testing.T{})
			ctx = logr.NewContext(ctx, logger)
		})

		It("pod is fresh", func() {
			freshnessValidator := NewFreshnessValidator(ctx, 1) // any resource created less than 1 hour ago is fresh
			thirtyMinutesAgo := metav1.Now().Add(time.Minute * -30)
			freshnessUnstructuredResource.SetCreationTimestamp(metav1.NewTime(thirtyMinutesAgo))

			violationsArray, err := freshnessValidator.Validate(ctx, []unstructured.Unstructured{freshnessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))
		})

		It("pod is fresh (no creationTimestamp)", func() {
			freshnessValidator := NewFreshnessValidator(ctx, 1) // any resource created less than 1 hour ago is fresh

			violationsArray, err := freshnessValidator.Validate(ctx, []unstructured.Unstructured{freshnessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))
		})

		It("pod is stale", func() {
			freshnessValidator := NewFreshnessValidator(ctx, 1) // any resource created less than 1 hour ago is fresh
			thirtyMinutesAgo := metav1.Now().Add(time.Minute * -90)
			freshnessUnstructuredResource.SetCreationTimestamp(metav1.NewTime(thirtyMinutesAgo))

			violationsArray, err := freshnessValidator.Validate(ctx, []unstructured.Unstructured{freshnessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(1))
		})

		It("pod is stale, but is exempt", func() {
			common.ExemptPodLabelName = "label1"
			common.ExemptPodLabelValue = "exempt"
			var labels map[string]string = make(map[string]string)
			labels["label1"] = "exempt"

			freshnessValidator := NewFreshnessValidator(ctx, 1) // any resource created less than 1 hour ago is fresh
			thirtyMinutesAgo := metav1.Now().Add(time.Minute * -90)
			freshnessUnstructuredResource.SetCreationTimestamp(metav1.NewTime(thirtyMinutesAgo))
			freshnessUnstructuredResource.SetName("stale-but-exempt")
			freshnessUnstructuredResource.SetLabels(labels)

			violationsArray, err := freshnessValidator.Validate(ctx, []unstructured.Unstructured{freshnessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))
		})

	})
})
