package allowed_pods

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/SAP/k8s-resource-validator/pkg/test_utils"
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

func TestAllowedPodsValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = "tests.xml"
	RunSpecs(t, "Allowed Pods Validator Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("Allowed Pods", func() {
	var (
		allowedPodUnstructuredResource unstructured.Unstructured
	)

	BeforeEach(func() {
		allowedPodUnstructuredResource = test_utils.CreateUnstructuredPodResource(false, podName, namespace, containerName)
		configDirectory = "/config/"
	})

	Describe("Allowed Pods", func() {
		BeforeEach(func() {
			ctx = context.Background()
			appFs = afero.NewMemMapFs()
			logger = testr.New(&testing.T{})
			ctx = logr.NewContext(ctx, logger)
			ctx = context.WithValue(ctx, common.FileSystemContextKey, appFs)
			allowedListItemsAsString := fmt.Sprintf("- name: %s\n  namespace: %s\n  kind: %s\n- name: %s\n  namespace: %s\n  kind: %s\n",
				podName, namespace, common.KIND_POD,
				replicaSetName, namespace, common.KIND_REPLICA_SET,
			)
			_ = appFs.MkdirAll(configDirectory, 0755)
			_ = afero.WriteFile(appFs, filepath.Join(configDirectory, allowlistFile), []byte(allowedListItemsAsString), 0644)
		})

		It("pod is in allowlist", func() {
			allowedPodsValidator := NewAllowedPodsValidator(ctx, configDirectory)
			violationsArray, err := allowedPodsValidator.Validate(ctx, []unstructured.Unstructured{allowedPodUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))

			allowedPods := allowedPodsValidator.(*AllowedPodsValidator).allowedPods
			Expect(allowedPods).To(HaveLen(1))
		})

		It("pod's owner is in allowlist", func() {
			allowedPodsValidator := NewAllowedPodsValidator(ctx, configDirectory)

			owner1 := metav1.OwnerReference{
				Kind: common.KIND_REPLICA_SET,
				Name: replicaSetName,
			}

			allowedPodUnstructuredResource.SetName("not-allowed")
			allowedPodUnstructuredResource.SetOwnerReferences([]metav1.OwnerReference{owner1})
			violationsArray, err := allowedPodsValidator.Validate(ctx, []unstructured.Unstructured{allowedPodUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))

			allowedPods := allowedPodsValidator.(*AllowedPodsValidator).allowedPods
			Expect(allowedPods).To(HaveLen(1))
		})

		It("pod is NOT in allowlist", func() {
			allowedPodsValidator := NewAllowedPodsValidator(ctx, configDirectory)
			allowedPodUnstructuredResource.SetName("not-allowed")
			violationsArray, err := allowedPodsValidator.Validate(ctx, []unstructured.Unstructured{allowedPodUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(1))

			allowedPods := allowedPodsValidator.(*AllowedPodsValidator).allowedPods
			Expect(allowedPods).To(HaveLen(0))
		})

		It("pod is NOT in allowlist, but is exempt", func() {
			common.ExemptPodLabelName = "label1"
			common.ExemptPodLabelValue = "exempt"
			labels := make(map[string]string)
			labels["label1"] = "exempt"

			allowedPodsValidator := NewAllowedPodsValidator(ctx, configDirectory)
			allowedPodUnstructuredResource.SetName("not-allowed-but-exempt")
			allowedPodUnstructuredResource.SetLabels(labels)

			violationsArray, err := allowedPodsValidator.Validate(ctx, []unstructured.Unstructured{allowedPodUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))

			allowedPods := allowedPodsValidator.(*AllowedPodsValidator).allowedPods
			Expect(allowedPods).To(HaveLen(0))
		})

		It("allowlist not found", func() {
			appFs.Remove(filepath.Join(configDirectory, allowlistFile))
			allowedPodsValidator := NewAllowedPodsValidator(ctx, configDirectory)
			allowedPodUnstructuredResource.SetName("not-allowed")
			violationsArray, err := allowedPodsValidator.Validate(ctx, []unstructured.Unstructured{allowedPodUnstructuredResource})
			Expect(err).To(HaveOccurred())
			Expect(violationsArray).To(HaveLen(0))

			allowedPods := allowedPodsValidator.(*AllowedPodsValidator).allowedPods
			Expect(allowedPods).To(HaveLen(0))
		})
	})
})
