package freshness

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/SAP/k8s-resource-validator/pkg/common"
)

const ValidatorName = "built-in:freshness"

func NewFreshnessValidator(ctx context.Context, freshnessThresholdInHours int32) common.Validator {
	response := FreshnessValidator{freshnessThresholdInHours: freshnessThresholdInHours, ctx: ctx}
	response.logger, _ = logr.FromContext(ctx)
	return &response
}

type FreshnessValidator struct {
	freshnessThresholdInHours int32
	ctx                       context.Context
	logger                    logr.Logger
}

func (v *FreshnessValidator) GetName() string {
	return ValidatorName
}

/*
*
 */
func (v *FreshnessValidator) Validate(resources []unstructured.Unstructured) (violations []common.Violation, err error) {
	pods := common.GetPods(resources)

	for _, p := range pods {
		if common.IsExempt(p) {
			v.logger.V(2).Info(fmt.Sprint("is exempt from checking for freshness:", p.GetNamespace(), p.GetName()))
		} else {
			podIsStale := isPodStale(p, v.freshnessThresholdInHours)
			if podIsStale {
				violation := common.NewViolation(p, "Pod is stale", 1, ValidatorName)
				violations = append(violations, violation)
			} else {
				v.logger.V(3).Info(fmt.Sprintf("pod is fresh: %s/%s", p.GetNamespace(), p.GetName()))
			}
		}
	}

	return
}

func isPodStale(pod unstructured.Unstructured, freshnessThresholdInHours int32) bool {
	creationTimestamp := pod.GetCreationTimestamp()
	if time.Time.IsZero(creationTimestamp.Time) {
		// don't report stale pod if creation time is not set (e.g. when running tests)
		return false
	}
	diff := metav1.Now().Sub(creationTimestamp.Time)
	return diff.Hours() > float64(freshnessThresholdInHours)
}
