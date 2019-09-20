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
	request  *grpc_provisioner_go.ProvisionClusterRequest
	Executor workflow.Executor
}

func NewCLIProvisioner(request *grpc_provisioner_go.ProvisionClusterRequest) *CLIProvisioner {
	return &CLIProvisioner{
		request:  request,
		Executor: workflow.GetExecutor(),
	}
}

func (cp *CLIProvisioner) Run() derrors.Error {
	log.Debug().Str("target_platform", cp.request.TargetPlatform.String()).Msg("Provision request received")
	provider, err := provider.NewInfrastructureProvider(cp.request.TargetPlatform, cp.request.AzureCredentials)
	if err != nil {
		log.Error().Msg("cannot obtain infrastructure provider")
		return err
	}
	operation, err := provider.Provision(entities.NewProvisionRequest(cp.request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create provision operation")
		return err
	}
	cp.Executor.ScheduleOperation(operation)
	start := time.Now()
	checks := 0
	for cp.Executor.IsManaged(cp.request.RequestId) {
		time.Sleep(15 * time.Second)
		if checks%4 == 0 {
			fmt.Printf("Provision operation %s - %s\n", entities.TaskProgressToString[operation.Progress()], time.Since(start).String())
		}
		checks++
	}
	elapsed := time.Since(start)
	fmt.Println("Provisioning took ", elapsed)
	// Process the result
	result := operation.Result()
	cp.printResult(result)
	return nil
}

// printResult prints the result of the command.
func (cp *CLIProvisioner) printResult(result entities.OperationResult){
	fmt.Printf("Request:\t%s\n", result.RequestId)
	fmt.Printf("Type:\t%s\n", entities.ToOperationTypeString[result.Type])
	fmt.Printf("Progress:\t%s\n",  entities.TaskProgressToString[result.Progress])
	fmt.Printf("Elapsed Time:\t%s\n",  result.ElapsedTime)
	if result.Progress == entities.Error{
		fmt.Printf("Error:\t%s\n",  result.ErrorMsg)
	}else{
		if result.ProvisionResult != nil{
	// TODO Write the kubeconfig result
			log.Debug().Str("raw", result.ProvisionResult.RawKubeConfig).Msg("KubeConfig")
		}else{
			log.Warn().Msg("expecting provisioning result")
		}
	}

}