package common

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	// "github.com/SAP/k8s-resource-validator/pkg/validators/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	configDirectory string
	appFs           afero.Fs
	logger          logr.Logger
	ctx             context.Context
)

func TestCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = "tests.xml"
	RunSpecs(t, "Common Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("Utils", func() {
	BeforeEach(func() {
		configDirectory = "/config/"
	})

	Describe("Test utils", func() {
		BeforeEach(func() {
			ctx = context.Background()
			appFs = afero.NewMemMapFs()
			logger = testr.New(&testing.T{})
			ctx = logr.NewContext(ctx, logger)
			ctx = context.WithValue(ctx, FileSystemContextKey, appFs)
		})

		It("resource is exempt", func() {
			ExemptPodLabelName = "label1"
			ExemptPodLabelValue = "exempt"
			var labels map[string]string = make(map[string]string)
			labels["label1"] = "exempt"

			resource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_POD,
					"metadata": map[string]interface{}{
						"name":      "name",
						"namespace": "namespace",
					},
				},
			}

			resource.SetLabels(labels)

			Expect(IsExempt(resource)).To(BeTrue())
		})

		It("resource is not exempt", func() {
			resource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_POD,
					"metadata": map[string]interface{}{
						"name":      "name",
						"namespace": "namespace",
					},
				},
			}

			Expect(IsExempt(resource)).To(BeFalse())
		})

		It("new violation", func() {
			resource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_POD,
					"metadata": map[string]interface{}{
						"name":      "name",
						"namespace": "namespace",
					},
				},
			}

			violationMessage := "message"
			violationLevel := 5
			validatorName := "validatorName"
			violation := NewViolation(resource, violationMessage, violationLevel, validatorName)

			Expect(violation.Level).To(Equal(violationLevel))
			Expect(violation.Message).To(Equal(violationMessage))
			Expect(violation.ValidatorName).To(Equal(violation.ValidatorName))
			Expect(violation.Resource.GetName()).To(Equal(violation.Resource.GetName()))
		})

		It("get pods", func() {
			podResource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_POD,
					"metadata": map[string]interface{}{
						"name":      "name",
						"namespace": "namespace",
					},
				},
			}

			otherResource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_REPLICA_SET,
					"metadata": map[string]interface{}{
						"name":      "name",
						"namespace": "namespace",
					},
				},
			}

			resources := []unstructured.Unstructured{podResource, otherResource}

			pods := GetPods(resources)
			Expect(len(pods)).To(Equal(1))
		})

		It("get owner references", func() {
			resource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_POD,
					"metadata": map[string]interface{}{
						"name":      "name",
						"namespace": "namespace",
					},
				},
			}

			ownerName := "owner"
			ownerResource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_REPLICA_SET,
					"metadata": map[string]interface{}{
						"name":      ownerName,
						"namespace": "namespace",
					},
				},
			}

			owner1 := metav1.OwnerReference{
				Kind: KIND_REPLICA_SET,
				Name: ownerName,
			}

			resource.SetOwnerReferences([]metav1.OwnerReference{owner1})

			resources := []unstructured.Unstructured{resource, ownerResource}

			foundOwner, err := GetOwnerReferences(resources, resource)
			Expect(err).To(Succeed())
			Expect(foundOwner[0].Name).To(Equal(ownerName))
		})

		It("group violations by resource", func() {
			validatorNameA := "a"
			validatorNameB := "b"

			resource1 := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_POD,
					"metadata": map[string]interface{}{
						"name":      "pod1",
						"namespace": "namespace",
					},
				},
			}

			violation1 := Violation{Resource: &resource1, ValidatorName: validatorNameA}

			resource2 := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       KIND_POD,
					"metadata": map[string]interface{}{
						"name":      "pod2",
						"namespace": "namespace",
					},
				},
			}

			violation2 := Violation{Resource: &resource2, ValidatorName: validatorNameA}

			violation3 := Violation{Resource: &resource2, ValidatorName: validatorNameB}

			violations := []Violation{violation1, violation2, violation3}
			groupedViolations := GetViolationsGroupedByResource(violations)
			Expect(len(groupedViolations)).To(Equal(2))
		})
	})
})
