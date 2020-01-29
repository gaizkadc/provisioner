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

package certmngr

import (
	"bytes"
	"fmt"
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/app/provisioner/k8s"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"strings"
)

//ClientCertificate is the name used by both the Certificate resource and the Secret for the TLS client certificate
const ClientCertificate = "tls-client-certificate"

//CACertificate is the name used by the Secret resource for the TLS CA Certificate
const CACertificate = "ca-certificate"

//CertManagerYAMLFile is the name that contains the cert-manager configuration
const CertManagerYAMLFile = "cert-manager.yaml"

//ProductionLetsEncryptURL to register a Let's encrypt account in their production environment
const ProductionLetsEncryptURL = "https://acme-v02.api.letsencrypt.org/directory"

//ProductionLetsEncryptCA is the filename that contains the CA certificate for letsencrypt in their production environment
const ProductionLetsEncryptCA = "letsencrypt_prod.pem"

//StagingLetsEncryptURL to register a Let's encrypt account in their staging environment
const StagingLetsEncryptURL = "https://acme-staging-v02.api.letsencrypt.org/directory"

//StagingLetsEncryptCA is the filename that contains the CA certificate for letsencrypt in their staging environment
const StagingLetsEncryptCA = "letsencrypt_staging.pem"

//LetsEncryptURLEntry is the placeholder to replace the Let's encrypt URL
const LetsEncryptURLEntry = "LETS_ENCRYPT_URL"

//ClientIDEntry is the placeholder to replace the Azure Service Principal Client ID
const ClientIDEntry = "CLIENT_ID"

//SubscriptionIDEntry is the placeholder to replace the Azure Subscription ID
const SubscriptionIDEntry = "SUBSCRIPTION_ID"

//TenantIDEntry is the placeholder to replace the Azure AD Tenant ID
const TenantIDEntry = "TENANT_ID"

//ResourceGroupNameEntry is the placeholder for the DNS Resource Group name
const ResourceGroupNameEntry = "RESOURCE_GROUP_NAME"

//DNSZoneEntry is the placeholder for the DNS Zone name
const DNSZoneEntry = "DNS_ZONE"

//ClusterNameEntry is the placeholder fot the cluster name
const ClusterNameEntry = "CLUSTER_NAME"

//ClientCertificateEntry is the placeholder for the TLS client certificate name
const ClientCertificateEntry = "CLIENT_CERTIFICATE_NAME"

//AzureCertificateIssuerTemplate to create a ClusterIssuer resource for Azure
const AzureCertificateIssuerTemplate = `
apiVersion: certmanager.io/v1alpha2
kind: ClusterIssuer
metadata:
  name: letsencrypt
spec:
  acme:
    server: LETS_ENCRYPT_URL
    email: jarvis@nalej.com
    privateKeySecretRef:
      name: letsencrypt
    dns01:
      providers:
        - name: azuredns
          azuredns:
            clientID: CLIENT_ID
            clientSecretSecretRef:
              name: k8s-service-principal
              key: client-secret
            subscriptionID: SUBSCRIPTION_ID
            tenantID: TENANT_ID
            resourceGroupName: RESOURCE_GROUP_NAME
            hostedZoneName: DNS_ZONE
`

//CertificateTemplate to create a Certificate resource
const CertificateTemplate = `
apiVersion: certmanager.io/v1alpha2
kind: Certificate
metadata:
 name: CLIENT_CERTIFICATE_NAME
 namespace: nalej
spec:
  secretName: CLIENT_CERTIFICATE_NAME
  issuerRef:
    name: letsencrypt
    kind: ClusterIssuer
  dnsNames:
    - '*.CLUSTER_NAME.DNS_ZONE'
  acme:
    config:
      - dns01:
          provider: azuredns
        domains:
          - '*.CLUSTER_NAME.DNS_ZONE'
`

// CertManagerHelper structure to install the cert manager on the freshly installed cluster.
type CertManagerHelper struct {
	config             *config.Config
	Kubernetes         *k8s.Kubernetes
	kubeConfigFilePath *string
}

// NewCertManagerHelper creates a helper to install the certificate manager.
func NewCertManagerHelper(config *config.Config) *CertManagerHelper {
	return &CertManagerHelper{
		config: config,
	}
}

// Connect establishes the connection with the target Kubernetes infrastructure.
func (cmh *CertManagerHelper) Connect(rawKubeConfig string) derrors.Error {
	kubeConfigFile, err := cmh.writeTempFile(rawKubeConfig, "kc")
	if err != nil {
		return err
	}
	cmh.kubeConfigFilePath = kubeConfigFile
	cmh.Kubernetes = k8s.NewKubernetesClient(*kubeConfigFile)
	// Connect to Kubernetes
	err = cmh.Kubernetes.Connect()
	if err != nil {
		return err
	}
	return nil
}

// Destroy will cleanup the temporal structures.
func (cmh *CertManagerHelper) Destroy() {
	// remove the file once the cert manager install finishes
	cmh.cleanupTempFile(*cmh.kubeConfigFilePath)
}

// InstallCertManager installs the cert manager on a given cluster.
func (cmh *CertManagerHelper) InstallCertManager() derrors.Error {
	// Be sure the cert manager yaml configuration file is there
	filePath := cmh.config.ResourcesPath+"/"+CertManagerYAMLFile
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Error().Err(err).Msg("cert manager configuration could not be found")
		return derrors.NewFailedPreconditionError("cert manager configuration could not be found", err)
	}

	yamlContent, ioErr := ioutil.ReadFile(filePath)
	if ioErr != nil {
		log.Error().Err(ioErr).Msg("error reading cert-manager configuration file")
		return derrors.NewInternalError("error reading cert-manager configuration file", ioErr)
	}

	chunks := bytes.Split(yamlContent,[]byte(yamlSeparator))
	log.Debug().Int("number of chunks", len(chunks)).Msg("number of chunks in the file")
	for _, chunk := range chunks {
		err := cmh.installCertManagerConfigEntry(chunk)
		if err != nil {
			return err
		}
	}

	/*
	scanner := bufio.NewScanner(bytes.NewReader(yamlContent))
	scanner.Split(splitYAMLDocument)

	// Process every token in the file
	for scanner.Scan() {
		chunk := scanner.Bytes()
		err := cmh.installCertManagerConfigEntry(chunk)
		if err != nil {
			return err
		}
	}
	*/

	return nil
}



const yamlSeparator = "---"

// splitYAMLDocument is a bufio.SplitFunc for splitting YAML streams into individual documents.
// This function is taken from the Kubernetes api
func splitYAMLDocument(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	sep := len([]byte(yamlSeparator))
	if i := bytes.Index(data, []byte(yamlSeparator)); i >= 0 {
		// We have a potential document terminator
		i += sep
		after := data[i:]
		if len(after) == 0 {
			// we can't read any more characters
			if atEOF {
				return len(data), data[:len(data)-sep], nil
			}
			return 0, nil, nil
		}
		if j := bytes.IndexByte(after, '\n'); j >= 0 {
			return i + j + 1, data[0 : i-sep], nil
		}
		return 0, nil, nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}


// installCertManagerConfigEntry takes an object definition from a yaml file and runs the associated k8s command.
func (cmh *CertManagerHelper) installCertManagerConfigEntry(chunk []byte) derrors.Error {
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
	return cmh.Kubernetes.Create(obj)
}

/*
// InstallCertManager installs the cert manager on a given cluster.
func (cmh *CertManagerHelper) InstallCertManager() derrors.Error {
	// List of files
	fileInfo, err := ioutil.ReadDir(cmh.config.ResourcesPath)
	if err != nil {
		log.Fatal().Err(err).Str("resourcesPath", cmh.config.ResourcesPath).Msg("cannot read resources dir")
	}
	targetFiles := make([]string, 0)
	for _, file := range fileInfo {
		if strings.HasPrefix(file.Name(), CertManagerYAMLPrefix) && strings.HasSuffix(file.Name(), ".yaml") {
			targetFiles = append(targetFiles, file.Name())
		}
	}
	// Now trigger the install process of all involved YAML
	var installErr derrors.Error
	for index := 0; index < len(targetFiles) && installErr == nil; index++ {
		installErr = cmh.installCertManagerFile(targetFiles[index])
	}
	// We let cert-manager to start itself inside the cluster
	// TODO: Provide a more accurate way to detect that cert-manager is ready to receive operations
	time.Sleep(30 * time.Second)
	return installErr
}

// installCertManager triggers the installation of the cert manager YAML
func (cmh *CertManagerHelper) installCertManagerFile(fileName string) derrors.Error {
	certManagerPath := path.Join(cmh.config.ResourcesPath, fileName)
	f, err := os.Open(certManagerPath)
	if err != nil {
		return derrors.NewPermissionDeniedError("cannot read component file", err)
	}
	defer f.Close()
	// We use a YAML decoder to decode the resource straight into an
	// unstructured object. This way, we can deal with resources that are
	// not known to this client - like CustomResourceDefinitions
	obj := runtime.Object(&unstructured.Unstructured{})
	yamlDecoder := yaml.NewYAMLOrJSONDecoder(f, 1024)
	err = yamlDecoder.Decode(obj)
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
	return cmh.Kubernetes.Create(obj)
}
*/

// cleanupTempFile removes the temporal file storing the kubeconfig file.
func (cmh *CertManagerHelper) cleanupTempFile(path string) {
	log.Debug().Str("path", path).Msg("removing temporal file")
	err := os.Remove(path)
	if err != nil {
		log.Error().Err(err).Msg("cannot delete temporal file")
	}
}

// writeTempFile creates a temporal file to store the kubeconfig file. This is done due to the
// available clients in Kubernetes not having a create-client-from-buffer or similar.
func (cmh *CertManagerHelper) writeTempFile(content string, prefix string) (*string, derrors.Error) {
	tmpfile, err := ioutil.TempFile(cmh.config.TempPath, prefix)
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

// RequestCertificateIssuerOnAzure creates the required entities in the cluster to request and issue a
// certificate.
func (cmh *CertManagerHelper) RequestCertificateIssuerOnAzure(
	clientID string, clientSecret string, subscriptionID string, tenantID string,
	resourceGroupName string,
	dnsZone string,
	isProduction bool) derrors.Error {
	// First create the secret that is used to validate the creation
	err := cmh.createServicePrincipalSecretOnAzure(clientSecret)
	if err != nil {
		return err
	}
	// Then create the Issuer that will generate the secret.
	return cmh.createCertificateIssuerOnAzure(clientID, subscriptionID, tenantID, resourceGroupName, dnsZone, isProduction)
}

// createServicePrincipalSecretOnAzure creates a secret in Kubernetes that enables the cert manager to
// validate and process the issuing of a new certificate.
func (cmh *CertManagerHelper) createServicePrincipalSecretOnAzure(
	clientSecret string) derrors.Error {
	opaqueSecret := &v1.Secret{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "k8s-service-principal",
			Namespace: "cert-manager",
		},
		Data: map[string][]byte{
			"client-secret": []byte(clientSecret),
		},
		Type: v1.SecretTypeOpaque,
	}
	err := cmh.Kubernetes.Create(opaqueSecret)
	if err != nil {
		return err
	}
	return nil
}

// createCertificateIssuerOnAzure creates a ClusterIssuer entity tailored for Azure to generate the certificate.
func (cmh *CertManagerHelper) createCertificateIssuerOnAzure(
	clientID string, subscriptionID string, tenantID string,
	resourceGroupName string,
	dnsZone string,
	isProduction bool) derrors.Error {

	letsEncryptURL := ProductionLetsEncryptURL
	if !isProduction {
		letsEncryptURL = StagingLetsEncryptURL
	}
	// Update the template
	toCreate := strings.ReplaceAll(AzureCertificateIssuerTemplate, LetsEncryptURLEntry, letsEncryptURL)
	toCreate = strings.ReplaceAll(toCreate, ClientIDEntry, clientID)
	toCreate = strings.ReplaceAll(toCreate, SubscriptionIDEntry, subscriptionID)
	toCreate = strings.ReplaceAll(toCreate, TenantIDEntry, tenantID)
	toCreate = strings.ReplaceAll(toCreate, ResourceGroupNameEntry, resourceGroupName)
	toCreate = strings.ReplaceAll(toCreate, DNSZoneEntry, dnsZone)

	return cmh.Kubernetes.CreateUnstructure(toCreate)

}

// CheckCertificateIssuer waits for the certificate to be issued by the authority
func (cmh *CertManagerHelper) CheckCertificateIssuer() derrors.Error {
	issued, err := cmh.Kubernetes.MatchCRDStatus(
		"", "certmanager.k8s.io",
		"v1alpha1",
		"clusterissuers", "letsencrypt",
		[]string{"status", "conditions", "0", "reason"}, "ACMEAccountRegistered")
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("unable to check certificate issuer")
		return err
	}
	log.Debug().Bool("issued", *issued).Msg("Certificate issuer")
	if !*issued {
		return derrors.NewFailedPreconditionError("invalid state for certificate issuer")
	}
	return nil
}

// CreateCertificate creates a new certificate request for a given cluster and dnsZone
func (cmh *CertManagerHelper) CreateCertificate(clusterName string, dnsZone string) derrors.Error {
	err := cmh.Kubernetes.CreateNamespaceIfNotExists("nalej")
	if err != nil {
		return err
	}
	toCreate := strings.ReplaceAll(CertificateTemplate, DNSZoneEntry, dnsZone)
	toCreate = strings.ReplaceAll(toCreate, ClusterNameEntry, clusterName)
	toCreate = strings.ReplaceAll(toCreate, ClientCertificateEntry, ClientCertificate)
	return cmh.Kubernetes.CreateUnstructure(toCreate)
}

//ValidateCertificate validates if a certificate has been issued successfully
func (cmh *CertManagerHelper) ValidateCertificate() derrors.Error {
	issued, err := cmh.Kubernetes.MatchCRDStatus(
		"nalej", "certmanager.k8s.io",
		"v1alpha1",
		"certificates", ClientCertificate,
		[]string{"status", "conditions", "0", "reason"}, "Ready")
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("unable to check cluster certificate")
		return err
	}
	log.Debug().Bool("issued", *issued).Msg("cluster certificate")
	if !*issued {
		return derrors.NewFailedPreconditionError("invalid state for cluster certificate")
	}
	return nil
}

func (cmh *CertManagerHelper) readCAfile(isProduction bool) ([]byte, derrors.Error) {
	var caFileName string
	if isProduction == true {
		caFileName = ProductionLetsEncryptCA
	} else {
		caFileName = StagingLetsEncryptCA
	}
	caFilePath := fmt.Sprintf("%s/ca/%s", cmh.config.ResourcesPath, caFileName)
	caContents, err := ioutil.ReadFile(caFilePath)
	if err != nil {
		derrors.NewInternalError("cannot read CA certificate file", err)
	}
	return caContents, nil
}

//CreateCASecret creates the Secret K8S resource to store the CA certificate
func (cmh *CertManagerHelper) CreateCASecret(isProduction bool) derrors.Error {
	caContents, err := cmh.readCAfile(isProduction)
	if err != nil {
		return err
	}

	err = cmh.Kubernetes.CreateNamespaceIfNotExists("nalej")
	if err != nil {
		return err
	}

	opaqueSecret := &v1.Secret{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      CACertificate,
			Namespace: "nalej",
		},
		Data: map[string][]byte{
			"ca.crt": caContents,
		},
		Type: v1.SecretTypeOpaque,
	}
	err = cmh.Kubernetes.Create(opaqueSecret)
	if err != nil {
		return err
	}

	return nil
}
