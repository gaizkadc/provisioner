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
 *
 */

package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-08-01/containerservice"
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/pkg/common"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	"time"
)

type ScalerOperation struct {
	*AzureOperation
	request entities.ScaleRequest
	config  *config.Config
}

func NewScalerOperation(credentials *AzureCredentials, request entities.ScaleRequest, config *config.Config) (*ScalerOperation, derrors.Error) {
	azureOp, err := NewAzureOperation(credentials)
	if err != nil {
		return nil, err
	}
	return &ScalerOperation{
		AzureOperation: azureOp,
		request:        request,
		config:         config,
	}, nil
}

func (so *ScalerOperation) RequestID() string {
	return so.request.RequestID
}

func (so *ScalerOperation) Metadata() entities.OperationMetadata {
	return entities.OperationMetadata{
		OrganizationID: so.request.OrganizationID,
		ClusterID:      so.request.ClusterID,
		RequestID:      so.request.RequestID,
	}
}

func (so *ScalerOperation) notifyError(err derrors.Error, callback func(requestId string)) {
	log.Error().Str("trace", err.DebugReport()).Msg("operation failed")
	so.setError(err.Error())
	callback(so.request.RequestID)
}

func (so *ScalerOperation) Execute(callback func(requestID string)) {
	log.Debug().Str("organizationID", so.request.OrganizationID).Str("clusterID", so.request.ClusterID).Int64("numNodes", so.request.NumNodes).Msg("executing scaling operation")
	so.started = time.Now()
	so.SetProgress(entities.InProgress)

	if so.request.NumNodes < 3 {
		so.notifyError(derrors.NewInvalidArgumentError("cannot scale a cluster to less than 3 nodes"), callback)
		return
	}

	scaled, err := so.scaleAKS()
	if err != nil {
		so.notifyError(err, callback)
		return
	}
	log.Debug().Interface("name", scaled.Name).Msg("cluster has been scaled")

	so.elapsedTime = time.Now().Sub(so.started).Nanoseconds()
	so.SetProgress(entities.Finished)
	callback(so.request.RequestID)
}

func (so *ScalerOperation) Cancel() derrors.Error {
	panic("implement me")
}

func (so *ScalerOperation) Result() entities.OperationResult {
	elapsed := so.elapsedTime
	if so.elapsedTime == 0 && so.taskProgress == entities.InProgress {
		// If the operation is in progress, retrieved the ongoing time.
		elapsed = time.Now().Sub(so.started).Nanoseconds()
	}

	return entities.OperationResult{
		RequestId:   so.request.RequestID,
		Type:        entities.Scale,
		Progress:    so.taskProgress,
		ElapsedTime: elapsed,
		ErrorMsg:    so.errorMsg,
	}
}

// ScaleAKS triggers the scaling of an existing management cluster.
func (so *ScalerOperation) scaleAKS() (*containerservice.ManagedCluster, derrors.Error) {
	so.AddToLog("Scaling existing cluster")
	clusterClient := containerservice.NewManagedClustersClient(so.credentials.SubscriptionId)
	clusterClient.Authorizer = so.managementAuthorizer

	existingCluster, err := so.getClusterDetails(so.request.IsManagementCluster, so.request.AzureOptions.ResourceGroup, so.request.ClusterID)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("existingCluster", existingCluster).Msg("AKS cluster retrieved")

	updated, err := so.getKubernetesUpdateRequest(existingCluster, so.request.NumNodes)
	ctx, cancel := common.GetContext()
	defer cancel()

	resourceName := so.getResourceName(so.request.IsManagementCluster, so.request.ClusterID)
	log.Debug().Str("resourceGroupName", so.request.AzureOptions.ResourceGroup).Str("resourceName", resourceName).Msg("CreateOrUpdate params")
	responseFuture, createErr := clusterClient.CreateOrUpdate(ctx, so.request.AzureOptions.ResourceGroup, resourceName, *updated)
	if createErr != nil {
		return nil, derrors.NewInternalError("cannot scale AKS cluster", createErr).WithParams(so.request)
	}

	so.AddToLog("waiting for AKS to be scaled")
	futureContext, cancelFuture := context.WithTimeout(context.Background(), ClusterCreateDeadline)
	defer cancelFuture()
	waitErr := responseFuture.WaitForCompletionRef(futureContext, clusterClient.Client)
	if waitErr != nil {
		return nil, derrors.AsError(waitErr, "AKS cluster scale failed")
	}
	scaledCluster, resultErr := responseFuture.Result(clusterClient)
	if resultErr != nil {
		log.Error().Interface("err", resultErr).Msg("AKS scale failed")
		return nil, derrors.AsError(resultErr, "AKS scale failed")
	}
	return &scaledCluster, nil
}
