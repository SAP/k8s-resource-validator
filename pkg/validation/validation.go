package validation

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	koanfYaml "github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	additionalResourceTypesFile = "additionalResourceTypes.yaml"
	configFileName              = "config.yaml"
	defaultConfigDirectory      = "/config/"
)

type Validation struct {
	Client                            *K8SProvider
	Resources                         []unstructured.Unstructured
	AbortValidationConfigMapField     string
	AbortValidationConfigMapName      string
	AbortValidationConfigMapNamespace string
	abortFunc                         common.AbortFunc
	ctx                               context.Context
	appFs                             afero.Fs
	logger                            logr.Logger
	PreValidated                      bool
}

func NewValidation(ctx context.Context) (*Validation, error) {
	response := Validation{}
	response.ctx = ctx
	response.appFs = ctx.Value(common.FileSystemContextKey).(afero.Fs)

	response.loadConfiguration()

	response.PreValidated = false

	// set default values
	response.AbortValidationConfigMapField = "deploying"
	response.AbortValidationConfigMapName = "landscape-state"
	response.AbortValidationConfigMapNamespace = "center"

	return &response, nil
}

/*
If this function is not called, a new client will be created in preValidate()
*/
func (v *Validation) SetContext(context context.Context) {
	v.ctx = context
}
func (v *Validation) SetAppFileSystem(appFs afero.Fs) {
	v.appFs = appFs
}
func (v *Validation) SetLogger(logger logr.Logger) {
	v.logger = logger
}

func (v *Validation) SetClient(client *K8SProvider) {
	v.Client = client
}

func (v *Validation) SetAbortFunc(abortFunc common.AbortFunc) {
	v.abortFunc = abortFunc
}

func (v *Validation) preValidate() (bool, error) {
	if !v.PreValidated {
		if v.Client == nil {
			var err error
			v.Client, err = getClient()
			if err != nil {
				v.logger.Error(err, "unable to create client")
				return false, err
			}
		}

		configDir := resolveConfigDirectory()

		additionalResourceTypes, err := v.readAdditionalResourceTypes(configDir)
		if err != nil {
			return false, err
		}

		v.Resources = fetchResources(v.ctx, *v.Client, additionalResourceTypes)

		if v.abortFunc == nil {
			shouldAbort, abortMessage, err := v.shouldAbortValidation(v.ctx, *v.Client)
			if err != nil {
				v.logger.Error(err, "error determining whether should abort the validation")
				return true, err
			}

			v.logger.V(2).Info(abortMessage)
			if shouldAbort {
				return true, nil
			}
		} else {
			return v.abortFunc()
		}

		v.PreValidated = true
	}

	return false, nil
}

/*
returns a slice of violations (empty if no violations are found)
*/
func (v *Validation) Validate(validators []common.Validator) ([]common.Violation, error) {
	var cumulativeErr error
	var violations []common.Violation

	aborted, err := v.preValidate()
	if err != nil {
		cumulativeErr = errors.Join(cumulativeErr, err)
		return violations, cumulativeErr
	}

	if aborted {
		return violations, nil
	}

	for _, validator := range validators {
		newViolations, err := validator.Validate(v.Resources)
		if err != nil {
			cumulativeErr = errors.Join(cumulativeErr, err)
			v.logger.Error(err, validator.GetName())
		}

		if len(newViolations) > 0 {
			violations = append(violations, newViolations...)
		}
	}

	return violations, cumulativeErr
}

func (v *Validation) readAdditionalResourceTypes(dir string) ([]schema.GroupVersionResource, error) {
	var additionalResourceTypes []schema.GroupVersionResource

	additionalResourceTypesFullPath := filepath.Join(dir, additionalResourceTypesFile)

	content, err := afero.ReadFile(v.appFs, additionalResourceTypesFullPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			v.logger.V(0).Info("couldn't find additional resource types file", additionalResourceTypesFullPath)
			return nil, nil
		}
		v.logger.Error(err, "couldn't read additional resource types file")
		return nil, err
	} else {
		err := yaml.Unmarshal(content, &additionalResourceTypes)
		if err != nil {
			v.logger.Error(err, "couldn't parse additional resource types file")
			return nil, err
		}
	}

	return additionalResourceTypes, nil
}

func resolveConfigDirectory() string {
	result := defaultConfigDirectory
	configDirectoryOverride := os.Getenv("CONFIG_DIR")
	if configDirectoryOverride != "" {
		result = configDirectoryOverride
	}
	return result
}

/*
	returns true if validation should be aborted. E.g. resources are currently being deployed.

return values:

	shouldAbort bool - true if should abort
	reason string    - reason for abortion
	err error        - in case an error occurred
*/
func (v *Validation) shouldAbortValidation(ctx context.Context, client K8SProvider) (bool, string, error) {
	configMap, err := client.ClientSet.CoreV1().ConfigMaps(v.AbortValidationConfigMapNamespace).Get(ctx, v.AbortValidationConfigMapName, metav1.GetOptions{})
	if err != nil {
		// if configMap not present, we perform validation anyway
		if k8sErrors.IsNotFound(err) {
			return false, fmt.Sprintf("Abort configMap %s not found: Resuming validation", v.AbortValidationConfigMapName), nil
		}
		return false, "", err
	}

	shouldAbort, ok := configMap.Data[v.AbortValidationConfigMapField]
	if !ok {
		// if field not present, we perform validation anyway
		return false, fmt.Sprintf("Field %s not found in abort configMap: Resuming validation", v.AbortValidationConfigMapField), nil
	}

	if string(shouldAbort) == "true" {
		message := fmt.Sprintf("Abort configMap %s set to \"true\": Aborting validation", v.AbortValidationConfigMapName)
		return true, message, nil
	}

	message := fmt.Sprintf("Abort configMap %s found, but field %s is NOT set to \"true\": Resuming validation", v.AbortValidationConfigMapName, v.AbortValidationConfigMapField)
	return false, message, nil
}

func (v *Validation) loadConfiguration() {
	k := koanf.New(".")

	content, err := afero.ReadFile(v.appFs, filepath.Join(resolveConfigDirectory(), configFileName))
	if err != nil {
		return
	}
	k.Load(rawbytes.Provider(content), koanfYaml.Parser())

	if k.String("exempt.labelName") != "" {
		common.ExemptPodLabelName = k.String("exempt.labelName")
	}

	if k.String("exempt.labelValue") != "" {
		common.ExemptPodLabelValue = k.String("exempt.labelValue")
	}

	if k.String("abort.configMapName") != "" {
		v.AbortValidationConfigMapName = k.String("abort.configMapName")
	}

	if k.String("abort.configMapNamespace") != "" {
		v.AbortValidationConfigMapNamespace = k.String("abort.configMapNamespace")
	}

	if k.String("abort.configMapField") != "" {
		v.AbortValidationConfigMapField = k.String("abort.configMapField")
	}
}
