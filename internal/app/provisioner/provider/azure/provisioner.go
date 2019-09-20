package azure

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest"
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/pkg/common"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	"time"
)

type ProvisionerOperation struct {
	AzureOperation
	request     entities.ProvisionRequest
}

func NewProvisionerOperation(credentials *AzureCredentials, authorizer autorest.Authorizer, request entities.ProvisionRequest) *ProvisionerOperation {
	return &ProvisionerOperation{
		AzureOperation: NewAzureOperation(credentials, authorizer),
		request:     request,
	}
}

func (po ProvisionerOperation) RequestID() string {
	return po.request.RequestID
}

func (po ProvisionerOperation) Metadata() entities.OperationMetadata {
	return entities.OperationMetadata{
		OrganizationID: po.request.OrganizationID,
		ClusterID:      po.request.ClusterID,
		RequestID:      po.request.RequestID,
	}
}

func (po ProvisionerOperation) Execute(callback func(requestId string)){
	log.Debug().Interface("credentials", po.credentials).Msg("executing provisioning operation")
	po.started = time.Now()
	//err := po.listServicePrincipal(po.request.RequestID)
	//err := po.listApplications(po.request.RequestID)
	//err := po.getApplication(po.request.RequestID)
	//err = po.listUsers(po.request.RequestID)

	app, err := po.createServicePrincipalForRBAC(po.request.RequestID)
	if err != nil{
		log.Error().Str("trace", err.DebugReport()).Msg("operation failed")
		po.setError(err.Error())
		callback(po.request.RequestID)
		return
	}
	log.Debug().Str("appId", *app.AppID).Msg("Application is ready")
	po.AddToLog(fmt.Sprintf("Application %s has been created", *app.AppID))
	// Give access to the SP for the target dns zone
	err = po.authorizeDNSToSP(*app.AppID, "nalej.tech")
	if err != nil{
		log.Error().Str("trace", err.DebugReport()).Msg("authorize DNS operation failed")
		po.setError(err.Error())
		callback(po.request.RequestID)
		return
	}

	log.Debug().Msg("provisioning finished")
	po.elapsedTime = time.Now().Sub(po.started).String()
	callback(po.request.RequestID)
	return
}

func (po ProvisionerOperation) Cancel() derrors.Error {
	panic("implement me")
}

func (po ProvisionerOperation) Result() entities.OperationResult {
	elapsed := po.elapsedTime
	if po.elapsedTime == "" && po.taskProgress == entities.InProgress{
		// If the operation is in progress, retrieved the ongoing time.
		elapsed = time.Now().Sub(po.started).String()
	}
	// TODO Fix with the final result
	return entities.OperationResult{
		RequestId:       po.request.RequestID,
		Type:            entities.Provision,
		Progress:        po.taskProgress,
		ElapsedTime:     elapsed,
		ErrorMsg:        po.errorMsg,
		ProvisionResult: nil,
	}
}



// CreateServicePrincipal creates a service principal for rbac. The code is based on the azure
// CLI code that is available at: https://github.com/Azure/azure-cli/blob/master/src/azure-cli/azure/cli/command_modules/role/custom.py
func (po ProvisionerOperation) createServicePrincipalForRBAC(requestID string) (*graphrbac.Application, derrors.Error) {
	log.Debug().Msg("creating service principal for RBAC")
	appClient := graphrbac.NewApplicationsClient(po.credentials.TenantId)
	appClient.Authorizer = po.authorizer
	spClient := graphrbac.NewServicePrincipalsClient(po.credentials.TenantId)
	spClient.Authorizer = po.authorizer
	log.Debug().Msg("creating base application")
	app, err := po.createApplication(appClient, po.request.ClusterID)
	if err != nil{
		return nil, err
	}
	log.Debug().Interface("app", app).Msg("application entity has been created, creating SP")

	// Once the main application entity is created, we need to create the associated service principal
	associatedSP, err := po.createServicePrincipal(spClient, *app.AppID, po.request.ClusterID)
	if err != nil{
		return nil, err
	}
	log.Debug().Interface("sp", associatedSP).Msg("service principal has been created")
	return app, nil
}


func (po ProvisionerOperation) listServicePrincipal(requestID string) derrors.Error {
	log.Debug().Msg("listing service principal")
	spClient := graphrbac.NewServicePrincipalsClient(po.credentials.TenantId)
	spClient.Authorizer = po.authorizer
	ctx, cancel := common.GetContext()
	defer cancel()

	sp, err := spClient.List(ctx, "")
	if err != nil {
		log.Error().Str("err", err.Error()).Msg("cannot list service principal")
		return derrors.AsError(err, "cannot list service principal")
	}
	log.Debug().Interface("sp", sp).Msg("service principal has been listed")
	return nil
}

func (po ProvisionerOperation) listUsers(requestID string) derrors.Error {
	log.Debug().Msg("listing users")
	spClient := graphrbac.NewUsersClient(po.credentials.TenantId)
	spClient.Authorizer = po.authorizer
	ctx, cancel := common.GetContext()
	defer cancel()

	sp, err := spClient.List(ctx, "")
	if err != nil {
		log.Error().Str("err", err.Error()).Msg("cannot list users")
		return derrors.AsError(err, "cannot list users")
	}
	log.Debug().Interface("sp", sp).Msg("users has been listed")
	return nil
}

func (po ProvisionerOperation) listApplications(requestID string) derrors.Error {
	log.Debug().Msg("listing applications")
	spClient := graphrbac.NewApplicationsClient(po.credentials.TenantId)
	spClient.Authorizer = po.authorizer
	ctx, cancel := common.GetContext()
	defer cancel()

	sp, err := spClient.List(ctx, "")
	if err != nil {
		log.Error().Str("err", err.Error()).Msg("cannot list applications")
		return derrors.AsError(err, "cannot list applications")
	}
	for _, app := range sp.Values(){
		if *app.AppID == po.credentials.ClientId{
			log.Debug().Str("objectID", *app.ObjectID).Str("appId", *app.AppID).Interface("identifierUris", app.IdentifierUris).Msg("retrieved app")
		}
	}
	return nil
}

func (po ProvisionerOperation) getApplication(requestID string) derrors.Error {
	log.Debug().Msg("get credentials application")
	spClient := graphrbac.NewApplicationsClient(po.credentials.TenantId)
	spClient.Authorizer = po.authorizer
	ctx, cancel := common.GetContext()
	defer cancel()

	sp, err := spClient.Get(ctx, po.credentials.ClientId)
	if err != nil {
		log.Error().Str("err", err.Error()).Msg("cannot get applications")
		return derrors.AsError(err, "cannot get applications")
	}
	log.Debug().Interface("result", sp).Msg("Application retrieved")
	return nil
}
