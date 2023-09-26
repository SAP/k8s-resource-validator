package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.tools.sap/I034929/k8s-resource-validator/pkg/common"
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
	preValidated                      bool
}

func NewValidation(ctx context.Context) (*Validation, error) {
	response := Validation{}
	response.ctx = ctx
	response.appFs = ctx.Value(common.FileSystemContextKey).(afero.Fs)

	response.loadConfiguration()

	response.preValidated = false

	// set default values
	response.AbortValidationConfigMapField = "deploying"
	response.AbortValidationConfigMapName = "landscape-state"
	response.AbortValidationConfigMapNamespace = "center"

	return &response, nil
}

/*
If this function is not called, a new client will be created in preValidate()
*/
func (v *Validation) SetClient(client *K8SProvider) {
	v.Client = client
}

func (v *Validation) SetAbortFunc(abortFunc common.AbortFunc) {
	v.abortFunc = abortFunc
}

func (v *Validation) preValidate() (aborted bool) {
	if !v.preValidated {
		if v.Client == nil {
			var err error
			v.Client, err = getClient()
			if err != nil {
				v.logger.V(0).Info("unable to create client", "error", err)
				panic(err)
			}
		}

		configDir := resolveConfigDirectory()

		additionalResourceTypes := v.readAdditionalResourceTypes(configDir)

		v.Resources = fetchResources(v.ctx, *v.Client, additionalResourceTypes)

		if v.abortFunc == nil {
			shouldAbort, abortMessage := v.shouldAbortValidation(v.ctx, *v.Client)
			v.logger.V(2).Info(abortMessage)
			if shouldAbort {
				return true
			}
		} else {
			return v.abortFunc()
		}

		v.preValidated = true
	}

	return false
}

/*
returns a slice of violations (empty if no violations are found)
*/
func (v *Validation) Validate(validators []common.Validator) []common.Violation {
	aborted := v.preValidate()

	var violations []common.Violation
	if aborted {
		return violations
	}

	for _, validator := range validators {
		newViolations, err := validator.Validate(v.ctx, v.Resources)
		if err == nil && len(newViolations) != 0 {
			violations = append(violations, newViolations...)
		}
		if err != nil {
			v.logger.V(1).Info("", "error", err)
		}
	}

	return violations
}

func (v *Validation) readAdditionalResourceTypes(dir string) []schema.GroupVersionResource {
	var additionalResourceTypes []schema.GroupVersionResource

	additionalResourceTypesFullPath := filepath.Join(dir, additionalResourceTypesFile)

	content, err := afero.ReadFile(v.appFs, additionalResourceTypesFullPath)
	if err != nil {
		v.logger.V(2).Info("couldn't find additional resource types file", "file", additionalResourceTypesFullPath)
		return nil
	} else {
		err := yaml.Unmarshal(content, &additionalResourceTypes)
		if err != nil {
			v.logger.V(0).Info("couldn't parse additional resource types file:", "error", err)
			return nil
		}
	}

	return additionalResourceTypes
}

func resolveConfigDirectory() string {
	result := defaultConfigDirectory
	configDirectoryOverride := os.Getenv("CONFIG_DIR")
	if configDirectoryOverride != "" {
		result = configDirectoryOverride
	}
	return result
}

// returns true if validation should be aborted. E.g. resources are currently being deployed.
func (v *Validation) shouldAbortValidation(ctx context.Context, client K8SProvider) (bool, string) {
	configMap, err := client.clientSet.CoreV1().ConfigMaps(v.AbortValidationConfigMapNamespace).Get(ctx, v.AbortValidationConfigMapName, metav1.GetOptions{})
	if err != nil {
		// if configMap not present, we perform validation anyway
		return false, fmt.Sprintf("Abort configMap %s not found: Resuming validation", v.AbortValidationConfigMapName)
	}

	shouldAbort, ok := configMap.Data[v.AbortValidationConfigMapField]
	if !ok {
		// if field not present, we perform validation anyway
		return false, fmt.Sprintf("Field %s not found in abort configMap: Resuming validation", v.AbortValidationConfigMapField)
	}

	if string(shouldAbort) == "true" {
		message := fmt.Sprintf("Abort configMap %s set to \"true\": Aborting validation", v.AbortValidationConfigMapName)
		return true, message
	}

	message := fmt.Sprintf("Abort configMap %s found, but field %s is NOT set to \"true\": Resuming validation", v.AbortValidationConfigMapName, v.AbortValidationConfigMapField)
	return false, message
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
