package validation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/SAP/k8s-resource-validator/pkg/validators/fake"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"github.com/tonglil/buflogr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

var (
	scheme          *runtime.Scheme
	ctx             context.Context
	configDirectory string
	appFs           afero.Fs
	logger          logr.Logger
	logBuffer       bytes.Buffer
)

func TestResourceValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = "tests.xml"
	RunSpecs(t, "Resource Validator Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("k8s-resource-validator tests", func() {
	BeforeEach(func() {
		os.Unsetenv("CONFIG_DIR")
		configDirectory = defaultConfigDirectory

		appFs = afero.NewMemMapFs()
		logger = buflogr.NewWithBuffer(&logBuffer)
		ctx = context.Background()
		ctx = logr.NewContext(ctx, logger)
		ctx = context.WithValue(ctx, common.FileSystemContextKey, appFs)

		scheme = runtime.NewScheme()

		_ = appsv1.AddToScheme(scheme)
		_ = corev1.AddToScheme(scheme)
		_ = batchv1.AddToScheme(scheme)
	})

	It("validate with no validators", func() {
		pod1Name := "name"
		namespace1 := "namespace"

		pod1 := &unstructured.Unstructured{}
		pod1.SetAPIVersion("v1")
		pod1.SetKind(common.KIND_POD)
		pod1.SetName(pod1Name)
		pod1.SetNamespace(namespace1)

		client := &K8SProvider{
			dynamic:   testclient.NewSimpleDynamicClient(scheme, pod1),
			clientSet: k8sfake.NewSimpleClientset(pod1),
		}

		validation, err := NewValidation(ctx)
		validation.SetClient(client)
		Expect(err).To(Succeed())

		violations, err := validation.Validate(nil)
		Expect(err).To(Succeed())

		Expect(len(violations)).To(Equal(0))
	})

	It("validate with fake validator", func() {
		client := &K8SProvider{
			dynamic:   testclient.NewSimpleDynamicClient(scheme),
			clientSet: k8sfake.NewSimpleClientset(),
		}

		numberOfViolations := 2
		validation, err := NewValidation(ctx)
		validation.SetClient(client)
		Expect(err).To(Succeed())

		fakeValidator, err := fake.NewFakeValidator(ctx, numberOfViolations, false)
		Expect(err).To(Succeed())

		violations, err := validation.Validate([]common.Validator{fakeValidator})
		Expect(err).To(Succeed())

		Expect(len(violations)).To(Equal(numberOfViolations))
	})

	It("validate should fail with error", func() {
		client := &K8SProvider{
			dynamic:   testclient.NewSimpleDynamicClient(scheme),
			clientSet: k8sfake.NewSimpleClientset(),
		}

		numberOfViolations := 2
		validation, err := NewValidation(ctx)
		validation.SetClient(client)
		Expect(err).To(Succeed())

		fakeValidator, err := fake.NewFakeValidator(ctx, numberOfViolations, true)
		Expect(err).To(Succeed())

		violations, err := validation.Validate([]common.Validator{fakeValidator})
		Expect(err).To(Not(Succeed()))

		Expect(len(violations)).To(Equal(0))
	})

	It("write violations to log", func() {
		const message = "unique message 123"
		const validatorName = "unique validator 456"
		const resourceName = "unique resource 789"

		resource := &unstructured.Unstructured{}
		resource.SetAPIVersion("v1")
		resource.SetKind(common.KIND_POD)
		resource.SetName(resourceName)
		resource.SetNamespace("namespace")
		errorViolation := common.Violation{Message: message, ValidatorName: validatorName, Resource: resource, Level: 0}
		infoViolation := common.Violation{Message: message, ValidatorName: validatorName, Resource: resource, Level: 1}
		violations := []common.Violation{errorViolation, infoViolation}

		logLengthBefore := len(logBuffer.String())
		err := LogViolations(ctx, violations, 0)
		Expect(err).To(Succeed())

		out := logBuffer.String()[logLengthBefore:]

		Expect(out).To(ContainSubstring("info violations"))
		Expect(out).To(ContainSubstring("error violations"))

		Expect(out).To(ContainSubstring(message))
		Expect(out).To(ContainSubstring(validatorName))
		Expect(out).To(ContainSubstring(resourceName))
	})

	It("write to log with no violations", func() {
		violations := []common.Violation{}

		logLengthBefore := len(logBuffer.String())
		err := LogViolations(ctx, violations, 1)
		Expect(err).To(Succeed())

		out := logBuffer.String()[logLengthBefore:]
		Expect(out).To(ContainSubstring("valid"))
	})

	It("abort validation", func() {
		abortConfigMapName := "name"
		abortConfigMapNamespace := "namespace"
		AbortValidationConfigMapField := "abort"

		abortConfigMap := &corev1.ConfigMap{}
		abortConfigMap.SetName(abortConfigMapName)
		abortConfigMap.SetNamespace(abortConfigMapNamespace)
		data := make(map[string]string)
		data[AbortValidationConfigMapField] = "true"
		abortConfigMap.Data = data

		client := &K8SProvider{
			dynamic:   testclient.NewSimpleDynamicClient(scheme, abortConfigMap),
			clientSet: k8sfake.NewSimpleClientset(abortConfigMap),
		}

		validation, err := NewValidation(ctx)
		validation.SetClient(client)
		validation.AbortValidationConfigMapField = AbortValidationConfigMapField
		validation.AbortValidationConfigMapName = abortConfigMapName
		validation.AbortValidationConfigMapNamespace = abortConfigMapNamespace
		Expect(err).To(Succeed())

		aborted, _ := validation.preValidate()

		Expect(aborted).To(BeTrue())
	})

	It("do not abort validation (configmap field is not true)", func() {
		abortConfigMapName := "name"
		abortConfigMapNamespace := "namespace"
		AbortValidationConfigMapField := "abort"

		abortConfigMap := &corev1.ConfigMap{}
		abortConfigMap.SetName(abortConfigMapName)
		abortConfigMap.SetNamespace(abortConfigMapNamespace)
		data := make(map[string]string)
		data[AbortValidationConfigMapField] = "false"
		abortConfigMap.Data = data

		client := &K8SProvider{
			dynamic:   testclient.NewSimpleDynamicClient(scheme, abortConfigMap),
			clientSet: k8sfake.NewSimpleClientset(abortConfigMap),
		}

		validation, err := NewValidation(ctx)
		validation.SetClient(client)
		validation.AbortValidationConfigMapField = AbortValidationConfigMapField
		validation.AbortValidationConfigMapName = abortConfigMapName
		validation.AbortValidationConfigMapNamespace = abortConfigMapNamespace
		Expect(err).To(Succeed())

		aborted, _ := validation.preValidate()

		Expect(aborted).To(BeFalse())
	})

	It("do not abort validation (configmap field not found)", func() {
		abortConfigMapName := "name"
		abortConfigMapNamespace := "namespace"
		AbortValidationConfigMapField := "abort"

		abortConfigMap := &corev1.ConfigMap{}
		abortConfigMap.SetName(abortConfigMapName)
		abortConfigMap.SetNamespace(abortConfigMapNamespace)
		data := make(map[string]string)
		data["doesnotexist"] = "true"
		abortConfigMap.Data = data

		client := &K8SProvider{
			dynamic:   testclient.NewSimpleDynamicClient(scheme, abortConfigMap),
			clientSet: k8sfake.NewSimpleClientset(abortConfigMap),
		}

		validation, err := NewValidation(ctx)
		validation.SetClient(client)
		validation.AbortValidationConfigMapField = AbortValidationConfigMapField
		validation.AbortValidationConfigMapName = abortConfigMapName
		validation.AbortValidationConfigMapNamespace = abortConfigMapNamespace
		Expect(err).To(Succeed())

		aborted, _ := validation.preValidate()

		Expect(aborted).To(BeFalse())
	})

	It("additional resource types file parsing error", func() {
		additionalResourceTyepeAsString := "-- "
		_ = appFs.MkdirAll(configDirectory, 0755)
		_ = afero.WriteFile(appFs, filepath.Join(configDirectory, additionalResourceTypesFile), []byte(additionalResourceTyepeAsString), 0644)

		client := &K8SProvider{
			dynamic:   testclient.NewSimpleDynamicClient(scheme),
			clientSet: k8sfake.NewSimpleClientset(),
		}

		validation, err := NewValidation(ctx)
		validation.SetClient(client)
		Expect(err).To(Succeed())

		grvs, err := validation.readAdditionalResourceTypes(configDirectory)
		Expect(err).To(Not(Succeed()))

		Expect(grvs).To(BeNil())
	})

	It("load configuration", func() {
		a := "abort-ns1"
		b := "abort-n1"
		c := "abort-f1"
		d := "exempt-ln1"
		e := "exempt-lv1"

		configAsString := fmt.Sprintf("abort:\n  configMapNamespace: \"%s\"\n  configMapName: \"%s\"\n  configMapField: \"%s\"\nexempt:\n  labelName: \"%s\"\n  labelValue: \"%s\"\n", a, b, c, d, e)
		_ = appFs.MkdirAll(configDirectory, 0755)
		_ = afero.WriteFile(appFs, filepath.Join(configDirectory, configFileName), []byte(configAsString), 0644)

		client := &K8SProvider{
			dynamic:   testclient.NewSimpleDynamicClient(scheme),
			clientSet: k8sfake.NewSimpleClientset(),
		}

		validation, err := NewValidation(ctx)
		validation.SetClient(client)
		Expect(err).To(Succeed())

		validation.loadConfiguration()

		Expect(validation.AbortValidationConfigMapNamespace).To(Equal(a))
		Expect(validation.AbortValidationConfigMapName).To(Equal(b))
		Expect(validation.AbortValidationConfigMapField).To(Equal(c))
		Expect(common.ExemptPodLabelName).To(Equal(d))
		Expect(common.ExemptPodLabelValue).To(Equal(e))
	})
})
