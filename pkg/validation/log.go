package validation

import (
	"context"
	"fmt"
	"os"

	"github.com/SAP/k8s-resource-validator/pkg/common"
	"github.com/go-logr/logr"
)

const LOGGER_NAME string = "k8s-resource-validator"

/*
LogViolations() writes violations to a logger.
It is a convenience method. You may choose to write violations to the log in a different format.

ctx is expected to contain the logger

thresholdLevelForErrors specifies that violations of this level or below are considered errors; others are info
*/
func LogViolations(ctx context.Context, violations []common.Violation, thresholdLevelForErrors int) {
	logger, err := logr.FromContext(ctx)
	if len(violations) == 0 {
		if err == nil {
			logger.WithName(LOGGER_NAME).V(2).Info("all resources are valid")
		} else {
			fmt.Fprint(os.Stdout, "all resources are valid")
		}
		return
	}

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
			if err == nil {
				logger.WithName(LOGGER_NAME).Error(nil, "error violations", resourceString, violationString)
			} else {
				fmt.Print("error violations", resourceString, violationString)
			}
		} else {
			if err == nil {
				logger.WithName(LOGGER_NAME).V(violation.Level).Info("info violations", resourceString, violationString)
			} else {
				fmt.Fprint(os.Stdout, resourceString, violationString)
			}
		}
	}
}
