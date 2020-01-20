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

package provisioner_cli

import (
	"fmt"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/nalej/provisioner/internal/pkg/workflow"
	"github.com/rs/zerolog/log"
	"time"
)

// CLIDecommissioner structure to watch the decommission process.
type CLIDecommissioner struct {
	*CLICommon
	request  *grpc_provisioner_go.DecomissionClusterRequest
	Executor workflow.Executor
	config   *config.Config
}

// NewCLIDecommissioner creates a new CLI managed decommissioner without a service.
func NewCLIDecommissioner(
	request *grpc_provisioner_go.DecomissionClusterRequest,
	config *config.Config) *CLIDecommissioner {
	return &CLIDecommissioner{
		CLICommon: &CLICommon{lastLogEntry: 0},
		request:   request,
		Executor:  workflow.GetExecutor(),
		config:    config,
	}
}

func (cs *CLIDecommissioner) Run() derrors.Error {
	vErr := cs.config.Validate()
	if vErr != nil {
		log.Error().Str("err", vErr.DebugReport()).Msg("invalid configuration")
		return vErr
	}
	cs.config.Print()
	log.Debug().Str("target_platform", cs.request.TargetPlatform.String()).Msg("Decommission request received")
	infraProvider, err := provider.NewInfrastructureProvider(cs.request.TargetPlatform, cs.request.AzureCredentials, cs.config)
	if err != nil {
		log.Error().Str("provider", cs.request.TargetPlatform.String()).Msg("cannot obtain infrastructure provider")
		return err
	}
	operation, err := infraProvider.Decommission(entities.NewDecommissionRequest(cs.request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create decommission operation")
		return err
	}

	cs.Executor.ScheduleOperation(operation)
	start := time.Now()
	checks := 0
	for cs.Executor.IsManaged(cs.request.RequestId) {
		time.Sleep(15 * time.Second)
		cs.printOperationLog(operation.Log())
		if checks%4 == 0 {
			fmt.Printf("Decommission operation %s - %s\n", entities.TaskProgressToString[operation.Progress()], time.Since(start).String())
		}
		checks++
	}
	elapsed := time.Since(start)
	fmt.Println("Decommissioning took ", elapsed)
	// Process the result
	result := operation.Result()
	cs.printJSONResult(cs.request.ClusterId, result)
	// cp.printTableResult(result)
	return nil
}
