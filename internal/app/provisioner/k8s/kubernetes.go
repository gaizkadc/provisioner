/*
 * Copyright 2019 Nalej
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package k8s

import (
	"github.com/nalej/derrors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"k8s.io/api/core/v1"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/tidwall/gjson"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"net"
)

// TODO This file is a copy from the file in the installer, a refactor to extract our own library to facilitate
// using Kubernetes should be created.
type Kubernetes struct {
	KubeConfigPath string                `json:"kubeConfigPath"`
	Client         *kubernetes.Clientset `json:"-"`

	// Discovery client for REST mapper to use, so we can figure out
	// the right endpoints for reserves
	discoveryClient *discovery.DiscoveryClient
	// Dynamic client used to create all resources
	dynClient dynamic.Interface
}

func NewKubernetesClient(kubeConfigPath string) *Kubernetes {
	return &Kubernetes{
		KubeConfigPath: kubeConfigPath,
	}
}

func (k *Kubernetes) Connect() derrors.Error {
	config, err := clientcmd.BuildConfigFromFlags("", k.KubeConfigPath)
	if err != nil {
		log.Error().Err(err).Msg("error building configuration from kubeconfig")
		return derrors.AsError(err, "error building configuration from kubeconfig")
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error().Err(err).Msg("error using configuration to build k8s clientset")
		return derrors.AsError(err, "error using configuration to build k8s clientset")
	}

	k.Client = clientset

	// Create the discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return derrors.NewInternalError("failed to create discovery client", err)
	}
	k.discoveryClient = discoveryClient

	// Create the dynamic client that can be used to create any object
	// by specifying what resource we're dealing with by using the REST mapper
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return derrors.NewInternalError("failed to create dynamic client", err)
	}
	k.dynClient = dynClient

	return nil
}

func (k *Kubernetes) ResolveIP(address string) ([]string, derrors.Error) {
	result := make([]string, 0)
	ips, err := net.LookupIP(address)
	if err != nil {
		log.Error().Err(err).Str("address", address).Msg("cannot resolve IP address")
		return nil, derrors.AsError(err, "cannot resolve IP address")
	}
	for _, ip := range ips {
		if len(ip) == net.IPv4len {
			result = append(result, ip.String())
		}
	}
	return result, nil
}

func (k *Kubernetes) ExistsNamespace(name string) (bool, derrors.Error) {
	namespaceClient := k.Client.CoreV1().Namespaces()
	opts := metaV1.ListOptions{}
	list, err := namespaceClient.List(opts)
	if err != nil {
		return false, derrors.AsError(err, "cannot obtain the namespace list")
	}
	found := false
	for _, n := range list.Items {
		log.Debug().Interface("n", n).Msg("A namespace")
		if n.Name == name {
			found = true
			break
		}
	}
	return found, nil
}

func (k *Kubernetes) CreateNamespace(name string) derrors.Error {
	toCreate := v1.Namespace{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name: name,
		},
	}
	err := k.Create(&toCreate)
	if err != nil {
		return derrors.AsError(err, "cannot create namespace")
	}

	return nil
}

func (k *Kubernetes) CreateNamespaceIfNotExists(name string) derrors.Error {
	found, fErr := k.ExistsNamespace(name)
	if fErr != nil {
		return fErr
	}

	if !found {
		err := k.CreateNamespace(name)
		if err != nil {
			return err
		}
	} else {
		log.Debug().Str("namespace", name).Msg("namespace already exists")
	}
	return nil
}

func (k *Kubernetes) CreateUnstructure(toCreate string) derrors.Error {
	reader := strings.NewReader(toCreate)
	// We use a YAML decoder to decode the resource straight into an
	// unstructured object. This way, we can deal with resources that are
	// not known to this client - like CustomResourceDefinitions
	obj := runtime.Object(&unstructured.Unstructured{})

	yamlDecoder := yaml.NewYAMLOrJSONDecoder(reader, 1024)
	err := yamlDecoder.Decode(obj)
	if err != nil {
		return derrors.NewInvalidArgumentError("cannot parse component file", err)
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	log.Debug().Str("resource", gvk.String()).Msg("decoded resource")

	// Now let's see if it's a resource we know and can type, so we can
	// decide if we need to do some modifications. We ignore the error
	// because that just means we don't have the specific implementation of
	// the resource type and that's ok
	clientScheme := scheme.Scheme
	typed, _ := scheme.Scheme.New(gvk)
	if typed != nil {
		// Ah, we can convert this to something specific to deal with!
		err := clientScheme.Convert(obj, typed, nil)
		if err != nil {
			return derrors.NewInternalError("cannot convert resource to specific type", err)
		}
	}

	return k.Create(obj)
}

func (k *Kubernetes) Create(obj runtime.Object) derrors.Error {
	// Create unstructured object
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return derrors.NewInvalidArgumentError("cannot convert object to unstructured", err).WithParams(obj)
	}
	unstructuredObj := &unstructured.Unstructured{
		Object: unstructuredMap,
	}

	gvk, derr := getKind(obj)
	if derr != nil {
		return derr
	}

	// Items in list resources need to be sent to the server one by one
	if unstructuredObj.IsList() {
		log.Debug().Str("resource", gvk.String()).Msg("creating each item in list resource")
		list, err := unstructuredObj.ToList()
		if err != nil {
			return derrors.NewInternalError("cannot create unstructured list", err)
		}
		err = list.EachListItem(func(obj runtime.Object) error { return k.Create(obj) })
		if err != nil {
			derr, ok := err.(derrors.Error)
			if ok {
				return derr
			}
			return derrors.NewInternalError("failed to create list item resource", err)
		}
		log.Debug().Str("resource", gvk.String()).Msg("created all items in list resource")
		return nil
	}

	// Create the REST mapper through a discovery client
	// We do this every time we create a resource, because if we created
	// a custom resource definition in a previous step, we need to
	// update the list of supported resources.
	resources, err := restmapper.GetAPIGroupResources(k.discoveryClient)
	if err != nil {
		return derrors.NewInternalError("failed to get api group resources", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(resources)

	// Get the right REST endpoint through the mapper
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return derrors.NewInternalError("unable to get REST mapping for object", err).WithParams(unstructuredObj)
	}

	var client dynamic.ResourceInterface
	namespace := unstructuredObj.GetNamespace()
	if namespace != "" {
		client = k.dynClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		client = k.dynClient.Resource(mapping.Resource)
	}

	log.Debug().Interface("obj", unstructuredObj).Msg("creating resource")

	created, err := client.Create(unstructuredObj, metaV1.CreateOptions{})
	if err != nil {
		return derrors.NewInternalError("unable to create object", err).WithParams(unstructuredObj)
	}

	log.Debug().Str("resource", created.GetSelfLink()).Interface("groupVersionKind", created.GroupVersionKind()).Msg("created")

	return nil
}

// We're using this function instead of just looking at the apiVersion and
// kind defined in the object so that we don't necessarily have to define
// those in typed objects. For unstructured objects, ObjectKinds will look
// at those anyway, and for typed objects we'll look at the object type.
func getKind(obj runtime.Object) (schema.GroupVersionKind, derrors.Error) {
	kinds, _, err := scheme.Scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, derrors.NewInvalidArgumentError("invalid object received")
	}

	// Not sure what to do if an object matches multiple kinds, let's
	// at least warn
	if len(kinds) > 1 {
		kindLog := log.Warn()
		for _, k := range kinds {
			kindLog = kindLog.Str("candidate", k.String())
		}
		kindLog.Msg("received ambiguous object, picking first candidate")
	}

	kind := kinds[0]
	log.Debug().Interface("groupVersionKind", kind).Msg("getKind")

	return kind, nil
}

// MatchUnstructureField matches a json path as defined by the gjson package with a given expected value.
func (k *Kubernetes) MatchUnstructuredField(obj *unstructured.Unstructured, key []string, expected string) bool {
	json, err := obj.MarshalJSON()
	if err != nil {
		return false
	}
	result := gjson.Get(string(json), strings.Join(key, "."))
	return result.String() == expected
}

// MatchCRDStatus retrieves a non-namespaced CRD, and checks if a set of keys matches a given value.
func (k *Kubernetes) MatchCRDStatus(namespace string, group string, version string, resource string, name string, key []string, expected string) (*bool, derrors.Error) {
	resourceRequest := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	log.Debug().Interface("resourceRequest", resourceRequest).Msg("MatchCRDStatus")

	var client dynamic.ResourceInterface
	if namespace == "" {
		client = k.dynClient.Resource(resourceRequest)
	} else {
		client = k.dynClient.Resource(resourceRequest).Namespace(namespace)
	}
	numRetries := 36
	issued := false
	for retry := 0; retry < numRetries && !issued; retry++ {
		unstructure, err := client.Get(name, metaV1.GetOptions{})
		if err != nil {
			log.Warn().Err(err).Msg("unable to retrieve resource")
		} else {
			log.Debug().Interface("raw", unstructure.Object).Msg("resource retrieved")
			matches := k.MatchUnstructuredField(unstructure, key, expected)
			log.Debug().Bool("match", matches).Msg("CRD status")
			if matches {
				return &matches, nil
			}
		}
		if !issued {
			time.Sleep(20 * time.Second)
		}
	}

	return &issued, nil
}

// WaitCRDStatus checks the status of an existing
func (k *Kubernetes) WaitCRDStatus(namespace string, group string, version string, resource string, name string,
	key []string, expected string, checkout time.Duration, timeout time.Duration) (derrors.Error) {
	ticker := time.NewTicker(checkout)
	exit := time.NewTimer(timeout)
	for {
		select{
		case <- ticker.C:
			ok, err := k.MatchCRDStatus(namespace, group, version, resource, name, key, expected)
			log.Debug().Msg("checking CRD status...")
			if *ok {
				return nil
			}
			if err != nil {
				return err
			}
		case <- exit.C:
			log.Error().Msg("timeout reached when waiting for CRD status")
			return derrors.NewInternalError("timeout reached when waiting for CRD status")
		}
	}
}
