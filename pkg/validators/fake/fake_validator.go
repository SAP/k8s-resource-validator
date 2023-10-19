package fake

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/SAP/k8s-resource-validator/pkg/common"
)

func NewFakeValidator(ctx context.Context, numberOfViolations int) common.Validator {
	response := FakeValidator{ctx: ctx, numberOfViolations: numberOfViolations}
	response.logger, _ = logr.FromContext(ctx)
	return &response
}

type FakeValidator struct {
	ctx                context.Context
	logger             logr.Logger
	numberOfViolations int
}

/*
*
 */
func (v *FakeValidator) Validate(ctx context.Context, resources []unstructured.Unstructured) (violations []common.Violation, err error) {
	for i := 0; i < v.numberOfViolations; i++ {
		resource := unstructured.Unstructured{}
		resource.SetName(fmt.Sprintf("%d", i))
		resource.SetNamespace("fake")
		resource.SetKind("Fake")
		violation := common.NewViolation(resource, "Fake resource violation", 1, v.GetName())
		violations = append(violations, violation)
	}

	return
}

func (v *FakeValidator) GetName() string {
	return "built-in:fake"
}
