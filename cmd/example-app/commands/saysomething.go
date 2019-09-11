/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

// This is an example of an executable command.

package commands

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"strings"
)

var cmdSaySomething = &cobra.Command{
	Use:   "something [string to print]",
	Short: "Print anything to the screen",
	Long: `print is for printing anything back to the screen.
For many years people have printed back to the screen.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		log.Info().Msgf("We say: %s", strings.Join(args, " "))
	},
}

func init() {
	rootCmd.AddCommand(cmdSaySomething)
}
