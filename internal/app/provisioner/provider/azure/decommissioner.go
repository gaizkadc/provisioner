/*
 * Copyright 2020 Nalej
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
	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-08-01/containerservice"
	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	"github.com/Azure/go-autorest/autorest"
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/pkg/common"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	"time"
)

const ClusterDecommissionDeadline = 30 * time.Minute

type DecommissionerOperation struct {
	*AzureOperation
	request entities.DecommissionRequest
	config  *config.Config
}

func NewDecommissionerOperation(credentials *AzureCredentials, request entities.DecommissionRequest, config *config.Config) (*DecommissionerOperation, derrors.Error) {
	azureOp, err := NewAzureOperation(credentials)
	if err != nil {
		return nil, err
	}
	return &DecommissionerOperation{
		AzureOperation: azureOp,
		request:        request,
		config:         config,
	}, nil
}

func (do *DecommissionerOperation) RequestID() string {
	return do.request.RequestID
}

func (do *DecommissionerOperation) Metadata() entities.OperationMetadata {
	return entities.OperationMetadata{
		OrganizationID: do.request.OrganizationID,
		ClusterID:      do.request.ClusterID,
		RequestID:      do.request.RequestID,
	}
}

func (do *DecommissionerOperation) notifyError(err derrors.Error, callback func(requestId string)) {
	log.Error().Str("trace", err.DebugReport()).Msg("decommission operation failed")
	do.setError(err.Error())
	callback(do.request.RequestID)
}

func (do *DecommissionerOperation) Execute(callback func(requestID string)) {
	log.Debug().Str("organizationID", do.request.OrganizationID).Str("clusterID", do.request.ClusterID).Msg("executing decommission operation")
	do.started = time.Now()
	do.SetProgress(entities.InProgress)

	do.AddToLog("Obtaining Cluster information")
	managedCluster, err := do.getClusterDetails(do.request.IsManagementCluster, do.request.AzureOptions.ResourceGroup, do.request.ClusterID)
	if err != nil {
		do.notifyError(err, callback)
		return
	}
	dnsZoneName := managedCluster.Tags[DnsZoneTag]
	if dnsZoneName == nil {
		do.notifyError(derrors.NewFailedPreconditionError(fmt.Sprintf("Cluster entity does not contain needed tag [%s]", DnsZoneTag)), callback)
		return
	}
	clusterName := managedCluster.Tags[ClusterNameTag]
	if clusterName == nil {
		do.notifyError(derrors.NewFailedPreconditionError(fmt.Sprintf("Cluster entity does not contain needed tag [%s]", ClusterNameTag)), callback)
		return
	}

	do.AddToLog("Obtaining DNS zone information")
	zone, err := do.getDNSZone(*dnsZoneName)
	if err != nil {
		do.notifyError(err, callback)
		return
	}
	dnsZoneResourceGroupName, err := do.getDNSResourceGroupName(zone)
	if err != nil {
		do.notifyError(err, callback)
		return
	}

	do.AddToLog("Deleting DNS entries")
	err = do.deleteDNSEntries(*clusterName, *dnsZoneResourceGroupName, *dnsZoneName)
	if err != nil {
		do.notifyError(err, callback)
		return
	}

	do.AddToLog("Decommissioning cluster")
	decommissionResponse, err := do.decommissionAksCluster()
	if err != nil {
		do.notifyError(err, callback)
		return
	}
	do.AddToLog("cluster has been decommissioned")
	log.Debug().Interface("response", *decommissionResponse).Msg("cluster has been decommissioned")

	do.elapsedTime = time.Now().Sub(do.started).Nanoseconds()
	do.SetProgress(entities.Finished)
	callback(do.request.RequestID)
}

func (do *DecommissionerOperation) Cancel() derrors.Error {
	panic("implement me") // TODO implement DecommissionerOperation.Cancel()
}

func (do *DecommissionerOperation) Result() entities.OperationResult {
	elapsed := do.elapsedTime
	if do.elapsedTime == 0 && do.taskProgress == entities.InProgress {
		// If the operation is in progress, retrieve the ongoing time.
		elapsed = time.Now().Sub(do.started).Nanoseconds()
	}

	return entities.OperationResult{
		RequestId:   do.request.RequestID,
		Type:        entities.Decommission,
		Progress:    do.taskProgress,
		ElapsedTime: elapsed,
		ErrorMsg:    do.errorMsg,
	}
}

func (do *DecommissionerOperation) deleteDNSEntries(clusterName string, resourceGroupName string, dnsZoneName string) derrors.Error {
	recordsetTypeA, err := do.listDnsRecords(resourceGroupName, dnsZoneName, clusterName)
	if err != nil {
		return err
	}
	recordsetTypeNS, err := do.listDnsRecords(resourceGroupName, dnsZoneName, fmt.Sprintf("%s.%s", clusterName, dnsZoneName))
	if err != nil {
		return err
	}
	toRemove := make([]dns.RecordSet, 0, len(recordsetTypeA)+len(recordsetTypeNS))
	toRemove = append(toRemove, recordsetTypeA...)
	toRemove = append(toRemove, recordsetTypeNS...)

	for _, recordSet := range toRemove {
		if recordSet.Name == nil {
			log.Debug().
				Str("resourceGroupName", resourceGroupName).
				Str("DNSZoneName", dnsZoneName).
				Interface("recordSet", recordSet).
				Msg("recovered a DNS recordset without name")
			continue
		}
		dnsRecordName := *recordSet.Name
		log.Debug().
			Str("resourceGroupName", resourceGroupName).
			Str("dnsRecordName", dnsRecordName).
			Str("DNSZoneName", dnsZoneName).
			Msg("Deleting DNS A entry")
		_, err := do.deleteDNSARecord(resourceGroupName, dnsRecordName, dnsZoneName)
		if err != nil {
			return err
		}
		do.AddToLog(fmt.Sprintf("DNS record set deleted %s", dnsRecordName))
	}
	return nil
}

// ScaleAKS triggers the scaling of an existing management cluster.
func (do *DecommissionerOperation) decommissionAksCluster() (*autorest.Response, derrors.Error) {
	do.AddToLog("Scaling existing cluster")
	clusterClient := containerservice.NewManagedClustersClient(do.credentials.SubscriptionId)
	clusterClient.Authorizer = do.managementAuthorizer

	existingCluster, err := do.getClusterDetails(do.request.IsManagementCluster, do.request.AzureOptions.ResourceGroup, do.request.ClusterID)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("existingCluster", existingCluster).Msg("AKS cluster retrieved")

	ctx, cancel := common.GetContext()
	defer cancel()

	resourceName := do.getResourceName(do.request.IsManagementCluster, do.request.ClusterID)
	log.Debug().Str("resourceGroupName", do.request.AzureOptions.ResourceGroup).Str("resourceName", resourceName).Msg("Delete params")
	deleteFuture, deleteErr := clusterClient.Delete(ctx, do.request.AzureOptions.ResourceGroup, resourceName)
	if deleteErr != nil {
		return nil, derrors.NewInternalError("cannot decommission AKS cluster", deleteErr).WithParams(do.request)
	}

	do.AddToLog("waiting for AKS cluster to be decommissioned")
	futureContext, cancelFuture := context.WithTimeout(context.Background(), ClusterDecommissionDeadline)
	defer cancelFuture()
	waitErr := deleteFuture.WaitForCompletionRef(futureContext, clusterClient.Client)
	if waitErr != nil {
		return nil, derrors.AsError(waitErr, "AKS cluster decommission failed")
	}
	decommissionResponse, resultErr := deleteFuture.Result(clusterClient)
	if resultErr != nil {
		log.Error().Interface("err", resultErr).Msg("AKS decommission failed")
		return nil, derrors.AsError(resultErr, "AKS decommission failed")
	}
	return &decommissionResponse, nil
}
