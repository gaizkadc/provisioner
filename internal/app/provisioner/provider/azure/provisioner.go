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

package azure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-08-01/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/app/provisioner/certmngr"
	"github.com/nalej/provisioner/internal/pkg/common"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
)

// OsType with the operating system type to be provisioned.
const OsType = "Linux"

// ClusterCreateDeadline with the deadline to install the new cluster on the Azure requests.
const ClusterCreateDeadline = 30 * time.Minute

// ManagementIPAddressNames with the list of addresses to be created in the management cluster.
var ManagementIPAddressNames = []string{entities.IngressIPAddressName, entities.DNSPublicIPAddress, entities.CoreDNSPublicIPAddress, entities.VPNServerPublicIPAddress}

// ApplicationIPAddressNames with the list of addresses to be created in the application cluster
var ApplicationIPAddressNames = []string{entities.IngressIPAddressName}

// ProvisionerOperation structure with the methods required to performing a provisioning for a new Kubernetes
// cluster in Azure.
type ProvisionerOperation struct {
	*AzureOperation
	request           entities.ProvisionRequest
	result            *entities.ProvisionResult
	config            *config.Config
	certManagerHelper *certmngr.CertManagerHelper
}

// NewProvisionerOperation creates a new Azure provisioning operation.
func NewProvisionerOperation(credentials *AzureCredentials, request entities.ProvisionRequest, config *config.Config) (*ProvisionerOperation, derrors.Error) {
	azureOp, err := NewAzureOperation(credentials)
	if err != nil {
		return nil, err
	}
	return &ProvisionerOperation{
		AzureOperation: azureOp,
		request:        request,
		result: &entities.ProvisionResult{
			ClusterName: request.ClusterName,
			Hostname:    fmt.Sprintf("%s.%s", request.ClusterName, request.AzureOptions.DNSZoneName),
		},
		config:            config,
		certManagerHelper: certmngr.NewCertManagerHelper(config),
	}, nil
}

// RequestID returns the request identifier associated with this operation
func (po ProvisionerOperation) RequestID() string {
	return po.request.RequestID
}

// Metadata returns the operation associated metadata
func (po ProvisionerOperation) Metadata() entities.OperationMetadata {
	return entities.OperationMetadata{
		OrganizationID: po.request.OrganizationID,
		ClusterID:      po.request.ClusterID,
		RequestID:      po.request.RequestID,
	}
}

func (po ProvisionerOperation) notifyError(err derrors.Error, callback func(requestId string)) {
	log.Error().Str("trace", err.DebugReport()).Msg("operation failed")
	po.setError(err.Error())
	callback(po.request.RequestID)
}

// Execute triggers the execution of the operation. The callback function on the execute is expected to be
// called when the operation finish its execution independently of the status.
func (po ProvisionerOperation) Execute(callback func(requestId string)) {
	log.Debug().Str("organizationID", po.request.OrganizationID).Str("clusterID", po.request.ClusterID).Str("clusterName", po.request.ClusterName).Str("resultClusterName", po.result.ClusterName).Msg("executing provisioning operation")
	po.started = time.Now()
	po.SetProgress(entities.InProgress)

	createdCluster, err := po.createAKSCluster()
	if err != nil {
		po.notifyError(err, callback)
		return
	}

	po.AddToLog(fmt.Sprintf("New cluster has been created with an associated resource group named as %s", *createdCluster.NodeResourceGroup))
	log.Debug().Msg("cluster is ready, creating the IP addresses")
	err = po.createAssociatedIPAddresses(*createdCluster.NodeResourceGroup)
	if err != nil {
		po.notifyError(err, callback)
		return
	}
	log.Debug().Msg("IP address have been reserved")
	po.AddToLog("IP address have been reserved")
	po.AddToLog("Obtaining DNS zone information")
	zone, err := po.getDNSZone(po.request.AzureOptions.DNSZoneName)
	if err != nil {
		po.notifyError(err, callback)
		return
	}
	log.Debug().Interface("zone", zone).Msg("Target zone details")
	po.AddToLog("Creating DNS entries")
	dnsZoneResourceGroupName, err := po.getDNSResourceGroupName(zone)
	if err != nil {
		po.notifyError(err, callback)
		return
	}

	if po.request.IsManagementCluster {
		err = po.createManagementDNSEntries(*dnsZoneResourceGroupName)
	} else {
		err = po.createApplicationDNSEntries(*dnsZoneResourceGroupName)
	}
	if err != nil {
		po.notifyError(err, callback)
		return
	}
	po.AddToLog("DNS entries have been defined")

	err = po.installCertManager()
	if err != nil {
		po.notifyError(err, callback)
		return
	}
	defer po.certManagerHelper.Destroy()
	po.AddToLog("Cert manager has been installed")

	err = po.requestCertificateIssuer(*dnsZoneResourceGroupName)
	if err != nil {
		po.notifyError(err, callback)
		return
	}
	po.AddToLog("certificate issuer requested")
	err = po.certManagerHelper.CheckCertificateIssuer()
	if err != nil {
		po.notifyError(err, callback)
		return
	}
	log.Debug().Msg("certificate issuer available")
	err = po.requestCertificate()
	if err != nil {
		po.notifyError(err, callback)
		return
	}
	po.AddToLog("validating cluster certificate")
	err = po.certManagerHelper.ValidateCertificate()
	if err != nil {
		po.notifyError(err, callback)
		return
	}

	if po.request.IsManagementCluster {
		po.AddToLog("Adding CA certificate")
		err = po.certManagerHelper.CreateCASecret(po.request.IsProduction)
		if err != nil {
			po.notifyError(err, callback)
			return
		}
		po.AddToLog("Added CA certificate as a secret")
	}

	log.Debug().Msg("provisioning finished")
	po.elapsedTime = time.Now().Sub(po.started).Nanoseconds()
	po.SetProgress(entities.Finished)
	callback(po.request.RequestID)
	return
}

// Cancel triggers the cancellation of the operation
func (po ProvisionerOperation) Cancel() derrors.Error {
	panic("implement me")
}

// setResultIP sets the resulting IP address
func (po ProvisionerOperation) setResultIP(addressName string, IP *network.PublicIPAddress) {
	po.result.SetIPAddress(addressName, *IP.IPAddress)
}

// Result returns the operation result if this operation is successful
func (po ProvisionerOperation) Result() entities.OperationResult {
	elapsed := po.elapsedTime
	if po.elapsedTime == 0 && po.taskProgress == entities.InProgress {
		// If the operation is in progress, retrieved the ongoing time.
		elapsed = time.Now().Sub(po.started).Nanoseconds()
	}

	// TODO Fix with the final result
	return entities.OperationResult{
		RequestId:       po.request.RequestID,
		Type:            entities.Provision,
		Progress:        po.taskProgress,
		ElapsedTime:     elapsed,
		ErrorMsg:        po.errorMsg,
		ProvisionResult: po.result,
	}
}

func (po ProvisionerOperation) getManagedClusterAgentProfile(numNodes *int32, vmSize *containerservice.VMSizeTypes, vmSubnetID *string) containerservice.ManagedClusterAgentPoolProfile {
	agentProfile := containerservice.ManagedClusterAgentPoolProfile{
		Name:         StringAsPTR("nalejpool"),
		Count:        numNodes,
		VMSize:       *vmSize,
		OsDiskSizeGB: Int32AsPTR(0),
		//VnetSubnetID:           vmSubnetID,
		VnetSubnetID: nil,
		// MaxPods not set to obtain the default value.
		MaxPods: nil,
		OsType:  OsType,
		// MaxCount not set due to autoscaling disabled
		MaxCount: nil,
		// MinCount not set due to autoscaling disabled.
		MinCount:            nil,
		EnableAutoScaling:   BoolAsPTR(false),
		Type:                containerservice.AvailabilitySet,
		OrchestratorVersion: StringAsPTR(po.request.KubernetesVersion),
		AvailabilityZones:   nil,
		EnableNodePublicIP:  BoolAsPTR(false),
		// ScaleSetPriority not set to use the default value (Regular).
		ScaleSetPriority: "",
		// ScaleSetEvictionPolicy not set use the default (Delete).
		ScaleSetEvictionPolicy: "",
		// NodeTaints not used.
		NodeTaints: nil,
	}
	return agentProfile
}

func (po ProvisionerOperation) getLinuxProperties() *containerservice.LinuxProfile {
	return &containerservice.LinuxProfile{
		// AdminUsername set to the default azure value to facilitate other admin tasks.
		AdminUsername: StringAsPTR("azureuser"),
		SSH:           nil,
	}
}

// getManagedClusterServicePrincipalProfile returns the service principal required to provision a new cluster.
func (po ProvisionerOperation) getManagedClusterServicePrincipalProfile() *containerservice.ManagedClusterServicePrincipalProfile {
	return &containerservice.ManagedClusterServicePrincipalProfile{
		ClientID: StringAsPTR(po.credentials.ClientId),
		Secret:   StringAsPTR(po.credentials.ClientSecret),
	}
}

// getNetworkProfileType returns the network profile of a new provisioned cluster.
func (po ProvisionerOperation) getNetworkProfileType() *containerservice.NetworkProfileType {
	return &containerservice.NetworkProfileType{
		NetworkPlugin:       "Kubenet",
		NetworkPolicy:       "",
		PodCidr:             nil,
		ServiceCidr:         nil,
		DNSServiceIP:        nil,
		DockerBridgeCidr:    nil,
		LoadBalancerSku:     "Basic",
		LoadBalancerProfile: nil,
	}
}

// getKubernetesCreateRequest creates the ManagedCluster object required to create a new AKS cluster.
func (po ProvisionerOperation) getKubernetesCreateRequest() (*containerservice.ManagedCluster, derrors.Error) {
	tags := make(map[string]*string, 0)
	tags["clusterName"] = StringAsPTR(po.getClusterName(po.request.ClusterName))
	tags["organizationID"] = StringAsPTR(po.request.OrganizationID)
	tags["clusterID"] = StringAsPTR(po.request.ClusterID)
	tags["created-by"] = StringAsPTR("Nalej")

	numNodes, err := Int64ToInt32(po.request.NumNodes)
	if err != nil {
		return nil, err
	}

	vmSize, err := po.getAzureVMSize(po.request.NodeType)
	if err != nil {
		return nil, err
	}
	dnsPrefix := po.getDNSPrefix(po.request.ClusterID)
	vmSubnetID := po.getVMSubnetID(po.request.ClusterID)

	agentProfile := po.getManagedClusterAgentProfile(numNodes, vmSize, &vmSubnetID)
	agentProfiles := []containerservice.ManagedClusterAgentPoolProfile{agentProfile}

	properties := &containerservice.ManagedClusterProperties{
		KubernetesVersion: StringAsPTR(po.request.KubernetesVersion),
		DNSPrefix:         &dnsPrefix,
		AgentPoolProfiles: &agentProfiles,
		// LinuxProfile not set as SSH access is not required
		LinuxProfile: nil,
		// WindowsProfile not set.
		WindowsProfile: nil,
		// ServicePrincipalProfile associated with the cluster.
		ServicePrincipalProfile: po.getManagedClusterServicePrincipalProfile(),
		AddonProfiles:           nil,
		// NodeResourceGroup is an output value
		NodeResourceGroup:       nil,
		EnableRBAC:              BoolAsPTR(false),
		EnablePodSecurityPolicy: nil,
		NetworkProfile:          po.getNetworkProfileType(),
		AadProfile:              nil,
		APIServerAccessProfile:  nil,
	}

	return &containerservice.ManagedCluster{
		ManagedClusterProperties: properties,
		Identity:                 nil,
		Location:                 StringAsPTR(po.request.Zone),
		Tags:                     tags,
	}, nil
}

// createAKSCluster creates a new Kubernetes cluster managed by Azure
//
// Equivalent function az aks create --resource-group $2 --location "$3" --name $AKS_NAME
// --service-principal $4 --client-secret $5 --node-count $6 --kubernetes-version $7
// --enable-addons monitoring --node-vm-size Standard_DS2_v2 --disable-rbac
func (po ProvisionerOperation) createAKSCluster() (*containerservice.ManagedCluster, derrors.Error) {
	po.AddToLog("Creating new cluster")
	clusterClient := containerservice.NewManagedClustersClient(po.credentials.SubscriptionId)
	clusterClient.Authorizer = po.managementAuthorizer
	parameters, err := po.getKubernetesCreateRequest()
	if err != nil {
		return nil, err
	}
	ctx, cancel := common.GetContext()
	defer cancel()

	var resourceName string
	if po.request.IsManagementCluster {
		resourceName = po.getResourceName(po.request.ClusterID, po.request.ClusterName)
	} else {
		resourceName = po.getClusterName(po.request.ClusterName)
	}
	log.Debug().Str("resourceGroupName", po.request.AzureOptions.ResourceGroup).Str("resourceName", resourceName).Msg("CreateOrUpdate params")
	responseFuture, createErr := clusterClient.CreateOrUpdate(ctx, po.request.AzureOptions.ResourceGroup, resourceName, *parameters)
	if createErr != nil {
		return nil, derrors.NewInternalError("cannot create AKS cluster", createErr).WithParams(po.request)
	}
	po.AddToLog("waiting for AKS to be created")
	futureContext, cancelFuture := context.WithTimeout(context.Background(), ClusterCreateDeadline)
	defer cancelFuture()
	waitErr := responseFuture.WaitForCompletionRef(futureContext, clusterClient.Client)
	if waitErr != nil {
		return nil, derrors.AsError(waitErr, "AKS cluster creation failed during creation")
	}
	managedCluster, resultErr := responseFuture.Result(clusterClient)
	if resultErr != nil {
		log.Error().Interface("err", resultErr).Msg("AKS creation failed")
		return nil, derrors.AsError(resultErr, "AKS creation failed")
	}
	log.Debug().Str("nodeResourceGroup", *managedCluster.NodeResourceGroup).Msg("AKS has been created")
	kubeConfig, err := po.retrieveKubeConfig(po.request.AzureOptions.ResourceGroup, resourceName)
	if err != nil {
		return nil, err
	}
	po.result.RawKubeConfig = *kubeConfig
	return &managedCluster, nil
}

// CreateServicePrincipal creates a service principal for rbac. The code is based on the azure
// CLI code that is available at: https://github.com/Azure/azure-cli/blob/master/src/azure-cli/azure/cli/command_modules/role/custom.py
func (po ProvisionerOperation) createServicePrincipalForRBAC(requestID string) (*graphrbac.Application, derrors.Error) {
	log.Debug().Msg("creating service principal for RBAC")
	appClient := graphrbac.NewApplicationsClient(po.credentials.TenantId)
	appClient.Authorizer = po.graphAuthorizer
	spClient := graphrbac.NewServicePrincipalsClient(po.credentials.TenantId)
	spClient.Authorizer = po.graphAuthorizer
	log.Debug().Msg("creating base application")
	app, err := po.createApplication(appClient, po.request.ClusterID)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("app", app).Msg("application entity has been created, creating SP")

	// Once the main application entity is created, we need to create the associated service principal
	associatedSP, err := po.createServicePrincipal(spClient, *app.AppID, po.request.ClusterID)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("sp", associatedSP).Msg("service principal has been created")
	return app, nil
}

// createAssociatedIPAddresses creates a set of publicly exposed IP addresses for the cluster.
func (po ProvisionerOperation) createAssociatedIPAddresses(nodeResourceGroup string) derrors.Error {
	po.AddToLog("Reserving IP addresses")
	var IPAddressPool []string
	if po.request.IsManagementCluster {
		IPAddressPool = ManagementIPAddressNames
	} else {
		IPAddressPool = ApplicationIPAddressNames
	}

	responseCh := make(chan ParallelIPCreateResponse, len(IPAddressPool))
	var wg sync.WaitGroup
	wg.Add(len(IPAddressPool))

	for _, addressName := range IPAddressPool {
		go po.createIPInParallel(responseCh, &wg, nodeResourceGroup, addressName, po.request.Zone)
	}

	wg.Wait()
	close(responseCh)
	for result := range responseCh {
		if result.error != nil {
			return result.error
		}
		po.setResultIP(result.AddressName, result.IPAddress)
	}
	return nil
}

// ParallelIPCreateResponse with the result send to the channel upon static IP reservation.
type ParallelIPCreateResponse struct {
	AddressName string
	IPAddress   *network.PublicIPAddress
	error       derrors.Error
}

// createIPInParallel manages the creation of the required IP addresses in parallel.
func (po ProvisionerOperation) createIPInParallel(response chan<- ParallelIPCreateResponse, wg *sync.WaitGroup, resourceGroupName string, addressName string, region string) {
	defer wg.Done()
	ip, err := po.createIPAddress(resourceGroupName, addressName, region)
	result := ParallelIPCreateResponse{
		AddressName: addressName,
		IPAddress:   ip,
		error:       err,
	}
	response <- result
}

// createDNSEntries triggers the creation of the different DNS entries required for a management cluster
func (po ProvisionerOperation) createManagementDNSEntries(resourceGroupName string) derrors.Error {
	dnsClusterRoot := po.getClusterName(po.request.ClusterName)

	toAdd := make(map[string]string, 0)
	// Ingress entries name.dnsZone and *.name.dnsZone
	toAdd[dnsClusterRoot] = po.result.StaticIPAddresses.Ingress
	toAdd[fmt.Sprintf("*.%s", dnsClusterRoot)] = po.result.StaticIPAddresses.Ingress
	// DNS
	toAdd[fmt.Sprintf("dns.%s", dnsClusterRoot)] = po.result.StaticIPAddresses.DNS
	// VPN Server
	toAdd[fmt.Sprintf("vpn-server.%s", dnsClusterRoot)] = po.result.StaticIPAddresses.VPNServer
	// CoreDNS
	toAdd[fmt.Sprintf("app-dns.%s", dnsClusterRoot)] = po.result.StaticIPAddresses.CoreDNSExt

	for dnsRecordName, IP := range toAdd {
		entry, err := po.createDNSARecord(resourceGroupName, dnsRecordName, po.request.AzureOptions.DNSZoneName, IP)
		if err != nil {
			return err
		}
		po.AddToLog(fmt.Sprintf("DNS entry created %s", *entry.Fqdn))
	}

	// TODO Append dnsZone to the ns
	// Create the NS entry for the endpoint resolution
	entry, err := po.createDNSNSRecord(resourceGroupName,
		fmt.Sprintf("ep.%s.%s", dnsClusterRoot, po.request.AzureOptions.DNSZoneName), fmt.Sprintf("app-dns.%s", dnsClusterRoot),
		po.request.AzureOptions.DNSZoneName)
	if err != nil {
		return err
	}
	po.AddToLog(fmt.Sprintf("DNS entry created %s", *entry.Fqdn))

	return nil
}

func (po ProvisionerOperation) createApplicationDNSEntries(resourceGroupName string) derrors.Error {
	dnsClusterRoot := po.getClusterName(po.request.ClusterName)

	toAdd := make(map[string]string, 0)
	// Ingress entries name.dnsZone and *.name.dnsZone
	toAdd[dnsClusterRoot] = po.result.StaticIPAddresses.Ingress
	toAdd[fmt.Sprintf("*.%s", dnsClusterRoot)] = po.result.StaticIPAddresses.Ingress

	for dnsRecordName, IP := range toAdd {
		entry, err := po.createDNSARecord(resourceGroupName, dnsRecordName, po.request.AzureOptions.DNSZoneName, IP)
		if err != nil {
			return err
		}
		po.AddToLog(fmt.Sprintf("DNS entry created %s", *entry.Fqdn))
	}

	return nil
}

// installCertManager triggers the installation of the cert manager component in charge of providing
// certificates.
func (po ProvisionerOperation) installCertManager() derrors.Error {
	po.AddToLog("installing cert manager")
	err := po.certManagerHelper.Connect(po.result.RawKubeConfig)
	if err != nil {
		return err
	}
	return po.certManagerHelper.InstallCertManager()
}

func (po ProvisionerOperation) requestCertificateIssuer(dnsResourceGroupName string) derrors.Error {
	po.AddToLog("requesting certificate")
	return po.certManagerHelper.RequestCertificateIssuerOnAzure(
		po.credentials.ClientId, po.credentials.ClientSecret,
		po.credentials.SubscriptionId, po.credentials.TenantId,
		dnsResourceGroupName,
		po.request.AzureOptions.DNSZoneName, po.request.IsProduction)
}

func (po ProvisionerOperation) requestCertificate() derrors.Error {
	return po.certManagerHelper.CreateCertificate(
		po.getClusterName(po.request.ClusterName), po.request.AzureOptions.DNSZoneName)
}
