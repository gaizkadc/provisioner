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
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-08-01/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/date"
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/pkg/common"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
)

// OrganizationIDTag with the name of the tag associated with the Organization Id that created the cluster.
const OrganizationIDTag = "organizationID"

// ClusterIDTag with the name of the tag associated with the Cluster Id assigned to the cluster.
const ClusterIDTag = "clusterID"

// ClusterNameTag with the name of the tag associated with the initial name of the cluster. This
// value even if it is not updated, is maintained for reference.
const ClusterNameTag = "clusterName"

// CreateByTag with the name of the tag used to indicate the creator of the cluster
const CreateByTag = "created-by"

// CreateByValue with the value of the CreateByTag to mark the cluster as Nalej managed
const CreateByValue = "Nalej"

const ContributorRole = "Contributor"
const IPAddressCreateDeadline = 5 * time.Minute

// AzureRetries defines the number of times an operation in Azure must be retried. This value is taken from the
// CLI implementation itself and its origin unknown for now.
const AzureRetries = 36

// AzureOperation structure with common functions shared among the different operations.
type AzureOperation struct {
	sync.Mutex
	credentials          *AzureCredentials
	graphAuthorizer      autorest.Authorizer
	managementAuthorizer autorest.Authorizer
	started              time.Time
	log                  []string
	taskProgress         entities.TaskProgress
	errorMsg             string
	elapsedTime          int64
}

// NewAzureOperation creates an AzureOperation with a set of credentials.
func NewAzureOperation(credentials *AzureCredentials) (*AzureOperation, derrors.Error) {
	graph, err := GetGraphAuthorizer(credentials)
	if err != nil {
		return nil, err
	}
	mngt, err := GetManagementAuthorizer(credentials)
	if err != nil {
		return nil, err
	}
	return &AzureOperation{
		credentials:          credentials,
		graphAuthorizer:      graph,
		managementAuthorizer: mngt,
		log:                  make([]string, 0),
		taskProgress:         entities.Init,
	}, nil
}

// Log returns the operation log.
func (ao *AzureOperation) Log() []string {
	ao.Lock()
	defer ao.Unlock()
	return ao.log
}

// AddToLog adds a new entry to the operation log.
func (ao *AzureOperation) AddToLog(entry string) {
	ao.Lock()
	defer ao.Unlock()
	ao.log = append(ao.log, entry)
}

// Progress returns the progress of an operation.
func (ao *AzureOperation) Progress() entities.TaskProgress {
	return ao.taskProgress
}

// SetProgress sets the progress of the ongoing operation.
func (ao *AzureOperation) SetProgress(progress entities.TaskProgress) {
	ao.taskProgress = progress
}

// setError updates all the fields to indicate that an error ocurred.
func (ao *AzureOperation) setError(errMsg string) {
	log.Debug().Str("previous", entities.TaskProgressToString[ao.taskProgress]).Str("error", errMsg).Msg("setting error")
	ao.elapsedTime = time.Now().Sub(ao.started).Nanoseconds()
	ao.taskProgress = entities.Error
	ao.errorMsg = errMsg
}

func (ao *AzureOperation) getTags(clusterID string) []string {
	//return []string{"created-by-nalej", clusterID}
	return []string{"created-by-nalej"}
}

// getPassword generates a PasswordCredential for an Application entity with a one year validity.
func (ao *AzureOperation) getPasswordCredentialsForNewApp() *[]graphrbac.PasswordCredential {
	startDate := date.Time{Time: time.Now().UTC()}
	endDate := date.Time{Time: time.Now().Add(time.Hour * 24 * 365).UTC()}
	// TODO: Improve method for random password generation.
	keyID := uuid.NewV4().String()
	randomPassword := uuid.NewV4().String()
	credential := graphrbac.PasswordCredential{
		AdditionalProperties: nil,
		StartDate:            &startDate,
		EndDate:              &endDate,
		KeyID:                &keyID,
		Value:                &randomPassword,
		CustomKeyIdentifier:  nil,
	}
	result := []graphrbac.PasswordCredential{credential}
	return &result
}

// createApplication creates an Application entity on the Graph RBAC.
func (ao *AzureOperation) createApplication(client graphrbac.ApplicationsClient, clusterID string) (*graphrbac.Application, derrors.Error) {
	timeMark := time.Now().Format("20060102-150405")
	displayName := fmt.Sprintf("nalej-%s-%s", clusterID, timeMark)
	name := fmt.Sprintf("http://%s", displayName)
	identifierUris := []string{name}
	homepage := fmt.Sprintf("https://nalej-%s", clusterID)
	availableToOthers := false
	//nalejWeb := "https://www.nalej.com"
	// In fact we need to create an application in azure terms.
	createAppRequest := graphrbac.ApplicationCreateParameters{
		DisplayName:             &displayName,
		IdentifierUris:          &identifierUris,
		AvailableToOtherTenants: &availableToOthers,
		Homepage:                &homepage,
		PasswordCredentials:     ao.getPasswordCredentialsForNewApp(),
		//WwwHomepage:                &nalejWeb,
	}

	ctx, cancel := common.GetContext()
	defer cancel()
	log.Debug().Interface("request", createAppRequest).Msg("creating application")
	app, err := client.Create(ctx, createAppRequest)
	if err != nil {
		return nil, derrors.AsError(err, "cannot create application entity in Azure")
	}
	log.Debug().Interface("app", app).Msg("application entity has been creating")
	return &app, nil
}

// createServicePrincipal creates a ServicePrincipal entity associated to an Application.
func (ao *AzureOperation) createServicePrincipal(client graphrbac.ServicePrincipalsClient, appID string, clusterID string) (*graphrbac.ServicePrincipal, derrors.Error) {
	appSpCreated := false
	tags := ao.getTags(clusterID)
	accountEnabled := true
	createSPRequest := graphrbac.ServicePrincipalCreateParameters{
		// AppID seems to be also the client ID, thanks Azure for the confusion.
		// AppID: &po.credentials.ClientId,
		AppID:          &appID,
		AccountEnabled: &accountEnabled,
		Tags:           &tags,
	}
	var associatedSP graphrbac.ServicePrincipal
	for retry := 0; retry < AzureRetries && !appSpCreated; retry++ {
		log.Debug().Int("retry", retry).Msg("attempting to create sp")
		ctxSP, cancelSP := common.GetContext()
		defer cancelSP()
		log.Debug().Msg("creating SP")
		sp, err := client.Create(ctxSP, createSPRequest)
		if err == nil {
			appSpCreated = true
			associatedSP = sp
		} else if strings.Contains(err.Error(), "does not reference") || strings.Contains(err.Error(), "does not exists") {
			log.Debug().Msg("creation of service principal failed, retrying in 5 seconds")
			time.Sleep(time.Second * 5)
		} else {
			return nil, derrors.AsError(err, "creation of associated service principal failed")
		}
	}
	if !appSpCreated {
		return nil, derrors.NewInternalError("unable to create service principal after retries")
	}
	return &associatedSP, nil
}

// getRoleID obtains the role associated with a given name on a Tenant
func (ao *AzureOperation) getRoleID(roleName string, scope string) (*string, derrors.Error) {
	roleDefClient := authorization.NewRoleDefinitionsClient(ao.credentials.TenantId)
	roleDefClient.Authorizer = ao.managementAuthorizer
	ctx, cancel := common.GetContext()
	defer cancel()
	log.Debug().Str("roleName", roleName).Str("scope", scope).Msg("obtaining role ID")
	roles, err := roleDefClient.List(ctx, scope, "")
	if err != nil {
		return nil, derrors.AsError(err, "cannot retrieve list of roles")
	}
	for _, role := range roles.Values() {
		if *role.RoleName == roleName {
			log.Debug().Interface("role", role).Msg("target role found")
			return role.ID, nil
		}
	}
	return nil, derrors.NewNotFoundError("role not found in scope")
}

// authorizeDNSToSP authorizes the management of a DNS zone to a service principal
func (ao *AzureOperation) authorizeDNSToSP(appID string, dnsZone string) derrors.Error {
	zoneClient := dns.NewZonesClient(ao.credentials.SubscriptionId)
	zoneClient.Authorizer = ao.managementAuthorizer
	log.Debug().Str("appID", appID).Str("zone", dnsZone).Msg("authorizing SP for DNS zone management")
	ctx, cancel := common.GetContext()
	defer cancel()
	zones, err := zoneClient.List(ctx, nil)
	if err != nil {
		return derrors.AsError(err, "cannot retrieve list of zones")
	}
	targetZoneId := ""
	for _, zone := range zones.Values() {
		if *zone.Name == dnsZone {
			log.Debug().Str("id", *zone.ID).Str("name", *zone.Name).Msg("target zone")
			targetZoneId = *zone.ID
		}
	}
	if targetZoneId == "" {
		return derrors.NewNotFoundError("unable to find target DNS zone on Azure")
	}

	scope := targetZoneId
	roleID, roleErr := ao.getRoleID(ContributorRole, scope)
	if roleErr != nil {
		return roleErr
	}
	log.Debug().Str("roleID", *roleID).Str("roleName", ContributorRole).Msg("role ID resolved")

	roleClient := authorization.NewRoleAssignmentsClient(ao.credentials.TenantId)
	roleClient.Authorizer = ao.managementAuthorizer
	roleProperties := &authorization.RoleAssignmentProperties{
		RoleDefinitionID: roleID,
		PrincipalID:      &appID,
	}
	roleAssignationRequest := authorization.RoleAssignmentCreateParameters{
		Properties: roleProperties,
	}
	roleCtx, roleCancel := common.GetContext()
	defer roleCancel()
	assignmentName := uuid.NewV4().String()
	result, err := roleClient.Create(roleCtx, scope, assignmentName, roleAssignationRequest)
	if err != nil {
		return derrors.AsError(err, "cannot assign role")
	}
	log.Debug().Str("ID", *result.ID).Msg("Role has been assigned")
	return nil
}

// getDNSResourceGroupName obtains the resourceGroupName from the Zone id.
func (ao *AzureOperation) getDNSResourceGroupName(zone *dns.Zone) (*string, derrors.Error) {
	id := *zone.ID
	first := strings.Index(id, "resourceGroups/")
	second := strings.Index(id, "/providers")
	if first == -1 || second == -1 {
		return nil, derrors.NewInvalidArgumentError("invalid DNS zone ID")
	}
	resourceGroupName := id[first+len("resourceGroups/") : second]
	return &resourceGroupName, nil
}

func (ao *AzureOperation) getDNSZone(zoneName string) (*dns.Zone, derrors.Error) {
	zoneClient := dns.NewZonesClient(ao.credentials.SubscriptionId)
	zoneClient.Authorizer = ao.managementAuthorizer
	ctx, cancel := common.GetContext()
	defer cancel()
	zones, err := zoneClient.List(ctx, nil)
	if err != nil {
		return nil, derrors.AsError(err, "cannot retrieve list of zones")
	}
	var targetZone dns.Zone
	found := false
	for _, zone := range zones.Values() {
		if *zone.Name == zoneName {
			log.Debug().Str("id", *zone.ID).Str("name", *zone.Name).Msg("target zone")
			targetZone = zone
			found = true
		}
	}
	if !found {
		return nil, derrors.NewNotFoundError("unable to find target DNS zone on Azure")
	}
	return &targetZone, nil
}

// getAzureVMSize analyzes the list of supported node types in Azure and returns the proper enum value
// or an error if the type of node requested is not found.
func (ao *AzureOperation) getAzureVMSize(nodeType string) (*containerservice.VMSizeTypes, derrors.Error) {
	all := containerservice.PossibleVMSizeTypesValues()
	for _, node := range all {
		if string(node) == nodeType {
			return &node, nil
		}
	}
	log.Warn().Str("nodeType", nodeType).Msg("user requested an unsupported node type")
	return nil, derrors.NewNotFoundError("invalid node type for Azure")
}

// getDNSPrefix generates a DNS prefix for the new cluster.
func (ao *AzureOperation) getDNSPrefix(clusterID string) string {
	return fmt.Sprintf("nalej-%s", clusterID)
}

// getVMSubnetID generates the VM subnet identifier
func (ao *AzureOperation) getVMSubnetID(clusterID string) string {
	return fmt.Sprintf("nalej-%s", clusterID)
}

// getClusterName returns a valid cluster name to create resources in Azure.
func (ao *AzureOperation) getClusterName(clusterName string) string {
	noSpaces := strings.ReplaceAll(clusterName, " ", "")
	noDots := strings.ReplaceAll(noSpaces, ".", "-")
	return strings.ToLower(noDots)
}

// GetResourceName returns the name of the azure resource based on the clusterID.
func (ao *AzureOperation) getResourceName(isManagement bool, clusterID string) string {
	if isManagement {
		// When installing a management cluster, the clusterID matches the clusterName
		return fmt.Sprintf("mngt-%s", ao.getClusterName(clusterID))
	}
	return fmt.Sprintf("appcluster-%s", clusterID)
}

// createIPAddress reserves an IP address.
//
// az network public-ip create --name $1 --resource-group $2 --allocation-method Static --location "$3"
func (ao *AzureOperation) createIPAddress(resourceGroupName string, addressName string, region string) (*network.PublicIPAddress, derrors.Error) {
	networkClient := network.NewPublicIPAddressesClient(ao.credentials.SubscriptionId)
	networkClient.Authorizer = ao.managementAuthorizer
	tags := make(map[string]*string, 0)
	tags[CreateByTag] = StringAsPTR(CreateByValue)

	properties := &network.PublicIPAddressPropertiesFormat{
		PublicIPAllocationMethod: network.Static,
		IPConfiguration:          nil,
		DNSSettings:              nil,
		IPAddress:                nil,
		// IdleTimeoutInMinutes set to the default value of the cli
		IdleTimeoutInMinutes: Int32AsPTR(4),
	}

	createRequest := network.PublicIPAddress{
		PublicIPAddressPropertiesFormat: properties,
		Location:                        StringAsPTR(region),
		Tags:                            tags,
	}
	ctx, cancel := common.GetContext()
	defer cancel()
	responseFuture, createErr := networkClient.CreateOrUpdate(ctx, resourceGroupName, addressName, createRequest)
	if createErr != nil {
		return nil, derrors.AsError(createErr, "cannot create IP address")
	}
	futureContext, cancelFuture := context.WithTimeout(context.Background(), IPAddressCreateDeadline)
	defer cancelFuture()
	waitErr := responseFuture.WaitForCompletionRef(futureContext, networkClient.Client)
	if waitErr != nil {
		return nil, derrors.AsError(waitErr, "IP address failed during creation")
	}
	IPAddress, resultErr := responseFuture.Result(networkClient)
	if resultErr != nil {
		log.Error().Interface("err", resultErr).Msg("IP address creation failed")
		return nil, derrors.AsError(resultErr, "IP address creation failed")
	}
	log.Debug().Interface("ip", IPAddress).Msg("ip address created")
	return &IPAddress, nil
}

// retrieveKubeConfig retrieves the KubeConfig file for a given cluster
//
//  az aks get-credentials --resource-group dev --name mngt-dhiguero001
func (ao *AzureOperation) retrieveKubeConfig(resourceGroupName string, resourceName string) (*string, derrors.Error) {
	ao.AddToLog("retrieving kubeConfig")
	clusterClient := containerservice.NewManagedClustersClient(ao.credentials.SubscriptionId)
	clusterClient.Authorizer = ao.managementAuthorizer
	ctx, cancel := common.GetContext()
	defer cancel()

	credentials, err := clusterClient.ListClusterUserCredentials(ctx, resourceGroupName, resourceName)
	if err != nil {
		return nil, derrors.AsError(err, "cannot obtain cluster credentials")
	}

	if credentials.Kubeconfigs == nil {
		return nil, derrors.NewInternalError("empty kubeconfig returned")
	}
	kubeConfigs := *credentials.Kubeconfigs
	if len(kubeConfigs) > 1 {
		return nil, derrors.NewInternalError("credentials returned more than one KubeConfig file")
	}
	result := kubeConfigs[0]
	asString := string(*result.Value)
	return &asString, nil
}

// createDNSARecord creates a DNS A record for a given domain and IP.
//az network dns record-set a add-record --resource-group $4 --zone-name $2 --record-set-name "$1" --ipv4-address $3 -o none
func (ao *AzureOperation) createDNSARecord(resourceGroupName string, recordName string, dnsZone string, IPAddress string) (*dns.RecordSet, derrors.Error) {
	dnsClient := dns.NewRecordSetsClient(ao.credentials.SubscriptionId)
	dnsClient.Authorizer = ao.managementAuthorizer
	aRecord := dns.ARecord{Ipv4Address: &IPAddress}
	records := []dns.ARecord{aRecord}
	recordType := dns.A
	recordSetProperties := &dns.RecordSetProperties{
		TTL:            Int64AsPTR(3600),
		TargetResource: nil,
		ARecords:       &records,
	}
	parameters := dns.RecordSet{
		RecordSetProperties: recordSetProperties,
	}
	ctx, cancel := common.GetContext()
	defer cancel()
	log.Debug().Str("resourceGroupName", resourceGroupName).Str("dnsZone", dnsZone).Str("recordName", recordName).Interface("parameters", parameters).Msg("creating entry")
	entry, err := dnsClient.CreateOrUpdate(ctx, resourceGroupName, dnsZone, recordName, recordType, parameters, "", "")
	if err != nil {
		return nil, derrors.AsError(err, "cannot create DNS entry")
	}
	log.Debug().Interface("A record", entry).Msg("DNS entry has been created")
	return &entry, nil
}

// createDNSARecord creates a DNS NS record for a given domain and IP.
//az network dns record-set ns add-record --resource-group $4 --zone-name $2 --record-set-name "$1" --nsdname "$3.$2" -o none
func (ao *AzureOperation) createDNSNSRecord(resourceGroupName string, recordName string, nsName string, dnsZone string) (*dns.RecordSet, derrors.Error) {
	dnsClient := dns.NewRecordSetsClient(ao.credentials.SubscriptionId)
	dnsClient.Authorizer = ao.managementAuthorizer
	nsRecord := dns.NsRecord{Nsdname: StringAsPTR(nsName)}
	records := []dns.NsRecord{nsRecord}
	recordType := dns.NS
	recordSetProperties := &dns.RecordSetProperties{
		TTL:            Int64AsPTR(3600),
		TargetResource: nil,
		NsRecords:      &records,
	}
	parameters := dns.RecordSet{
		RecordSetProperties: recordSetProperties,
	}
	ctx, cancel := common.GetContext()
	defer cancel()
	log.Debug().Str("resourceGroupName", resourceGroupName).Str("dnsZone", dnsZone).Str("recordName", recordName).Interface("parameters", parameters).Msg("creating entry")
	entry, err := dnsClient.CreateOrUpdate(ctx, resourceGroupName, dnsZone, recordName, recordType, parameters, "", "")
	if err != nil {
		return nil, derrors.AsError(err, "cannot create DNS entry")
	}
	log.Debug().Interface("A record", entry).Msg("DNS entry has been created")
	return &entry, nil
}

// GetClusterDetails retrieves the information of an existing cluster.
func (ao *AzureOperation) getClusterDetails(isManagementCluster bool, resourceGroupName string, clusterID string) (*containerservice.ManagedCluster, derrors.Error) {
	clusterClient := containerservice.NewManagedClustersClient(ao.credentials.SubscriptionId)
	clusterClient.Authorizer = ao.managementAuthorizer
	resourceName := ao.getResourceName(isManagementCluster, clusterID)

	ctx, cancel := common.GetContext()
	defer cancel()
	managedCluster, err := clusterClient.Get(ctx, resourceGroupName, resourceName)
	if err != nil {
		return nil, derrors.NewNotFoundError("cannot retrieve managed cluster", err)
	}
	return &managedCluster, nil
}

// GetKubernetesUpdateRequest modifies the existing cluster object changing the number of nodes.
func (ao *AzureOperation) getKubernetesUpdateRequest(existingCluster *containerservice.ManagedCluster, numNodes int64) (*containerservice.ManagedCluster, derrors.Error) {
	createdBy, exists := existingCluster.Tags[CreateByTag]
	if !exists || *createdBy != CreateByValue {
		return nil, derrors.NewInvalidArgumentError("cannot manage non Nalej created clusters")
	}
	if existingCluster.AgentPoolProfiles == nil || len(*existingCluster.AgentPoolProfiles) != 1 {
		return nil, derrors.NewInternalError("expecting a single agent pool profile")
	}
	numNodesPtr, err := Int64ToInt32(numNodes)
	if err != nil {
		return nil, err
	}
	(*existingCluster.AgentPoolProfiles)[0].Count = numNodesPtr
	return existingCluster, nil
}

// getKubernetesCreateRequest creates the ManagedCluster object required to create or update a new AKS cluster.
func (ao *AzureOperation) getKubernetesCreateRequest(
	organizationID string, clusterID string, clusterName string, kubernetesVersion string,
	numNodes int64, nodeType string, zone string,
) (*containerservice.ManagedCluster, derrors.Error) {

	tags := make(map[string]*string, 0)
	tags[OrganizationIDTag] = StringAsPTR(organizationID)
	tags[ClusterIDTag] = StringAsPTR(clusterID)
	tags[ClusterNameTag] = StringAsPTR(ao.getClusterName(clusterName))
	tags[CreateByTag] = StringAsPTR(CreateByValue)

	numNodesPtr, err := Int64ToInt32(numNodes)
	if err != nil {
		return nil, err
	}

	vmSize, err := ao.getAzureVMSize(nodeType)
	if err != nil {
		return nil, err
	}
	dnsPrefix := ao.getDNSPrefix(clusterID)
	vmSubnetID := ao.getVMSubnetID(clusterID)

	agentProfile := ao.getManagedClusterAgentProfile(numNodesPtr, vmSize, &vmSubnetID, kubernetesVersion)
	agentProfiles := []containerservice.ManagedClusterAgentPoolProfile{agentProfile}

	properties := &containerservice.ManagedClusterProperties{
		KubernetesVersion: StringAsPTR(kubernetesVersion),
		DNSPrefix:         &dnsPrefix,
		AgentPoolProfiles: &agentProfiles,
		// LinuxProfile not set as SSH access is not required
		LinuxProfile: nil,
		// WindowsProfile not set.
		WindowsProfile: nil,
		// ServicePrincipalProfile associated with the cluster.
		ServicePrincipalProfile: ao.getManagedClusterServicePrincipalProfile(),
		AddonProfiles:           nil,
		// NodeResourceGroup is an output value
		NodeResourceGroup:       nil,
		EnableRBAC:              BoolAsPTR(false),
		EnablePodSecurityPolicy: nil,
		NetworkProfile:          ao.getNetworkProfileType(),
		AadProfile:              nil,
		APIServerAccessProfile:  nil,
	}

	return &containerservice.ManagedCluster{
		ManagedClusterProperties: properties,
		Identity:                 nil,
		Location:                 StringAsPTR(zone),
		Tags:                     tags,
	}, nil
}

func (ao *AzureOperation) getManagedClusterAgentProfile(numNodes *int32, vmSize *containerservice.VMSizeTypes, vmSubnetID *string, kubernetesVersion string) containerservice.ManagedClusterAgentPoolProfile {
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
		OrchestratorVersion: StringAsPTR(kubernetesVersion),
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

// getNetworkProfileType returns the network profile of a new provisioned cluster.
func (ao *AzureOperation) getNetworkProfileType() *containerservice.NetworkProfileType {
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

// getManagedClusterServicePrincipalProfile returns the service principal required to provision a new cluster.
func (ao *AzureOperation) getManagedClusterServicePrincipalProfile() *containerservice.ManagedClusterServicePrincipalProfile {
	return &containerservice.ManagedClusterServicePrincipalProfile{
		ClientID: StringAsPTR(ao.credentials.ClientId),
		Secret:   StringAsPTR(ao.credentials.ClientSecret),
	}
}
