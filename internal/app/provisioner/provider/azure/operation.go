package azure

import (
	"context"
	"fmt"
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
	"strings"
	"sync"
	"time"
)

const ContributorRole = "Contributor"
const IPAddressCreateDeadline = 5 * time.Minute

// AzureRetries defines the number of times an operation in Azure must be retried. This value is taken from the
// CLI implementation itself and its origin unknown for now.
const AzureRetries = 36

type AzureOperation struct {
	sync.Mutex
	credentials          *AzureCredentials
	graphAuthorizer      autorest.Authorizer
	managementAuthorizer autorest.Authorizer
	started              time.Time
	log                  []string
	taskProgress         entities.TaskProgress
	errorMsg             string
	elapsedTime          string
}

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

func (ao *AzureOperation) Log() []string {
	ao.Lock()
	defer ao.Unlock()
	return ao.log
}

func (ao *AzureOperation) AddToLog(entry string) {
	ao.Lock()
	defer ao.Unlock()
	ao.log = append(ao.log, entry)
}

func (ao *AzureOperation) Progress() entities.TaskProgress {
	return ao.taskProgress
}

func (ao *AzureOperation) SetProgress(progress entities.TaskProgress) {
	ao.taskProgress = progress
}

func (ao *AzureOperation) setError(errMsg string) {
	log.Debug().Str("previous", entities.TaskProgressToString[ao.taskProgress]).Str("error", errMsg).Msg("setting error")
	ao.elapsedTime = time.Now().Sub(ao.started).String()
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

func (ao *AzureOperation) getClusterName(clusterName string) string {
	return strings.ToLower(strings.ReplaceAll(clusterName, " ", ""))
}

func (ao *AzureOperation) getResourceName(clusterID string, clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterID, ao.getClusterName(clusterName))
}

// createIPAddress reserves an IP address.
//
// az network public-ip create --name $1 --resource-group $2 --allocation-method Static --location "$3"
func (ao *AzureOperation) createIPAddress(resourceGroupName string, addressName string, region string) (*network.PublicIPAddress, derrors.Error) {
	networkClient := network.NewPublicIPAddressesClient(ao.credentials.SubscriptionId)
	networkClient.Authorizer = ao.managementAuthorizer
	tags := make(map[string]*string, 0)
	tags["created-by"] = StringAsPTR("Nalej")

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
