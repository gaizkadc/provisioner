/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package commands

import (
	"github.com/nalej/provisioner/internal/app/provisioner"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var cfg = config.Config{}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Launch the server API",
	Long:  `Launch the server API`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		log.Info().Msg("Launching provisioner")
		cfg.Debug = debugLevel
		cfg.LaunchService = true
		service := provisioner.NewService(cfg)
		err := service.Run()
		if err != nil{
			log.Fatal().Err(err).Msg("unable to run provisioner service")
		}
	},
}

func init() {
	runCmd.Flags().IntVar(&cfg.Port, "port", 9010, "Port to launch the provisioner gRPC API")
	runCmd.Flags().StringVar(&cfg.TempPath, "tempPath", "./temp/",
		"Directory to store temporal files")
	runCmd.Flags().StringVar(&cfg.ResourcesPath, "resourcesPath", "./resources/",
		"Directory with the provisioner resources files")
	rootCmd.AddCommand(runCmd)
}