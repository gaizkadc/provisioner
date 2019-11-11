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

package entities

import (
	"github.com/nalej/derrors"
	grpc_installer_go "github.com/nalej/grpc-installer-go"
	grpc_provisioner_go "github.com/nalej/grpc-provisioner-go"
	"github.com/rs/zerolog/log"
)

const IngressIPAddressName = "ingressPublicIPAddress"
const DNSPublicIPAddress = "dnsPublicIPAddress"
const CoreDNSPublicIPAddress = "corednsPublicIPAddress"
const VPNServerPublicIPAddress = "vpnserverPublicIPAddress"

// ValidProvisionClusterRequest validates the request to create a new cluster.
func ValidProvisionClusterRequest(request *grpc_provisioner_go.ProvisionClusterRequest) derrors.Error {
	if request.RequestId == "" {
		return derrors.NewInvalidArgumentError("request_id must be set")
	}
	if !request.IsManagementCluster && request.OrganizationId == "" {
		return derrors.NewInvalidArgumentError("organization_id must be set")
	}
	if !request.IsManagementCluster && request.ClusterId == "" {
		return derrors.NewInvalidArgumentError("cluster_id must be set")
	}
	if request.NumNodes <= 0 {
		return derrors.NewInvalidArgumentError("num_nodes must be positive")
	}
	if request.NodeType == "" {
		return derrors.NewInvalidArgumentError("node_type must be set")
	}
	if request.TargetPlatform == grpc_installer_go.Platform_AZURE && request.AzureCredentials == nil {
		return derrors.NewInvalidArgumentError("azure_credentials must be set when type is Azure")
	}
	if request.TargetPlatform == grpc_installer_go.Platform_AZURE && request.AzureOptions == nil {
		return derrors.NewInvalidArgumentError("azure_options must be set when type is Azure")
	}
	return nil
}

type AzureOptions struct {
	// ResourceGroup where the cluster will be provisioned.
	ResourceGroup string
	// DnsZoneName with the name of the target DNS zone onto which the new cluster entries will be added.
	DNSZoneName string
}

// ProvisionRequest with the information required to perform a provisioning operation.
type ProvisionRequest struct {
	// RequestID with the request identifier.
	RequestID string
	// OrganizationId with the organization identifier.
	OrganizationID string
	// ClusterId with the cluster identifier.
	ClusterID string
	// ClusterName with the name of the cluster to be created
	ClusterName string
	// Kubernetes version with the version of Kubernetes to be installed. This version may not be available on all
	// providers.
	KubernetesVersion string
	// NumNodes with the number of nodes of the cluster to be created.
	NumNodes int64
	// NodeType with the type of node to be used. This value must exist in the target infrastructure provider.
	NodeType string
	// Zone where the cluster will be provisioned. This value must exist in the target infrastructure provider.
	Zone string
	// IsManagementCluster to determine if the provisioning is for a management or application cluster.
	IsManagementCluster bool
	// IsProduction determines if the cluster to be provisioned is for production or staging. This flag
	// will affect the provisioned cluster in options such as who is the signer of the certificate.
	IsProduction bool
	// AzureOptions with the provisioning specific options.
	AzureOptions *AzureOptions
}

func NewAzureOptions(request *grpc_provisioner_go.AzureProvisioningOptions) *AzureOptions {
	if request == nil {
		return nil
	}
	return &AzureOptions{
		ResourceGroup: request.ResourceGroup,
		DNSZoneName:   request.DnsZoneName,
	}
}

func NewProvisionRequest(request *grpc_provisioner_go.ProvisionClusterRequest) ProvisionRequest {
	return ProvisionRequest{
		RequestID:           request.RequestId,
		OrganizationID:      request.OrganizationId,
		ClusterID:           request.ClusterId,
		ClusterName:         request.ClusterName,
		KubernetesVersion:   request.KubernetesVersion,
		NumNodes:            request.NumNodes,
		NodeType:            request.NodeType,
		Zone:                request.Zone,
		IsManagementCluster: request.IsManagementCluster,
		IsProduction:        request.IsProduction,
		AzureOptions:        NewAzureOptions(request.AzureOptions),
	}
}

type ScaleRequest struct {
	// NumNodes with the number of nodes of the cluster to be scaled to.
	NumNodes int
	// NodeType with the type of node to be used. This value must exist in the target infrastructure provider.
	NodeType string
	// Zone where the cluster will be provisioned. This value must exist in the target infrastructure provider.
	Zone string
}

type StaticIPAddresses struct {
	Ingress    string
	DNS        string
	ZtPlanet   string
	CoreDNSExt string
	VPNServer  string
}

// ProvisionResult with the result of a successful provisioning.
type ProvisionResult struct {
	// ClusterName with the name of the cluster
	ClusterName string
	// Hostname with the full hostname of the cluster
	Hostname string
	// RawKubeConfig contains the contents of the resulting kubeconfig files
	RawKubeConfig string
	// StaticIPAddresses with the generated addresses.
	StaticIPAddresses StaticIPAddresses
}

// SetIPAddress sets the corresponding IP address by matching the name.
func (pr *ProvisionResult) SetIPAddress(addressName string, IP string) {
	switch addressName {
	case IngressIPAddressName:
		pr.StaticIPAddresses.Ingress = IP
	case DNSPublicIPAddress:
		pr.StaticIPAddresses.DNS = IP
	case CoreDNSPublicIPAddress:
		pr.StaticIPAddresses.CoreDNSExt = IP
	case VPNServerPublicIPAddress:
		pr.StaticIPAddresses.VPNServer = IP
	default:
		log.Error().Str("addressName", addressName).Msg("target address name not supported")
	}
}
