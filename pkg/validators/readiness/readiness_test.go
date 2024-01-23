package readiness

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/SAP/k8s-resource-validator/pkg/test_utils"
)

const (
	podName       = "name"
	namespace     = "namespace"
	containerName = "test-container"
)

var (
	configDirectory string
	appFs           afero.Fs
	logger          logr.Logger
	ctx             context.Context
)

func TestReadinessValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = "tests.xml"
	RunSpecs(t, "Readiness Validator Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("Readiness", func() {
	var (
		readinessUnstructuredResource unstructured.Unstructured
	)

	BeforeEach(func() {
		readinessUnstructuredResource = test_utils.CreateUnstructuredPodResource(false, podName, namespace, containerName)
		configDirectory = "/config/"
	})

	Describe("Fresh Resources", func() {
		BeforeEach(func() {
			ctx = context.Background()
			appFs = afero.NewMemMapFs()
			logger = testr.New(&testing.T{})
			ctx = logr.NewContext(ctx, logger)
			ctx = context.WithValue(ctx, common.FileSystemContextKey, appFs)

			readinessListItemsAsString := fmt.Sprintf("- name: %s\n  namespace: %s\n  kind: %s\n",
				podName, namespace, common.KIND_POD,
			)
			_ = appFs.MkdirAll(configDirectory, 0755)
			_ = afero.WriteFile(appFs, filepath.Join(configDirectory, readinesslistFile), []byte(readinessListItemsAsString), 0644)
		})

		It("pod is ready (conditions)", func() {
			readinessValidator := NewReadinessValidator(ctx, configDirectory, false)

			readyCondition := make(map[string]interface{})
			readyCondition["type"] = "Ready"
			readyCondition["status"] = "True"
			readyConditions := []interface{}{readyCondition}
			unstructured.SetNestedField(readinessUnstructuredResource.Object, readyConditions, "status", "conditions")

			violationsArray, err := readinessValidator.Validate([]unstructured.Unstructured{readinessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))
		})

		It("pod is ready (ready is true)", func() {
			readinessValidator := NewReadinessValidator(ctx, configDirectory, false)

			unstructured.SetNestedField(readinessUnstructuredResource.Object, true, "status", "ready")

			violationsArray, err := readinessValidator.Validate([]unstructured.Unstructured{readinessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))
		})

		It("pod is not ready (conditions)", func() {
			readinessValidator := NewReadinessValidator(ctx, configDirectory, false)

			readyCondition := make(map[string]interface{})
			readyCondition["type"] = "Ready"
			readyCondition["status"] = "False"
			readyConditions := []interface{}{readyCondition}
			unstructured.SetNestedField(readinessUnstructuredResource.Object, readyConditions, "status", "conditions")

			violationsArray, err := readinessValidator.Validate([]unstructured.Unstructured{readinessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(1))
		})

		It("pod is not ready (missing status)", func() {
			freshnessValidator := NewReadinessValidator(ctx, configDirectory, false)

			violationsArray, err := freshnessValidator.Validate([]unstructured.Unstructured{readinessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(1))
		})

		It("could not read readiness file", func() {
			freshnessValidator := NewReadinessValidator(ctx, "/doesnotexist/", false)

			violationsArray, err := freshnessValidator.Validate([]unstructured.Unstructured{readinessUnstructuredResource})
			Expect(err).To(HaveOccurred())
			Expect(violationsArray).To(HaveLen(0))
		})

		It("could not parse readiness file", func() {
			freshnessValidator := NewReadinessValidator(ctx, configDirectory, false)

			readinessListItemsAsString := "-- "
			_ = afero.WriteFile(appFs, filepath.Join(configDirectory, readinesslistFile), []byte(readinessListItemsAsString), 0644)

			violationsArray, err := freshnessValidator.Validate([]unstructured.Unstructured{readinessUnstructuredResource})
			Expect(err).To(HaveOccurred())
			Expect(violationsArray).To(HaveLen(0))
		})

		It("resource in readiness list not found", func() {
			readinessValidator := NewReadinessValidator(ctx, configDirectory, false)

			readinessListItemsAsString := fmt.Sprintf("- name: %s\n  namespace: %s\n  kind: %s\n",
				"doesnotexist", namespace, common.KIND_POD,
			)
			_ = afero.WriteFile(appFs, filepath.Join(configDirectory, readinesslistFile), []byte(readinessListItemsAsString), 0644)

			violationsArray, err := readinessValidator.Validate([]unstructured.Unstructured{readinessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(1))
		})

		It("resource in readiness list not found, but ignoring", func() {
			readinessValidator := NewReadinessValidator(ctx, configDirectory, true)

			readinessListItemsAsString := fmt.Sprintf("- name: %s\n  namespace: %s\n  kind: %s\n",
				"doesnotexist", namespace, common.KIND_POD,
			)
			_ = afero.WriteFile(appFs, filepath.Join(configDirectory, readinesslistFile), []byte(readinessListItemsAsString), 0644)

			violationsArray, err := readinessValidator.Validate([]unstructured.Unstructured{readinessUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))
		})
	})
})
