package commands

import (
	"github.com/nalej/grpc-infrastructure-go"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
	provisioner_cli "github.com/nalej/provisioner/internal/app/provisioner-cli"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// ProvisionRequest contains the elements that will be requested from the user simulating a request.
var provisionRequest grpc_provisioner_go.ProvisionClusterRequest

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision a new cluster",
	Long:  `Provision a new cluster using a specific infrastructure provider`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		log.Info().Msg("Provisioning cluster")
		ConfigureProvisioning()
		TriggerProvisioning()
	},
}

func TriggerProvisioning() {
	cliProvisioner := provisioner_cli.NewCLIProvisioner(&provisionRequest)
	err := cliProvisioner.Run()
	ExitOnError(err, "provisioning failed")
}

func ConfigureProvisioning() {
	provisionRequest.RequestId = "cli-request"
	provisionRequest.OrganizationId = "nalej"
	provisionRequest.ClusterId = "mngt"
	// From the CLI only management clusters may be provisioned.
	provisionRequest.IsManagementCluster = true
	// Only kubernetes clusters for now
	provisionRequest.ClusterType = grpc_infrastructure_go.ClusterType_KUBERNETES
	// Determine target platform
	targetPlatform, err := GetTargetPlatform(targetPlatform)
	ExitOnError(err, "cannot determine target platform")
	provisionRequest.TargetPlatform = targetPlatform

	// Load credentials depending on the target platform
	if provisionRequest.TargetPlatform == grpc_installer_go.Platform_AZURE {
		if azureCredentialsPath == "" {
			log.Fatal().Msg("azureCredentialsPath must be specified")
		}
		credentials, err := LoadAzureCredentials(azureCredentialsPath)
		ExitOnError(err, "cannot load infrastructure provider credentials")
		provisionRequest.AzureCredentials = credentials
	}
}

func init() {
	provisionCmd.Flags().Int64Var(&provisionRequest.NumNodes, "numNodes", 3, "Number of nodes in the cluster")
	provisionCmd.Flags().StringVar(&provisionRequest.NodeType, "nodeType", "", "Type of node to be requested")
	provisionCmd.Flags().StringVar(&provisionRequest.Zone, "Zone", "", "Zone where the cluster must be created")
	provisionCmd.Flags().StringVar(&targetPlatform, "platform", "", "Target plaftorm determining the provider: AZURE or BAREMETAL")
	provisionCmd.MarkFlagRequired("platform")
	provisionCmd.Flags().StringVar(&azureCredentialsPath, "azureCredentialsPath", "", "Path to the file containing the azure credentials")
	rootCmd.AddCommand(provisionCmd)
}
