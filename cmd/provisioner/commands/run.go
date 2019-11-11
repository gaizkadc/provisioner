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
		if err != nil {
			log.Fatal().Err(err).Msg("unable to run provisioner service")
		}
	},
}

func init() {
	runCmd.Flags().IntVar(&cfg.Port, "port", 8930, "Port to launch the provisioner gRPC API")
	runCmd.Flags().StringVar(&cfg.TempPath, "tempPath", "./temp/",
		"Directory to store temporal files")
	runCmd.Flags().StringVar(&cfg.ResourcesPath, "resourcesPath", "./resources/",
		"Directory with the provisioner resources files")
	rootCmd.AddCommand(runCmd)
}
