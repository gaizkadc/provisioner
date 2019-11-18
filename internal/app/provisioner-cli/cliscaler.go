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
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/nalej/provisioner/internal/pkg/workflow"
	"github.com/rs/zerolog/log"
	"time"
)

// CLIScaler structure to watch the scaling process.
type CLIScaler struct {
	*CLICommon
	request  *grpc_provisioner_go.ScaleClusterRequest
	Executor workflow.Executor
	config   *config.Config
}

// NewCLIScaler creates a new CLI managed scaler without a service.
func NewCLIScaler(
	request *grpc_provisioner_go.ScaleClusterRequest,
	config *config.Config) *CLIScaler {
	return &CLIScaler{
		CLICommon: &CLICommon{lastLogEntry: 0},
		request:   request,
		Executor:  workflow.GetExecutor(),
		config:    config,
	}
}

func (cs *CLIScaler) Run() derrors.Error {
	vErr := cs.config.Validate()
	if vErr != nil {
		log.Fatal().Str("err", vErr.DebugReport()).Msg("invalid configuration")
	}
	cs.config.Print()
	log.Debug().Str("target_platform", cs.request.TargetPlatform.String()).Msg("Scale request received")
	infraProvider, err := provider.NewInfrastructureProvider(cs.request.TargetPlatform, cs.request.AzureCredentials, cs.config)
	if err != nil {
		log.Error().Msg("cannot obtain infrastructure provider")
		return err
	}
	operation, err := infraProvider.Scale(entities.NewScaleRequest(cs.request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create provision operation")
		return err
	}

	cs.Executor.ScheduleOperation(operation)
	start := time.Now()
	checks := 0
	for cs.Executor.IsManaged(cs.request.RequestId) {
		time.Sleep(15 * time.Second)
		cs.printOperationLog(operation.Log())
		if checks%4 == 0 {
			fmt.Printf("Provision operation %s - %s\n", entities.TaskProgressToString[operation.Progress()], time.Since(start).String())
		}
		checks++
	}
	elapsed := time.Since(start)
	fmt.Println("Provisioning took ", elapsed)
	// Process the result
	result := operation.Result()
	cs.printJSONResult("unknown", result)
	// cp.printTableResult(result)
	return nil
}
