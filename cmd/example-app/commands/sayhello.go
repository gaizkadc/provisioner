/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

// This is an example of an executable command.

package commands

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)


var helloCmd = &cobra.Command{
	Use:   "hello",
	Short: "Print a hello message",
	Long:  `A long description about what is a hello message`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		log.Info().Msg("Hello!")
	},
}

func init() {
	rootCmd.AddCommand(helloCmd)
}