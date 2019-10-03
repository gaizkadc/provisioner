package certmngr

import (
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
	"path"
	"strings"
	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const CertManagerYAMLPrefix = "cert-manager."

const ProductionLetsEncryptURL = "https://acme-v02.api.letsencrypt.org/directory"
const StagingLetsEncryptURL = "https://acme-staging-v02.api.letsencrypt.org/directory"

const LetsEncryptURLEntry = "LETS_ENCRYPT_URL"
const ClientIDEntry = "CLIENT_ID"
const SubscritionIDEntry = "SUBSCRIPTION_ID"
const TenantIDEntry = "TENANT_ID"
const ResourceGroupNameEntry = "RESOURCE_GROUP_NAME"
const DNSZoneEntry = "DNS_ZONE"
const ClusterNameEntry = "CLUSTER_NAME"

const AzureCertificateIssuerTemplate = `
apiVersion: certmanager.k8s.io/v1alpha1
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

const CertificateTemplate = `
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
 name: ingress-tls
 namespace: nalej
spec:
  secretName: ingress-tls
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
	config     *config.Config
	Kubernetes *k8s.Kubernetes
	kubeConfigFilePath *string
}

// NewCertManagerHelper creates a helper to install the certificate manager.
func NewCertManagerHelper(config *config.Config) *CertManagerHelper {
	return &CertManagerHelper{
		config: config,
	}
}

// Connect establishes the connection with the target Kubernetes infrastructure.
func (cmh *CertManagerHelper) Connect(rawKubeConfig string) derrors.Error{
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
	// List of files
	fileInfo, err := ioutil.ReadDir(cmh.config.ResourcesPath)
	if err != nil {
		log.Fatal().Err(err).Str("resourcesPath", cmh.config.ResourcesPath).Msg("cannot read resources dir")
	}
	targetFiles := make([]string, 0)
	for _, file := range fileInfo {
		if strings.HasPrefix(file.Name(), CertManagerYAMLPrefix) && strings.HasSuffix(file.Name(), ".yaml"){
			targetFiles = append(targetFiles, file.Name())
		}
	}
	// Now trigger the install process of all involved YAML
	var installErr derrors.Error
	for index := 0; index < len(targetFiles) && installErr == nil; index ++{
		installErr = cmh.installCertManagerFile(targetFiles[index])
	}
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

// requestCertificate creates the required entities in the cluster to request and issue a
// certificate.
func (cmh *CertManagerHelper) RequestCertificateIssuerOnAzure(
	clientID string, clientSecret string, subscriptionID string, tenantID string,
	resourceGroupName string,
	dnsZone string,
	isProduction bool) derrors.Error{
	// First create the secret that is used to validate the creation
	err := cmh.createServicePrincipalSecretOnAzure(clientSecret)
	if err != nil{
		return err
	}
	// Then create the Issuer that will generate the secret.
	err = cmh.createCertificateIssuerOnAzure(clientID, subscriptionID, tenantID, resourceGroupName, dnsZone, isProduction)
	return nil
}

// createServicePrincipalSecret creates a secret in Kubernetes that enables the cert manager to
// validate and process the issuing of a new certificate.
func (cmh *CertManagerHelper) createServicePrincipalSecretOnAzure(
	clientSecret string) derrors.Error{
	opaqueSecret := &v1.Secret{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:         "k8s-service-principal",
			Namespace:    "cert-manager",
		},
		Data: map[string][]byte{
			"client-secret": []byte(clientSecret),
		},
		Type: v1.SecretTypeOpaque,
	}
	err := cmh.Kubernetes.Create(opaqueSecret)
	if err != nil{
		return err
	}
	return nil
}

// createCertificateIssuerOnAzure creates a ClusterIssuer entity tailored for Azure to generate the certificate.
func (cmh *CertManagerHelper) createCertificateIssuerOnAzure(
	clientID string, subscriptionID string, tenantID string,
	resourceGroupName string,
	dnsZone string,
	isProduction bool) derrors.Error{

	letsEncryptURL := ProductionLetsEncryptURL
	if !isProduction{
		letsEncryptURL = StagingLetsEncryptURL
	}
	// Update the template
	toCreate := strings.ReplaceAll(AzureCertificateIssuerTemplate, LetsEncryptURLEntry, letsEncryptURL)
	toCreate = strings.ReplaceAll(toCreate, ClientIDEntry, clientID)
	toCreate = strings.ReplaceAll(toCreate, SubscritionIDEntry, subscriptionID)
	toCreate = strings.ReplaceAll(toCreate, TenantIDEntry, tenantID)
	toCreate = strings.ReplaceAll(toCreate, ResourceGroupNameEntry, resourceGroupName)
	toCreate = strings.ReplaceAll(toCreate, DNSZoneEntry, dnsZone)

	return cmh.Kubernetes.CreateUnstructure(toCreate)

}

// checkCertificateIssuer waits for the certificate to be issued by the authority
func (cmh *CertManagerHelper) CheckCertificateIssuer() derrors.Error{
	issued, err := cmh.Kubernetes.MatchCRDStatus(
		"certmanager.k8s.io",
		"v1alpha1",
		"clusterissuers", "letsencrypt",
		[]string{"status", "conditions", "0", "reason"}, "ACMEAccountRegistered")
	if err != nil{
		return err
	}
	log.Debug().Bool("issued", *issued).Msg("Certificate state")
	return nil
}

// CreateCertificate creates a new certificate request for a given cluster and dnsZone
func (cmh *CertManagerHelper) CreateCertificate(clusterName string, dnsZone string) derrors.Error{
	toCreate := strings.ReplaceAll(CertificateTemplate, DNSZoneEntry, dnsZone)
	toCreate = strings.ReplaceAll(toCreate, ClusterNameEntry, clusterName)
	return cmh.Kubernetes.CreateUnstructure(toCreate)
}