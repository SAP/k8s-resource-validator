package privileged_pods

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/SAP/k8s-resource-validator/pkg/test_utils"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	podName       = "name"
	podNamespace  = "namespace"
	containerName = "test-container"
	allowlistFile = "allowlist.yaml"
)

var (
	configDirectory string
	appFs           afero.Fs
	ctx             context.Context
	logger          logr.Logger
)

func TestPrivilegedPodsValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = "tests.xml"
	RunSpecs(t, "Privileged Pods Validator Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("Privileged Pods", func() {
	var (
		privilegedPodUnstructuredResource unstructured.Unstructured
		procMountUnmaskType               = corev1.UnmaskedProcMount
		isExpectedPodPrivileged           = true
		allowPrivilegeEscalation          = true
		privilegedValTrueReason           = fmt.Sprintf(privilegedReasonTpl, "Privileged", "true")
		privilegedSecurityContext         = &corev1.SecurityContext{
			Privileged: &isExpectedPodPrivileged,
		}
		excessiveCapabilitySecurityContext = &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"CAP_SYS_ADMIN"},
			},
		}
		procMountUnmaskSecurityContext = &corev1.SecurityContext{
			ProcMount: &procMountUnmaskType,
		}
		privilegedEscalationSecurityContext = &corev1.SecurityContext{
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		}
		privilegedRiskContainer = []corev1.Container{
			{
				Name:            containerName,
				SecurityContext: privilegedSecurityContext,
			},
		}
		privilegedRiskEphemeralContainer = []corev1.EphemeralContainer{
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					Name:            containerName,
					SecurityContext: privilegedSecurityContext,
				},
			},
		}
		notPrivilegedContainer = []corev1.Container{
			{
				Name:            containerName,
				SecurityContext: &corev1.SecurityContext{},
			},
		}
		notPrivilegedEphemeralContainer = []corev1.EphemeralContainer{
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					Name:            containerName,
					SecurityContext: &corev1.SecurityContext{},
				},
			},
		}
		podWithPrivilegedContainer = corev1.Pod{Spec: corev1.PodSpec{
			Containers: privilegedRiskContainer,
		}}
		podWithPrivilegedInitContainer = corev1.Pod{Spec: corev1.PodSpec{
			InitContainers: privilegedRiskContainer,
		}}
		podWithPrivilegedEphemeralContainer = corev1.Pod{Spec: corev1.PodSpec{
			EphemeralContainers: privilegedRiskEphemeralContainer,
		}}
		podWithPrivilegedEphemeralAndInitContainers = corev1.Pod{Spec: corev1.PodSpec{
			InitContainers:      privilegedRiskContainer,
			EphemeralContainers: privilegedRiskEphemeralContainer,
		}}
	)

	BeforeEach(func() {
		privilegedPodUnstructuredResource = test_utils.CreateUnstructuredPodResource(true, podName, podNamespace, containerName)
		configDirectory = "/config/"
	})

	Describe("isPrivilegedPod", func() {
		DescribeTable("returns nil in success and - empty string in case there is no risk for privileged pod OR message contains reason for privileged pod in case of encountered one", func(unstructuredPod unstructured.Unstructured, expectedPrivilegedReason string) {
			isPrivilegedMsg, err := isPrivilegedPod(unstructuredPod)
			Expect(err).To(Succeed())
			Expect(isPrivilegedMsg).To(Equal(expectedPrivilegedReason))
		},
			Entry("unstructured pod is privileged", test_utils.CreateUnstructuredPodResource(true, podName, podNamespace, containerName), privilegedValTrueReason),
			Entry("unstructured pod is not privileged", test_utils.CreateUnstructuredPodResource(false, podName, podNamespace, containerName), ""),
		)

		It("returns error when couldn't create v1.pod resource from unstructured", func() {
			isPrivilegedMsg, err := isPrivilegedPod(nil)
			Expect(err).To(MatchError(errUnstructuredPodResource))
			Expect(isPrivilegedMsg).To(BeEmpty())
		})
	})

	Describe("isPrivilegedContainerExist", func() {
		DescribeTable("It returns string that contains reason for privileged pod if any of pod's containers is may run with privileged mode", func(pod corev1.Pod, privilegedReason string) {
			isPrivilegedMsg := doesPrivilegedContainerExist(pod)
			Expect(isPrivilegedMsg).To(Equal(privilegedReason))
		},
			Entry("when app container is privileged", podWithPrivilegedContainer, privilegedValTrueReason),
			Entry("when init container is privileged", podWithPrivilegedInitContainer, privilegedValTrueReason),
			Entry("when ephemeral container is privileged", podWithPrivilegedEphemeralContainer, privilegedValTrueReason),
			Entry("when ephemeral and init containers are privileged", podWithPrivilegedEphemeralAndInitContainers, privilegedValTrueReason+privilegedValTrueReason),
		)
		It("returns empty string if no one of pod's containers is in risk for running with privileged mode", func() {
			isPrivilegedMsg := doesPrivilegedContainerExist(corev1.Pod{Spec: corev1.PodSpec{Containers: notPrivilegedContainer}})
			Expect(isPrivilegedMsg).To(BeEmpty())
		})
	})

	Describe("foundPrivilegedVulnerabilityContainer", func() {
		DescribeTable("It returns the reason for containers interface that may consist of container that is in high risk to run in privileged mode", func(containers interface{}, privilegedReason string) {
			mayBePrivilegedContainerReason := foundPrivilegedVulnerabilityContainer(containers)
			Expect(mayBePrivilegedContainerReason).To(Equal(privilegedReason))
		},
			Entry("when containers from type '[]v1.Container'", privilegedRiskContainer, privilegedValTrueReason),
			Entry("when containers from type '[]v1.EphemeralContainer'", privilegedRiskEphemeralContainer, privilegedValTrueReason),
		)
		DescribeTable("It returns empty string if containers interface is not consist of container that in high risk to run in privileged mode", func(containers interface{}) {
			mayBePrivilegedContainerReason := foundPrivilegedVulnerabilityContainer(containers)
			Expect(mayBePrivilegedContainerReason).To(BeEmpty())
		},
			Entry("when containers from type '[]v1.Container'", notPrivilegedContainer),
			Entry("when containers from type '[]v1.EphemeralContainer'", notPrivilegedEphemeralContainer),
		)
	})

	Describe("foundSecurityContextPrivilegedVulnerability", func() {
		DescribeTable("returns reason for privileged pod if pod's container is in high risk to run in privileged mode", func(contexts *corev1.SecurityContext, privilegedReasonMsg string) {
			mayPrivilegedReason := foundSecurityContextPrivilegedVulnerability(contexts)
			Expect(mayPrivilegedReason).To(Equal(privilegedReasonMsg))
		},
			Entry("when one of the containers run in privileged mode", privilegedSecurityContext, privilegedValTrueReason),
			Entry("when one of the containers run with 'CAP_SYS_ADMIN' capability", excessiveCapabilitySecurityContext, fmt.Sprintf(privilegedReasonTpl, "capabilities", "CAP_SYS_ADMIN")),
			Entry("when one of the containers run with security context procMount different from 'Default'", procMountUnmaskSecurityContext, fmt.Sprintf(privilegedReasonTpl, "ProcMount", "Unmasked")),
			Entry("when one of the containers run with security context which allow privilege escalation", privilegedEscalationSecurityContext, fmt.Sprintf(privilegedReasonTpl, "AllowPrivilegeEscalation", "true")),
		)
		It("returns empty string if there is no risk for pod to run in privileged mode", func() {
			privilegedValue := false
			mayPrivilegedReason := foundSecurityContextPrivilegedVulnerability(&corev1.SecurityContext{Privileged: &privilegedValue})
			Expect(mayPrivilegedReason).To(BeEmpty())
		})
		It("returns empty string when security context is nil", func() {
			mayPrivilegedReason := foundSecurityContextPrivilegedVulnerability(nil)
			Expect(mayPrivilegedReason).To(BeEmpty())
		})
	})

	Describe("Pre Approved Pods", func() {
		BeforeEach(func() {
			ctx = context.Background()
			appFs = afero.NewMemMapFs()
			logger = testr.New(&testing.T{})
			ctx = context.Background()
			ctx = logr.NewContext(ctx, logger)
			allowedListItemsAsString := fmt.Sprintf("- name: %s\n  namespace: %s\n  kind: %s", podName, podNamespace, common.KIND_POD)
			_ = appFs.MkdirAll(configDirectory, 0755)
			_ = afero.WriteFile(appFs, filepath.Join(configDirectory, allowlistFile), []byte(allowedListItemsAsString), 0644)
		})

		It("returns nil and Errors in Violations array when encountered privileged pod", func() {
			privilegedPodsValidator := NewPrivilegedPodsValidator(ctx)
			privilegedPodUnstructuredResource.SetName("foreign-privileged-pod")
			violationsArray, err := privilegedPodsValidator.Validate(ctx, []unstructured.Unstructured{privilegedPodUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(1))
			Expect(violationsArray[0].Message).To(ContainSubstring("found privileged pod"))
			Expect(violationsArray[0].Level).To(Equal(1))
		})

		It("ignores privileged pods that belongs to allow list - returns nil and empty Errors array", func() {
			privilegedPodsValidator := NewPrivilegedPodsValidator(ctx)
			privilegedPodUnstructuredResource.SetName("foreign-privileged-pod")
			privilegedPodsValidator.(*PrivilegedPodsValidator).SetPreApprovedPods([]unstructured.Unstructured{privilegedPodUnstructuredResource})
			violationsArray, err := privilegedPodsValidator.Validate(ctx, []unstructured.Unstructured{privilegedPodUnstructuredResource})
			Expect(err).To(Succeed())
			Expect(violationsArray).To(HaveLen(0))
		})
	})
})
