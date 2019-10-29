package commands

import (
	grpc_infrastructure_go "github.com/nalej/grpc-infrastructure-go"
	grpc_installer_go "github.com/nalej/grpc-installer-go"
	grpc_provisioner_go "github.com/nalej/grpc-provisioner-go"
	provisioner_cli "github.com/nalej/provisioner/internal/app/provisioner-cli"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// ProvisionRequest contains the elements that will be requested from the user simulating a request.
var provisionRequest grpc_provisioner_go.ProvisionClusterRequest

// azureOptions contains the options specific for an Azure installation.
var azureOptions grpc_provisioner_go.AzureProvisioningOptions

// kubeConfigOutputPath with the path where the kubeconfig file should be stored after provisioning.
var kubeConfigOutputPath string

// Overall provisioner configuration
var cfg = &config.Config{}

// provisionCmd with the command to provision a new cluster.
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

// TriggerProvisioning triggers the execution of the CLI managed provisioning.
func TriggerProvisioning() {
	cliProvisioner := provisioner_cli.NewCLIProvisioner(&provisionRequest, kubeConfigOutputPath, cfg)
	err := cliProvisioner.Run()
	ExitOnError(err, "provisioning failed")
}

// ConfigureProvisioning configures the options using the standard gRPC structures for the provisioning command.
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
		if azureOptions.ResourceGroup == "" {
			log.Fatal().Msg("resourceGroup must be specified")
		}
		if azureOptions.DnsZoneName == "" {
			log.Fatal().Msg("dnsZoneName must be specified")
		}
		provisionRequest.AzureOptions = &azureOptions
	}
	cfg.LaunchService = false
}

func init() {
	provisionCmd.Flags().StringVar(&provisionRequest.ClusterName, "name", "",
		"Name of the cluster")
	provisionCmd.MarkFlagRequired("name")
	provisionCmd.Flags().StringVar(&provisionRequest.KubernetesVersion, "kubernetesVersion", "1.13.11",
		"Kubernetes version to be installed. The available versions depend on the target platform.")
	provisionCmd.Flags().Int64Var(&provisionRequest.NumNodes, "numNodes", 3,
		"Number of nodes in the cluster")
	provisionCmd.Flags().StringVar(&provisionRequest.NodeType, "nodeType", "",
		"Type of node to be requested")
	provisionCmd.MarkFlagRequired("nodeType")
	provisionCmd.Flags().StringVar(&provisionRequest.Zone, "zone", "",
		"Zone where the cluster must be created")
	provisionCmd.Flags().StringVar(&azureOptions.ResourceGroup, "resourceGroup", "",
		"Target resource group where the cluster will be created. Only for Azure platform.")
	provisionCmd.Flags().StringVar(&azureOptions.DnsZoneName, "dnsZoneName", "",
		"Name of the DNS zone where the entries will be added.")
	provisionCmd.Flags().StringVar(&targetPlatform, "platform", "",
		"Target plaftorm determining the provider: AZURE or BAREMETAL")
	provisionCmd.Flags().BoolVar(&provisionRequest.IsProduction, "isProduction", false,
		"Whether the provisioning if for a production cluster")
	provisionCmd.MarkFlagRequired("platform")
	provisionCmd.Flags().StringVar(&azureCredentialsPath, "azureCredentialsPath", "",
		"Path to the file containing the azure credentials")
	provisionCmd.Flags().StringVar(&kubeConfigOutputPath, "kubeConfigOutputPath", "/tmp/",
		"Path to directory where the resulting kubeconfig will be stored")
	provisionCmd.Flags().StringVar(&cfg.TempPath, "tempPath", "./temp/",
		"Directory to store temporal files")
	provisionCmd.Flags().StringVar(&cfg.ResourcesPath, "resourcesPath", "./resources/",
		"Directory with the provisioner resources files")
	rootCmd.AddCommand(provisionCmd)
}
