package azure

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/authorization/mgmt/2015-07-01/authorization"
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

// AzureRetries defines the number of times an operation in Azure must be retried. This value is taken from the
// CLI implementation itself and its origin unknown for now.
const AzureRetries = 36

type AzureOperation struct{
	sync.Mutex
	credentials *AzureCredentials
	authorizer  autorest.Authorizer
	started time.Time
	log []string
	taskProgress entities.TaskProgress
	errorMsg string
	elapsedTime string
}

func NewAzureOperation(credentials *AzureCredentials, authorizer  autorest.Authorizer) AzureOperation{
	return AzureOperation{
		credentials:  credentials,
		authorizer:   authorizer,
		log:          make([]string, 0),
		taskProgress: entities.Init,
	}
}

func (ao AzureOperation) Log() []string {
	ao.Lock()
	defer ao.Unlock()
	return ao.log
}

func (ao AzureOperation) AddToLog(entry string) {
	ao.Lock()
	defer ao.Unlock()
	ao.log = append(ao.log, entry)
}

func (ao AzureOperation) Progress() entities.TaskProgress {
	return ao.taskProgress
}

func (ao AzureOperation) SetProgress(progress entities.TaskProgress) {
	ao.taskProgress = progress
}

func (ao AzureOperation) setError(errMsg string){
	ao.elapsedTime = time.Now().Sub(ao.started).String()
	ao.taskProgress = entities.Error
	ao.errorMsg = errMsg
}

func (ao AzureOperation) getTags(clusterID string) []string{
	//return []string{"created-by-nalej", clusterID}
	return []string{"created-by-nalej"}
}

// getPassword generates a PasswordCredential for an Application entity with a one year validity.
func (ao AzureOperation) getPasswordCredentialsForNewApp() *[]graphrbac.PasswordCredential{
	startDate := date.Time{Time:time.Now().UTC()}
	endDate := date.Time{Time:time.Now().Add(time.Hour * 24 * 365).UTC()}
	// TODO: Improve method for random password generation.
	keyID:= uuid.NewV4().String()
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
func (ao AzureOperation) createApplication(client graphrbac.ApplicationsClient, clusterID string) (*graphrbac.Application, derrors.Error){
	timeMark := time.Now().Format("20060102-150405")
	displayName := fmt.Sprintf("nalej-%s-%s", clusterID, timeMark)
	name := fmt.Sprintf("http://%s", displayName)
	identifierUris := []string{name}
	homepage := fmt.Sprintf("https://nalej-%s", clusterID)
	availableToOthers := false
	//nalejWeb := "https://www.nalej.com"
	// In fact we need to create an application in azure terms.
	createAppRequest := graphrbac.ApplicationCreateParameters{
		DisplayName:                &displayName,
		IdentifierUris:             &identifierUris,
		AvailableToOtherTenants:    &availableToOthers,
		Homepage:                   &homepage,
		PasswordCredentials:        ao.getPasswordCredentialsForNewApp(),
		//WwwHomepage:                &nalejWeb,
	}

	ctx, cancel := common.GetContext()
	defer cancel()
	log.Debug().Interface("request", createAppRequest).Msg("creating application")
	app, err := client.Create(ctx, createAppRequest)
	if err != nil{
		return nil, derrors.AsError(err, "cannot create application entity in Azure")
	}
	log.Debug().Interface("app", app).Msg("application entity has been creating")
	return &app, nil
}

// createServicePrincipal creates a ServicePrincipal entity associated to an Application.
func (ao AzureOperation) createServicePrincipal(client graphrbac.ServicePrincipalsClient, appID string, clusterID string) (*graphrbac.ServicePrincipal, derrors.Error){
	appSpCreated := false
	tags := ao.getTags(clusterID)
	accountEnabled := true
	createSPRequest := graphrbac.ServicePrincipalCreateParameters{
		// AppID seems to be also the client ID, thanks Azure for the confusion.
		// AppID: &po.credentials.ClientId,
		AppID: &appID,
		AccountEnabled: &accountEnabled,
		Tags:  &tags,
	}
	var associatedSP graphrbac.ServicePrincipal
	for retry := 0; retry < AzureRetries && !appSpCreated; retry++{
		log.Debug().Int("retry", retry).Msg("attempting to create sp")
		ctxSP, cancelSP := common.GetContext()
		defer cancelSP()
		log.Debug().Msg("creating SP")
		sp, err := client.Create(ctxSP, createSPRequest)
		if err == nil{
			appSpCreated = true
			associatedSP = sp
		}else if strings.Contains(err.Error(), "does not reference") || strings.Contains(err.Error(), "does not exists"){
			log.Debug().Msg("creation of service principal failed, retrying in 5 seconds")
			time.Sleep(time.Second * 5)
		}else{
			return nil, derrors.AsError(err, "creation of associated service principal failed")
		}
	}
	if !appSpCreated{
		return nil, derrors.NewInternalError("unable to create service principal after retries")
	}
	return &associatedSP, nil
}

func (ao AzureOperation) getRoleID(roleName string, scope string)(*string, derrors.Error){
	roleDefClient := authorization.NewRoleDefinitionsClient(ao.credentials.TenantId)
	roleDefClient.Authorizer = ao.authorizer
	ctx, cancel := common.GetContext()
	defer cancel()
	log.Debug().Str("roleName", roleName).Str("scope", scope).Msg("obtaining role ID")
	roles, err := roleDefClient.List(ctx, scope, "")
	if err != nil{
		return nil, derrors.AsError(err, "cannot retrieve list of roles")
	}
	for _, role := range roles.Values(){
		if *role.Name == roleName{
			log.Debug().Interface("role", role).Msg("target role found")
			return role.ID, nil
		}
	}
	return nil, derrors.NewNotFoundError("role not found in scope")
}

func (ao AzureOperation) authorizeDNSToSP(appID string, dnsZone string) derrors.Error{
	zoneClient := dns.NewZonesClient(ao.credentials.TenantId)
	zoneClient.Authorizer = ao.authorizer
	log.Debug().Str("appID", appID).Str("zone", dnsZone).Msg("authorizing SP for DNS zone management")
	ctx, cancel := common.GetContext()
	defer cancel()
	zones , err := zoneClient.List(ctx, nil)
	if err != nil {
		return derrors.AsError(err, "cannot retrieve list of zones")
	}
	targetZoneId := ""
	for _, zone := range zones.Values(){
		if *zone.Name == dnsZone{
			log.Debug().Str("id", *zone.ID).Str("name", *zone.Name).Msg("target zone")
			targetZoneId = *zone.ID
		}
	}
	if targetZoneId == ""{
		return derrors.NewNotFoundError("unable to find target DNS zone on Azure")
	}

	scope := targetZoneId
	roleID, roleErr := ao.getRoleID(ContributorRole, scope)
	if err != nil{
		return roleErr
	}

	roleClient := authorization.NewRoleAssignmentsClient(ao.credentials.TenantId)
	roleClient.Authorizer = ao.authorizer
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
	if err != nil{
		return derrors.AsError(err, "cannot assign role")
	}
	log.Debug().Str("ID", *result.ID).Msg("Role has been assigned")
	return nil
}