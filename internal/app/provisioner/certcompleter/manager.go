/*
 * Copyright 2020 Nalej
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

package certcompleter

import (
    "bytes"
    "github.com/nalej/derrors"
    "github.com/nalej/provisioner/internal/app/provisioner/k8s"
    "github.com/nalej/provisioner/internal/pkg/config"
    "github.com/rs/zerolog/log"
    "io/ioutil"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/util/yaml"
    "k8s.io/client-go/kubernetes/scheme"
    "os"
    "time"
)

// The cert completer is a workaround provided by github.com/erwinvaneyk/cert-completer
// to fill the ca.crt field in the secrets generated with cert-manager using ACME.
// There is an ongoing issue/discussion github.com/jetstack/cert-manager/issues/2111 that
// affects the fill of the ca.crt field whe using certmanager with ACME with the new
// versions of cert-manager. There is not a clear decision about how to proceed in this moment.
// Right now this additional components seem to be a reasonable temporary workaround.

//CertCompleterYAMLFile name of the file with the yaml configuration
const CertCompleterYAMLFile = "cert-completer.yaml"

//YamlSeparator is the string to be used when splitting yaml files into chunks
const YamlSeparator = "---"


// CertCompleterHelper structure to install the cert completer solution on the freshly installed cluster.
type CertCompleterHelper struct {
    config             *config.Config
    Kubernetes         *k8s.Kubernetes
    kubeConfigFilePath *string
}

// NewCertCompleterHelper creates a helper to install the certificate completer manager.
func NewCertCompleterHelper(config *config.Config) *CertCompleterHelper {
    return &CertCompleterHelper{
        config: config,
    }
}

// Connect establishes the connection with the target Kubernetes infrastructure.
func (cch *CertCompleterHelper) Connect(rawKubeConfig string) derrors.Error {
    kubeConfigFile, err := cch.writeTempFile(rawKubeConfig, "kc")
    if err != nil {
        return err
    }
    cch.kubeConfigFilePath = kubeConfigFile
    cch.Kubernetes = k8s.NewKubernetesClient(*kubeConfigFile)
    // Connect to Kubernetes
    err = cch.Kubernetes.Connect()
    if err != nil {
        return err
    }
    return nil
}

func (cch *CertCompleterHelper) InstallCertCompleter() derrors.Error {
    // Be sure the cert manager yaml configuration file is there
    filePath := cch.config.ResourcesPath+"/"+CertCompleterYAMLFile
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        log.Error().Err(err).Msg("cert completer configuration could not be found")
        return derrors.NewFailedPreconditionError("cert completer configuration could not be found", err)
    }

    yamlContent, ioErr := ioutil.ReadFile(filePath)
    if ioErr != nil {
        log.Error().Err(ioErr).Msg("error reading cert-completer configuration file")
        return derrors.NewInternalError("error reading cert-completer configuration file", ioErr)
    }

    chunks := bytes.Split(yamlContent,[]byte(YamlSeparator))
    log.Debug().Int("number of chunks", len(chunks)).Msg("number of chunks in the file")
    for _, chunk := range chunks {
        err := cch.installCertCompleterConfigEntry(chunk)
        if err != nil {
            return err
        }
    }
    log.Info().Msg("waiting for cert-completer to be up and ready...")
    waitErr := cch.Kubernetes.WaitCRDStatus("default", "apps", "v1",
        "deployments", "cert-completer-controller-manager", []string{"status", "conditions", "0","type"}, "Available",
        time.Second * 20, time.Minute * 3)
    if waitErr != nil {
        return waitErr
    }
    return nil
}

// installCertManagerConfigEntry takes an object definition from a yaml file and runs the associated k8s command.
func (cch *CertCompleterHelper) installCertCompleterConfigEntry(chunk []byte) derrors.Error {
    // We use a YAML decoder to decode the resource straight into an
    // unstructured object. This way, we can deal with resources that are
    // not known to this client - like CustomResourceDefinitions
    obj := runtime.Object(&unstructured.Unstructured{})
    yamlDecoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(chunk), 1024)
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
    return cch.Kubernetes.Create(obj)
}

// writeTempFile creates a temporal file to store the kubeconfig file. This is done due to the
// available clients in Kubernetes not having a create-client-from-buffer or similar.
func (cch *CertCompleterHelper) writeTempFile(content string, prefix string) (*string, derrors.Error) {
    tmpfile, err := ioutil.TempFile(cch.config.TempPath, prefix)
    if err != nil {
        return nil, derrors.AsError(err, "cannot create temporal file")
    }
    _, err = tmpfile.Write([]byte(content))
    if err != nil {
        return nil, derrors.AsError(err, "cannot write temporal file")
    }
    err = tmpfile.Close()
    if err != nil {
        return nil, derrors.AsError(err, "cannot close temporal file")
    }
    tmpName := tmpfile.Name()
    return &tmpName, nil
}
