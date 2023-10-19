package privileged_pods

import (
	"context"
	"errors"
	"fmt"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"
)

var (
	excessiveCapabilities      = []string{capabilityCapSysAdmin}
	errPodResource             = errors.New("object is not type pod")
	errUnstructuredPodResource = errors.New("unstructured pod resource is nil")
)

const (
	capabilityCapSysAdmin = "CAP_SYS_ADMIN"
	privilegedReasonTpl   = "securityContext %s value is %s\n"
)

func NewPrivilegedPodsValidator(ctx context.Context) common.Validator {
	response := PrivilegedPodsValidator{ctx: ctx}
	response.logger, _ = logr.FromContext(ctx)
	return &response
}

type PrivilegedPodsValidator struct {
	preApprovedPods []unstructured.Unstructured
	ctx             context.Context
	logger          logr.Logger
}

/*
*

	the return violations array is non-nil if invalid resources were found
	the return error is non-nil if another type of error was encountered
*/
func (v *PrivilegedPodsValidator) Validate(ctx context.Context, resources []unstructured.Unstructured) ([]common.Violation, error) {
	pods := common.GetPods(resources)
	var violations []common.Violation
	for _, pod := range pods {
		namespace, name := pod.GetNamespace(), pod.GetName()
		if common.IsExempt(pod) {
			v.logger.V(2).Info(fmt.Sprintf("is exempt: %s/%s", namespace, name))
			continue
		}

		idx := common.IndexFunc(v.preApprovedPods, func(p unstructured.Unstructured) bool {
			return p.GetName() == pod.GetName() && p.GetNamespace() == pod.GetNamespace() && p.GetKind() == pod.GetKind()
		})
		if idx >= 0 {
			v.logger.V(2).Info(fmt.Sprintf("is exempt: %s/%s", namespace, name))
			continue
		}

		var err error
		violations, err = v.handlePrivilegedPod(pod, violations, namespace, name)
		if err != nil {
			return nil, err
		}
	}
	return violations, nil
}

func (v *PrivilegedPodsValidator) GetName() string {
	return "built-in:privileged-pods"
}

func (v *PrivilegedPodsValidator) SetPreApprovedPods(pods []unstructured.Unstructured) {
	v.preApprovedPods = pods
}

func (v *PrivilegedPodsValidator) handlePrivilegedPod(p unstructured.Unstructured, violations []common.Violation, namespace, name string) ([]common.Violation, error) {
	privilegedPodMsg, err := isPrivilegedPod(p)
	if err != nil {
		return nil, err
	}

	if privilegedPodMsg != "" {
		violation := common.NewViolation(p, "found privileged pod", 1, v.GetName())
		violations = append(violations, violation)
	}
	return violations, nil
}

// isPrivilegedPod function convert a resource interface to a v1.Pod object and creates a message for suspicious configurations
// in pod's containers security context which can lead the pod to run with privileged mode. (message is empty in case of there is no privileged pod)
// it ignores pods that belongs to the allowlist.
func isPrivilegedPod(podResource interface{}) (string, error) {
	var pod v1.Pod
	err := createPodFromUnstructuredResource(podResource, &pod)
	if err != nil {
		return "", err
	}

	return doesPrivilegedContainerExist(pod), nil
}

func createPodFromUnstructuredResource(podResource interface{}, pod *v1.Pod) error {
	err := validateObjectPod(pod)
	if err != nil {
		return err
	}
	if podResource == nil {
		return errUnstructuredPodResource
	}
	err = ConvertResourceToObj(podResource.(unstructured.Unstructured), pod)
	return err
}

// ConvertResourceToObj function converts the unstructured object to the wanted object interface type
func ConvertResourceToObj(resource unstructured.Unstructured, obj interface{}) error {
	err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(resource.Object, obj, true)
	return err
}

func validateObjectPod(obj interface{}) error {
	if _, ok := obj.(*v1.Pod); !ok {
		return errPodResource
	}
	return nil
}

// doesPrivilegedContainerExist gets a pod and validates each of its container types, returns a message according search findings
func doesPrivilegedContainerExist(pod v1.Pod) string {
	containersTypeContainer := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	privilegedReason := foundPrivilegedVulnerabilityContainer(containersTypeContainer) + foundPrivilegedVulnerabilityContainer(pod.Spec.EphemeralContainers)
	return privilegedReason
}

func foundPrivilegedVulnerabilityContainer(containersInput interface{}) string {
	switch containers := containersInput.(type) {
	case []v1.Container:
		for _, container := range containers {
			privilegedReason := foundSecurityContextPrivilegedVulnerability(container.SecurityContext)
			if privilegedReason != "" {
				return privilegedReason
			}
		}
	case []v1.EphemeralContainer:
		for _, container := range containers {
			privilegedReason := foundSecurityContextPrivilegedVulnerability(container.SecurityContext)
			if privilegedReason != "" {
				return privilegedReason
			}
		}
	}

	return ""
}

// foundSecurityContextPrivilegedVulnerability gets a container's security context and checks for excessive permissions of configurations.
// returns reason for privileged notification.
func foundSecurityContextPrivilegedVulnerability(securityContext *v1.SecurityContext) string {
	if securityContext == nil {
		return ""
	}

	// Check if ProcMount is set to UnmaskedProcMount

	if securityContext.ProcMount != nil && *securityContext.ProcMount == v1.UnmaskedProcMount {
		return fmt.Sprintf(privilegedReasonTpl, "ProcMount", "Unmasked")
	}

	// Check if AllowPrivilegeEscalation is true
	if securityContext.AllowPrivilegeEscalation != nil && *securityContext.AllowPrivilegeEscalation {
		return fmt.Sprintf(privilegedReasonTpl, "AllowPrivilegeEscalation", "true")
	}

	// Check if Privileged is true
	if securityContext.Privileged != nil && *securityContext.Privileged {
		return fmt.Sprintf(privilegedReasonTpl, "Privileged", "true")
	}

	// Check if any of the capabilities in the Add slice are in the excessiveCapabilities slice
	if securityContext.Capabilities != nil {
		for _, capability := range securityContext.Capabilities.Add {
			if slices.Contains(excessiveCapabilities, string(capability)) {
				return fmt.Sprintf(privilegedReasonTpl, "capabilities", string(capability))
			}
		}
	}
	// If none of the checks pass, return empty string
	return ""
}
