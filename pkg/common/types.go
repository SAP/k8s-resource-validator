package common

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type FileSystemContextKeyType string
const FileSystemContextKey FileSystemContextKeyType = "fs"

type Validator interface {
	/*
		The return violations slice is non-nil if invalid resources were found
		The return error is non-nil if another type of error was encountered
	*/
	Validate(ctx context.Context, resources []unstructured.Unstructured) (violations []Violation, err error)
	GetName() string
}

type Violation struct {
	Message       string                    // an error describing the violation
	Resource      *unstructured.Unstructured // the violating resource
	Level         int                       // verbosity level: 0 is the most severe
	ValidatorName string
}


type AbortFunc func() bool
