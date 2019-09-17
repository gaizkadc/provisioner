package provisioner_cli

import (
	"fmt"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/nalej/provisioner/internal/pkg/workflow"
	"github.com/rs/zerolog/log"
	"time"
)

// CLIProvisioner structure to watch the provisioning process.
type CLIProvisioner struct {
	request *grpc_provisioner_go.ProvisionClusterRequest
	Executor workflow.Executor
}

func NewCLIProvisioner(request *grpc_provisioner_go.ProvisionClusterRequest) *CLIProvisioner{
	return &CLIProvisioner{
		request:  request,
		Executor: workflow.GetExecutor(),
	}
}

func (cp * CLIProvisioner) Run() derrors.Error{
	log.Debug().Str("target_platform", cp.request.TargetPlatform.String()).Msg("Provision request received")
	provider, err := provider.NewInfrastructureProvider(cp.request.TargetPlatform, cp.request.AzureCredentials)
	if err != nil{
		log.Error().Msg("cannot obtain infrastructure provider")
		return err
	}
	operation, err := provider.Provision(entities.NewProvisionRequest(cp.request))
	if err != nil{
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create provision operation")
		return err
	}
	cp.Executor.ScheduleOperation(operation)
	start := time.Now()
	checks := 0
	for cp.Executor.IsManaged(cp.request.RequestId){
		if checks % 4 == 0{
			fmt.Printf("Provision operation %s - %s\n", operation.Progress(), time.Since(start).String())
		}
		checks++
	}
	elapsed := time.Since(start)
	fmt.Println("Provisioning took ", elapsed)
	// Process the result
	result := operation.Result()
	if result.Progress == entities.Error{
		fmt.Println("Provisioning failed")
		return derrors.NewInternalError(result.ErrorMsg)
	}
	task_elapsed := time.Duration(result.ElapsedTime) * time.Millisecond
	log.Debug().Str("elapsed_time", task_elapsed.String()).Msg("result metadata")
	if result.ProvisionResult != nil{
		log.Debug().Str("raw", result.ProvisionResult.RawKubeConfig).Msg("KubeConfig")
	}
	// TODO Write the kubeconfig result
	return nil
}

