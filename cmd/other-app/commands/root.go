/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

package commands

import (
	"github.com/nalej/golang-template/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"fmt"
	"os"
)

var debugLevel bool
var consoleLogging bool

var msg = `          ----------
         |  --------  |
         | |########| |       __________
         | |########| |      /__________\
 --------|  --------  |------|    --=-- |-------------
|         ----,-,-----'      |  ======  |             |
|       ______|_|_______     |__________|             |
|      /  %%%%%%%%%%%%  \                             |
|     /  %%%%%%%%%%%%%%  \                            |
|     ^^^^^^^^^^^^^^^^^^^^                            |
+-----------------------------------------------------+
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^`

var rootCmd = &cobra.Command{
	Use:   "example",
	Short: "Example of a cobra command",
	Long:  `A long explanation about what is a cobra command`,
	Version: "unknown-version",
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		log.Info().Msg("You're running other example application.")
		fmt.Println(msg)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debugLevel, "debug", false, "Set debug level")
	rootCmd.PersistentFlags().BoolVar(&consoleLogging, "consoleLogging", false, "Pretty print logging")
}

func Execute() {
	rootCmd.SetVersionTemplate(version.GetVersionInfo())
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// SetupLogging sets the debug level and console logging if required.
func SetupLogging() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debugLevel {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if consoleLogging {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}
