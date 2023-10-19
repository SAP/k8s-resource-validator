package validation

import (
	"context"
	"fmt"
	"os"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/go-logr/logr"
)

const LOGGER_NAME string = "k8s-k8s-resource-validator"

/*
Logger verbosity levels (info):
	0 - k8s-resource-validator errors (e.g. unable to load config file)
	1 - certain* validation violations
	2 - k8s-resource-validator info (e.g. number of pods found)

Logger errors:
	certain* validation violations are written to log as errors

*Violation levels:
	violation levels below a given threshold are written to log as errors
	other violation levels are written to log as info with verbosity level 1
*/

/*
Log() writes errors to a logger.
Its main purpose is to group validation errors.
This is useful when the logger is remote a service, and minimizing traffic to the logging service is desirable.

ctx is expected to contain the logger

thresholdLevelForErrors specifies that violations of this level or below are considered errors; others are info
*/
func Log(ctx context.Context, violations []common.Violation, thresholdLevelForErrors int) {
	logger, err := logr.FromContext(ctx)
	if len(violations) == 0 {
		if err == nil {
			logger.WithName(LOGGER_NAME).V(2).Info("all resources are valid")
		} else {
			fmt.Fprint(os.Stdout, "all resources are valid")
		}
		return
	}

	var errorKeysAndValues []interface{}
	var infoKeysAndValues []interface{}

	for _, violation := range violations {
		resourceString := fmt.Sprintf("name: %s; namespace: %s, kind: %s",
			violation.Resource.GetName(),
			violation.Resource.GetNamespace(),
			violation.Resource.GetKind(),
		)
		violationString := fmt.Sprintf("validator: %s; message: %s",
			violation.ValidatorName,
			violation.Message,
		)

		if violation.Level <= thresholdLevelForErrors {
			errorKeysAndValues = append(errorKeysAndValues, "resource", resourceString)
			errorKeysAndValues = append(errorKeysAndValues, "violation", violationString)
		} else {
			infoKeysAndValues = append(infoKeysAndValues, "resource", resourceString)
			infoKeysAndValues = append(infoKeysAndValues, "violation", violationString)
		}
	}

	if len(errorKeysAndValues) > 0 {
		if err == nil {
			logger.WithName(LOGGER_NAME).Error(nil, "error violations", errorKeysAndValues...)
		} else {
			fmt.Print("error violations", errorKeysAndValues)
		}

	}
	if len(infoKeysAndValues) > 0 {
		if err == nil {
			logger.WithName(LOGGER_NAME).V(1).Info("info violations", infoKeysAndValues...)
		} else {
			fmt.Fprint(os.Stdout, infoKeysAndValues)
		}
	}
}
