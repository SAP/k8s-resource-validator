package readiness

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/SAP/k8s-resource-validator/pkg/common"
)

const (
	ValidatorName     = "built-in:readiness"
	readinesslistFile = "readinesslist.yaml"
)

type ReadinesslistItem struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	Kind      string `yaml:"kind"`
}

func NewReadinessValidator(ctx context.Context, configDir string, ignoreMissingResources bool) common.Validator {
	response := ReadinessValidator{configDir: configDir, ctx: ctx, ignoreMissingResources: ignoreMissingResources}
	response.logger, _ = logr.FromContext(ctx)
	response.appFs = ctx.Value(common.FileSystemContextKey).(afero.Fs)
	return &response
}

type ReadinessValidator struct {
	appFs                  afero.Fs
	ctx                    context.Context
	configDir              string
	logger                 logr.Logger
	ignoreMissingResources bool
}

func (v *ReadinessValidator) GetName() string {
	return ValidatorName
}

// validates all the resources from readinesslist are ready
func (v *ReadinessValidator) Validate(resources []unstructured.Unstructured) (violations []common.Violation, err error) {
	var readinesslist []ReadinesslistItem
	readinesslist, err = v.readReadinesslist(v.configDir)
	if err != nil {
		return nil, err
	}

	for _, readinesslistItem := range readinesslist {
		resource, found := getReadinesslistItemResource(resources, readinesslistItem)
		if !found {
			if v.ignoreMissingResources {
				v.logger.V(2).Info("could not find readinesslist item, but set to ignore")
			} else {
				violation := common.NewViolation(*resource, "readiness violation", 1, ValidatorName)
				violations = append(violations, violation)
			}
			continue
		}

		var isReady bool
		isReady, err = isResourceReady(resource)
		if err != nil {
			msg := fmt.Sprintf("could not determine readiness of resource Kind: %s Name: %s Namespace: %s",
				resource.GetKind(), resource.GetName(), resource.GetNamespace())
			v.logger.Error(err, msg)
			continue
		}

		if isReady {
			v.logger.V(2).Info(fmt.Sprintf("resource Kind: %s Name: %s Namespace: %s is ready",
				resource.GetKind(), resource.GetName(), resource.GetNamespace()))
		} else {
			violation := common.NewViolation(*resource, "readiness violation", 1, ValidatorName)
			violations = append(violations, violation)
		}
	}

	return
}

func (v *ReadinessValidator) readReadinesslist(dir string) ([]ReadinesslistItem, error) {
	readinesslistFileFullPath := filepath.Join(dir, readinesslistFile)
	var readinesslist []ReadinesslistItem

	content, err := afero.ReadFile(v.appFs, readinesslistFileFullPath)
	if err != nil {
		v.logger.Error(err, "couldn't find readinesslist file", readinesslistFileFullPath)
		return nil, err
	} else {
		err := yaml.Unmarshal(content, &readinesslist)
		if err != nil {
			v.logger.Error(err, "couldn't parse readinesslist file")
			return nil, err
		}
	}

	return readinesslist, nil
}

func getReadinesslistItemResource(resources []unstructured.Unstructured, readinesslistItem ReadinesslistItem) (*unstructured.Unstructured, bool) {
	idx := common.IndexFunc(resources, func(resourceIter unstructured.Unstructured) bool {
		return readinesslistItem.Kind == resourceIter.GetKind() &&
			readinesslistItem.Name == resourceIter.GetName() &&
			readinesslistItem.Namespace == resourceIter.GetNamespace()
	})

	if idx > -1 {
		return &resources[idx], true
	}

	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       readinesslistItem.Kind,
			"metadata": map[string]interface{}{
				"name":      readinesslistItem.Name,
				"namespace": readinesslistItem.Namespace,
			},
		},
	}

	return &resource, false
}

func isResourceReady(resource *unstructured.Unstructured) (bool, error) {
	conditions, conditionsFound, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
	if err != nil {
		return false, err
	}
	if conditionsFound {
		idx := common.IndexFunc(conditions, func(condition interface{}) bool {
			conditionAsMap := condition.(map[string]interface{})
			return conditionAsMap["type"].(string) == "Ready" &&
				conditionAsMap["status"].(string) == "True"
		})
		if idx > -1 {
			return true, nil
		} else {
			return false, nil
		}
	} else {
		ready, readyFieldFound, err := unstructured.NestedBool(resource.Object, "status", "ready")
		if err != nil {
			return false, err
		}
		if readyFieldFound {
			return ready, nil
		} else {
			return false, nil
		}
	}
}
