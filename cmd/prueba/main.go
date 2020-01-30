package main

import (
    "github.com/rs/zerolog/log"
    "github.com/tidwall/gjson"
    metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/tools/clientcmd"
)

func main() {

    KubeConfigPath := "/Users/juan/nalej/credentials/jmmaster60.yaml"

    config, err := clientcmd.BuildConfigFromFlags("", KubeConfigPath)
    if err != nil {
        log.Error().Err(err).Msg("error building configuration from kubeconfig")
        log.Panic().Msg("error building configuration from kubeconfig")
    }

    // create the clientset
    /*
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Error().Err(err).Msg("error using configuration to build k8s clientset")
        log.Panic().Msg("error using configuration to build k8s clientset")
    }
    */

    // Create the discovery client
    /*
    discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
    if err != nil {
        log.Panic().Msg("failed to create discovery client")
    }
    k.discoveryClient = discoveryClient
    */

    // Create the dynamic client that can be used to create any object
    // by specifying what resource we're dealing with by using the REST mapper
    dynClient, err := dynamic.NewForConfig(config)
    if err != nil {
        log.Panic().Msg("failed to create dynamic client")
    }


    namespace := "cert-manager"
    group :="apps"
    version :="v1"
    resource := "deployments"
    name := "cert-manager-webhook"


    resourceRequest := schema.GroupVersionResource{
        Group:    group,
        Version:  version,
        Resource: resource,
    }

    var client dynamic.ResourceInterface
    client = dynClient.Resource(resourceRequest).Namespace(namespace)
    structure, err := client.Get(name, metaV1.GetOptions{})
    if err != nil {
        log.Panic().Err(err).Msg("there was an error")
    }

    log.Info().Interface("---->", structure).Msg("output")

    json, err := structure.MarshalJSON()
    result := gjson.Get(string(json), "status.conditions.0.type")
    log.Info().Msgf("---------->%s", result)



}
