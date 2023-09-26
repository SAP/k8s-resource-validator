package test_utils

import (
	"github.tools.sap/I034929/k8s-resource-validator/pkg/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CreateUnstructuredPodResource(isPrivileged bool, name string, namespace string, containerName string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       common.KIND_POD,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"containers": []map[string]interface{}{
					{
						"name": containerName,
						"securityContext": map[string]interface{}{
							"privileged": isPrivileged,
						},
					},
				},
			},
		},
	}
}
