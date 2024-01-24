package fake

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/SAP/k8s-resource-validator/pkg/common"
)

const ValidatorName = "built-in:fake"

func NewFakeValidator(ctx context.Context, numberOfViolations int, shouldFailWithError bool) (common.Validator, error) {
	response := FakeValidator{ctx: ctx, numberOfViolations: numberOfViolations, shouldFailWithError: shouldFailWithError}

	var err error
	response.logger, err = logr.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

type FakeValidator struct {
	ctx                 context.Context
	logger              logr.Logger
	numberOfViolations  int
	shouldFailWithError bool
}

func (v *FakeValidator) GetName() string {
	return ValidatorName
}

/*
*
 */
func (v *FakeValidator) Validate(resources []unstructured.Unstructured) (violations []common.Violation, err error) {
	if v.shouldFailWithError {
		return nil, errors.New("fake error")
	}

	for i := 0; i < v.numberOfViolations; i++ {
		resource := unstructured.Unstructured{}
		resource.SetName(fmt.Sprintf("%d", i))
		resource.SetNamespace("fake")
		resource.SetKind("Fake")
		violation := common.NewViolation(resource, "Fake resource violation", 1, ValidatorName)
		violations = append(violations, violation)
	}

	return
}
