package common

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	KIND_POD                    = "Pod"
	KIND_REPLICATION_CONTROLLER = "ReplicationController"
	KIND_DEPLOYMENT             = "Deployment"
	KIND_REPLICA_SET            = "ReplicaSet"
	KIND_DAEMON_SET             = "DaemonSet"
	KIND_STATEFUL_SET           = "StatefulSet"
	KIND_JOB                    = "Job"
	KIND_CRON_JOB               = "CronJob"
)

var (
	ExemptPodLabelName  = "resources.gardener.cloud/managed-by"
	ExemptPodLabelValue = "gardener"

	errUnableToFindOwner = errors.New("couldn't find owner references")
)

func GetOwnerReferences(resources []unstructured.Unstructured, item unstructured.Unstructured) ([]metav1.OwnerReference, error) {
	idx := IndexFunc(resources, func(p unstructured.Unstructured) bool {
		namespace := p.GetNamespace()
		name := p.GetName()
		kind := p.GetKind()
		return item.GetKind() == kind && item.GetName() == name && item.GetNamespace() == namespace
	})

	if idx > -1 {
		return resources[idx].GetOwnerReferences(), nil
	}

	return nil, errUnableToFindOwner
}

func GetPods(resources []unstructured.Unstructured) []unstructured.Unstructured {
	var pods []unstructured.Unstructured
	for _, s := range resources {
		if s.GetKind() == KIND_POD {
			pods = append(pods, s)
		}
	}
	return pods
}

func IndexFunc[E any](s []E, f func(E) bool) int {
	for i, v := range s {
		if f(v) {
			return i
		}
	}
	return -1
}

func IsExempt(resource unstructured.Unstructured) bool {
	// TODO: support multiple keys?
	key := ExemptPodLabelName
	values := []string{ExemptPodLabelValue}
	var requirementLabels labels.Set = resource.GetLabels()
	requirement, _ := labels.NewRequirement(key, selection.Equals, values)
	matches := requirement.Matches(requirementLabels)
	return matches
}

func NewViolation(resource unstructured.Unstructured, message string, level int, validatorName string) Violation {
	response := Violation{Resource: &resource, Level: level, Message: message, ValidatorName: validatorName}
	return response
}

type ViolationTarget struct {
	Kind      string
	Name      string
	Namespace string
	Group     string
}

func GetViolationsGroupedByResource(violations []Violation) [][]Violation {
	tempMap := make(map[string][]Violation)

	for _, violation := range violations {
		key := fmt.Sprintf("%s-%s-%s-%s",
			violation.Resource.GetKind(),
			violation.Resource.GetName(),
			violation.Resource.GetNamespace(),
			violation.Resource.GroupVersionKind().Group,
		)

		existing, ok := tempMap[key]
		if ok {
			tempMap[key] = append(existing, violation)
		} else {
			tempMap[key] = []Violation{violation}
		}
	}

	response := make([][]Violation, 0, len(tempMap))

	for _, violationsPerResource := range tempMap {
		response = append(response, violationsPerResource)
	}
	return response
}
