package validation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var alreadyTriedToConnect bool = false
var kubeconfig string

func init() {
	kubeconfig = os.Getenv("KUBECONFIG")
}

type K8SProvider struct {
	dynamic   dynamic.Interface
	clientSet kubernetes.Interface
}

func getClient() (*K8SProvider, error) {
	var provider K8SProvider

	if !alreadyTriedToConnect {
		// ensure flag.String() is allocated only once to avoid resource exhaustion
		alreadyTriedToConnect = true

		configIsResolved := false
		// create the in-cluster config
		config, err := rest.InClusterConfig()
		if err == nil {
			configIsResolved = true
		}

		// ...if that fails, create an out-of-cluster config based on KUBECONFIG env var
		if !configIsResolved {
			if len(kubeconfig) > 0 {
				config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
				if err == nil {
					configIsResolved = true
				}
			}
		}

		// ...if that fails, create an out-of-cluster config based on ~/.kube/config file
		if !configIsResolved {
			if home := homedir.HomeDir(); home != "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			} else {
				kubeconfig = ""
			}

			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return nil, err
			}
		}

		provider.dynamic, err = dynamic.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		provider.clientSet, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}

	}
	return &provider, nil
}

func fetchResourcesOfKind(ctx context.Context, client K8SProvider, gvr schema.GroupVersionResource) []unstructured.Unstructured {
	logger, _ := logr.FromContext(ctx)
	resources, err := client.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error(err, "failed to list resource", gvr.Resource)
		return nil
	} else {
		logger.V(2).Info(fmt.Sprintf("there are %d %s in the cluster", len(resources.Items), gvr.Resource))
		return resources.Items
	}
}

func fetchResources(ctx context.Context, client K8SProvider, additionalResourceTypes []schema.GroupVersionResource) []unstructured.Unstructured {
	var gvr schema.GroupVersionResource
	var allResources, resources []unstructured.Unstructured
	gvr = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	resources = fetchResourcesOfKind(ctx, client, gvr)
	allResources = append(allResources, resources...)

	gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	resources = fetchResourcesOfKind(ctx, client, gvr)
	allResources = append(allResources, resources...)

	gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	resources = fetchResourcesOfKind(ctx, client, gvr)
	allResources = append(allResources, resources...)

	gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	resources = fetchResourcesOfKind(ctx, client, gvr)
	allResources = append(allResources, resources...)

	gvr = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "replicationcontrollers"}
	resources = fetchResourcesOfKind(ctx, client, gvr)
	allResources = append(allResources, resources...)

	gvr = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	resources = fetchResourcesOfKind(ctx, client, gvr)
	allResources = append(allResources, resources...)

	gvr = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	resources = fetchResourcesOfKind(ctx, client, gvr)
	allResources = append(allResources, resources...)

	gvr = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
	resources = fetchResourcesOfKind(ctx, client, gvr)
	allResources = append(allResources, resources...)

	for _, s := range additionalResourceTypes {
		gvr = schema.GroupVersionResource{Group: s.Group, Version: s.Version, Resource: s.Resource}
		resources = fetchResourcesOfKind(ctx, client, gvr)
		allResources = append(allResources, resources...)
	}

	return allResources
}
