/*
This file is provided as a sample.

# This project is intended to be used as a Go library

See k8s-resource-validator-app
*/
package main

import (
	"context"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/afero"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/SAP/k8s-resource-validator/pkg/validation"
	"github.com/SAP/k8s-resource-validator/pkg/validators/allowed_pods"
	"github.com/SAP/k8s-resource-validator/pkg/validators/freshness"
	"github.com/SAP/k8s-resource-validator/pkg/validators/privileged_pods"
	"github.com/SAP/k8s-resource-validator/pkg/validators/readiness"
)

func main() {
	// initialize context with logger and file system
	ctx := context.Background()
	stdr.SetVerbosity(2)
	// 0 verbosity indicates a k8s-resource-validator error (e.g. unable to load config file)
	// 1 verbosity indicates info-level violations
	// 2 verbosity indicates a k8s-resource-validator info
	// k8s-resource-validator violations are written to log as errors
	logger := stdr.New(stdlog.New(os.Stderr, "", stdlog.LstdFlags))

	ctx = logr.NewContext(ctx, logger)
	appFs := afero.NewOsFs()
	ctx = context.WithValue(ctx, common.FileSystemContextKey, appFs)

	// create validation instance
	validationInstance, err := validation.NewValidation(ctx)
	if err != nil {
		panic(err)
	}

	// load configuration
	configDirectory := "."
	configDirectoryOverride := os.Getenv("CONFIG_DIR")
	if configDirectoryOverride != "" {
		configDirectory = configDirectoryOverride
	}

	k := koanf.New(".")

	k.Load(file.Provider(filepath.Join(configDirectory, "config.yaml")), yaml.Parser())

	var freshnessThresholdInHours int32 = 24 * 28 // 4 weeks
	if k.Int("freshness.thresholdInHours") != 0 {
		freshnessThresholdInHours = int32(k.Int("freshness.thresholdInHours"))
	}

	// instantiate relevant validators
	readinessValidator := readiness.NewReadinessValidator(ctx, configDirectory, false)
	freshnessValidator := freshness.NewFreshnessValidator(ctx, freshnessThresholdInHours)
	allowedPodsValidator := allowed_pods.NewAllowedPodsValidator(ctx, configDirectory)
	privilegedPodsValidator := privileged_pods.NewPrivilegedPodsValidator(ctx)

	validatorList := []common.Validator{
		allowedPodsValidator,
		readinessValidator,
		freshnessValidator,
		privilegedPodsValidator,
	}

	// optionally set an abort function
	// validationInstance.SetAbortFunc(func() bool {
	// 	return len(validationInstance.Resources) == 0 // place your custom abort logic here
	// })

	// perform the validation
	violations := validationInstance.Validate(validatorList)

	// aggregate violations
	aggregatedViolations := getCustomViolations(violations)

	// log violations
	validation.Log(ctx, aggregatedViolations, 0)
}

// perform custom post-validation manipulation, before sending violations to logger
func getCustomViolations(originalViolations []common.Violation) []common.Violation {
	customViolations := []common.Violation{}
	violationsGroupedByResource := common.GetViolationsGroupedByResource(originalViolations)
	for _, violationsPerResource := range violationsGroupedByResource {
		if len(violationsPerResource) == 1 {
			// ignore privileged pods if it's the only violation by a specific resource
			if violationsPerResource[0].ValidatorName != "built-in:privileged-pods" {
				customViolations = append(customViolations, violationsPerResource[0])
			}
		} else if len(violationsPerResource) > 1 {
			// elevate the violation level, if a specific resource caused more than one violation
			aggregatedViolation := common.Violation{}
			var sb strings.Builder
			for i, aViolation := range violationsPerResource {
				if i > 0 {
					sb.WriteString("+")
				}
				sb.WriteString(aViolation.ValidatorName)
			}
			aggregatedViolation.Resource = violationsPerResource[0].Resource
			aggregatedViolation.Level = 0 // set to highest severity
			aggregatedViolation.ValidatorName = sb.String()
			aggregatedViolation.Message = "aggregated violation"
			customViolations = append(customViolations, aggregatedViolation)
		}
	}

	return customViolations
}
