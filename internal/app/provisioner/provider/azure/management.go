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
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	"time"
)

type ManagementOperation struct {
	*AzureOperation
	targetOp         entities.ManagementOperationType
	request          entities.ClusterRequest
	config           *config.Config
	KubeConfigResult *string
}

func NewManagementOperation(credentials *AzureCredentials, request entities.ClusterRequest, operation entities.ManagementOperationType, config *config.Config) (*ManagementOperation, derrors.Error) {
	azureOp, err := NewAzureOperation(credentials)
	if err != nil {
		return nil, err
	}
	return &ManagementOperation{
		AzureOperation: azureOp,
		targetOp:       operation,
		request:        request,
		config:         config,
	}, nil
}

// RequestID returns the request identifier associated with this operation
func (mo *ManagementOperation) RequestID() string {
	return mo.request.RequestID
}

// Metadata returns the operation associated metadata
func (mo *ManagementOperation) Metadata() entities.OperationMetadata {
	return entities.OperationMetadata{
		OrganizationID: mo.request.OrganizationID,
		ClusterID:      mo.request.ClusterID,
		RequestID:      mo.request.RequestID,
	}
}

func (mo *ManagementOperation) notifyError(err derrors.Error, callback func(requestId string)) {
	log.Error().Str("trace", err.DebugReport()).Msg("operation failed")
	mo.setError(err.Error())
	callback(mo.request.RequestID)
}

// Execute triggers the execution of the operation. The callback function on the execute is expected to be
// called when the operation finish its execution independently of the status.
func (mo *ManagementOperation) Execute(callback func(requestId string)) {
	log.Debug().Str("organizationID", mo.request.OrganizationID).Str("clusterID", mo.request.ClusterID).Msg("executing management operation")
	mo.started = time.Now()
	mo.SetProgress(entities.InProgress)

	if mo.targetOp != entities.GetKubeConfig {
		err := derrors.NewUnimplementedError("target operation is not supported").WithParams(mo.targetOp)
		mo.notifyError(err, callback)
		return
	}

	resourceName := mo.getResourceName(mo.request.IsManagementCluster, mo.request.ClusterID)
	log.Debug().Str("resourceGroupName", mo.request.AzureOptions.ResourceGroup).Str("resourceName", resourceName).Msg("GetKubeConfig params")
	result, err := mo.retrieveKubeConfig(mo.request.AzureOptions.ResourceGroup, resourceName)
	if err != nil {
		mo.notifyError(err, callback)
		return
	}
	mo.KubeConfigResult = result
	mo.elapsedTime = time.Now().Sub(mo.started).Nanoseconds()
	mo.SetProgress(entities.Finished)
	callback(mo.request.RequestID)
}

// Cancel triggers the cancellation of the operation
func (mo *ManagementOperation) Cancel() derrors.Error {
	panic("implement me")
}

// Result returns the operation result if this operation is successful
func (mo *ManagementOperation) Result() entities.OperationResult {
	elapsed := mo.elapsedTime
	if mo.elapsedTime == 0 && mo.taskProgress == entities.InProgress {
		// If the operation is in progress, retrieved the ongoing time.
		elapsed = time.Now().Sub(mo.started).Nanoseconds()
	}
	return entities.OperationResult{
		RequestId:        mo.request.RequestID,
		Type:             entities.Management,
		Progress:         mo.taskProgress,
		ElapsedTime:      elapsed,
		ErrorMsg:         mo.errorMsg,
		KubeConfigResult: mo.KubeConfigResult,
	}
}
