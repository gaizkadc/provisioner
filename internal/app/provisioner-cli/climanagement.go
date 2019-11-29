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

package provisioner_cli

import (
	"fmt"
	"time"

	"github.com/nalej/derrors"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider"
	providerEntities "github.com/nalej/provisioner/internal/app/provisioner/provider/entities"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/nalej/provisioner/internal/pkg/workflow"
	"github.com/rs/zerolog/log"
)

// CLIManagement structure to watch the provisioning process.
type CLIManagement struct {
	*CLICommon
	request   *grpc_provisioner_go.ClusterRequest
	Operation entities.ManagementOperationType
	Executor  workflow.Executor
	config    *config.Config
}

// NewCLIManagement creates a new CLI for management operations.
func NewCLIManagement(
	request *grpc_provisioner_go.ClusterRequest,
	operation entities.ManagementOperationType,
	kubeConfigOutputPath string,
	config *config.Config) *CLIManagement {
	return &CLIManagement{
		CLICommon: &CLICommon{lastLogEntry: 0, kubeConfigOutputPath: kubeConfigOutputPath},
		request:   request,
		Operation: operation,
		Executor:  workflow.GetExecutor(),
		config:    config,
	}
}

// Run triggers the provisioning of a cluster.
func (cm *CLIManagement) Run() derrors.Error {
	vErr := cm.config.Validate()
	if vErr != nil {
		log.Fatal().Str("err", vErr.DebugReport()).Msg("invalid configuration")
	}
	cm.config.Print()
	log.Debug().Str("target_platform", cm.request.TargetPlatform.String()).Bool("isManagementCluster", cm.request.IsManagementCluster).Msg("Cluster request received")
	infraProvider, err := provider.NewInfrastructureProvider(cm.request.TargetPlatform, cm.request.AzureCredentials, cm.config)
	if err != nil {
		log.Error().Msg("cannot obtain infrastructure provider")
		return err
	}

	if cm.Operation == entities.GetKubeConfig {
		return cm.GetKubeConfig(infraProvider)
	} else {
		log.Error().Msg("unsupported operation")
	}

	return nil
}

func (cm *CLIManagement) GetKubeConfig(infraProvider providerEntities.InfrastructureProvider) derrors.Error {
	operation, err := infraProvider.GetKubeConfig(entities.NewClusterRequest(cm.request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create get kubeconfig operation")
		return err
	}
	start := time.Now()
	checks := 0
	wfc := &WaitForCompletion{Called: false}
	operation.SetProgress(entities.InProgress)
	operation.Execute(wfc.finished)
	for !wfc.Called {
		time.Sleep(15 * time.Second)
		cm.printOperationLog(operation.Log())
		if checks%4 == 0 {
			fmt.Printf("GetKubeConfig operation %s - %s\n", entities.TaskProgressToString[operation.Progress()], time.Since(start).String())
		}
		checks++
	}
	elapsed := time.Since(start)
	fmt.Println("Retrieving kubeconfig took ", elapsed)
	// Process the result
	result := operation.Result()
	cm.printTableResult(result)
	return nil
}

// printResult prints the result of the command.
func (cm *CLIManagement) printTableResult(result entities.OperationResult) {
	writer := NewTabWriterHelper()
	writer.Println("Request:\t", result.RequestId)
	writer.Println("Type:\t", entities.ToOperationTypeString[result.Type])
	writer.Println("Progress:\t", entities.TaskProgressToString[result.Progress])
	writer.Println("Elapsed Time:\t", time.Duration(result.ElapsedTime).String())
	if result.Progress == entities.Error {
		writer.Println("Error:\t", result.ErrorMsg)
	} else {
		if result.KubeConfigResult != nil {
			writer.Println("KubeConfig:\t", cm.writeKubeConfig(cm.request.ClusterId, *result.KubeConfigResult))
		} else {
			log.Warn().Msg("expecting kubeconfig result")
		}
	}
	err := writer.Flush()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot write result to stdout")
	}
}

type WaitForCompletion struct {
	Called bool
}

func (wfc *WaitForCompletion) finished(requestID string) {
	wfc.Called = true
}
