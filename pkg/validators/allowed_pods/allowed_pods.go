package allowed_pods

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	allowlistFile = "allowlist.yaml"
	ValidatorName = "built-in:allowed-pods"
)

type AllowlistItem struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	Kind      string `yaml:"kind"`
}

func NewAllowedPodsValidator(ctx context.Context, configDir string) common.Validator {
	response := AllowedPodsValidator{
		configDir:   configDir,
		allowedPods: []unstructured.Unstructured{},
		ctx:         ctx,
	}
	response.logger, _ = logr.FromContext(ctx)
	response.appFs = ctx.Value(common.FileSystemContextKey).(afero.Fs)
	return &response
}

type AllowedPodsValidator struct {
	configDir   string
	appFs       afero.Fs
	allowedPods []unstructured.Unstructured
	ctx         context.Context
	logger      logr.Logger
}

func (v *AllowedPodsValidator) GetName() string {
	return ValidatorName
}

func (v *AllowedPodsValidator) Validate(ctx context.Context, resources []unstructured.Unstructured) ([]common.Violation, error) {
	pods := common.GetPods(resources)
	rawAllowlist, err := v.readAllowlist(v.configDir)
	if err != nil {
		return nil, err
	}

	allowlist := allowListToUnstructured(rawAllowlist)

	var violations []common.Violation
	for _, pod := range pods {
		namespace, name := pod.GetNamespace(), pod.GetName()
		if common.IsExempt(pod) {
			v.logger.V(2).Info(fmt.Sprintf("is exempt: %s/%s", namespace, name))
			continue
		}

		if isInAllowlist(resources, allowlist, pod) {
			v.logger.V(2).Info(fmt.Sprintf("found in allowlist: %s/%s", namespace, name))
			v.allowedPods = append(v.allowedPods, pod)
			continue
		}
		violation := common.NewViolation(pod, "NOT found in allowlist", 1, ValidatorName)
		violations = append(violations, violation)
	}
	return violations, nil
}

func (v *AllowedPodsValidator) readAllowlist(dir string) ([]AllowlistItem, error) {
	var allowlistFileFullPath = filepath.Join(dir, allowlistFile)
	var allowlist []AllowlistItem

	content, err := afero.ReadFile(v.appFs, allowlistFileFullPath)
	if err != nil {
		v.logger.Error(err, "couldn't read allowlist file", allowlistFileFullPath)
		return nil, err
	} else {
		err := yaml.Unmarshal(content, &allowlist)
		if err != nil {
			v.logger.Error(err, "couldn't parse allowlist file")
			return nil, err
		}
	}

	return allowlist, nil
}

func isInAllowlist(allResources []unstructured.Unstructured, allowlist []unstructured.Unstructured, item unstructured.Unstructured) bool {
	idx := common.IndexFunc(allowlist, func(itemIter unstructured.Unstructured) bool {
		return item.GetKind() == itemIter.GetKind() &&
			item.GetName() == itemIter.GetName() &&
			item.GetNamespace() == itemIter.GetNamespace()
	})

	if idx > -1 {
		return true
	}

	ownerReferences, err := common.GetOwnerReferences(allResources, item)
	if err == nil {
		for _, s := range ownerReferences {
			owner := unstructured.Unstructured{}
			owner.SetName(s.Name)
			owner.SetNamespace(item.GetNamespace())
			owner.SetKind(s.Kind)
			found := isInAllowlist(allResources, allowlist, owner)
			if found {
				return true
			}
		}
	}

	return false
}

func allowListToUnstructured(allowList []AllowlistItem) []unstructured.Unstructured {
	response := make([]unstructured.Unstructured, len(allowList))

	for i, e := range allowList {
		obj := unstructured.Unstructured{}
		obj.SetName(e.Name)
		obj.SetNamespace(e.Namespace)
		obj.SetKind(e.Kind)
		response[i] = obj
	}

	return response
}
