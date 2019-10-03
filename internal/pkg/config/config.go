/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package config

import (
	"github.com/nalej/derrors"
	"github.com/nalej/edge-inventory-proxy/version"
	"github.com/rs/zerolog/log"
)

type Config struct {
	// LaunchService determines if the service needs to be launched.
	LaunchService bool
	// Debug level is active.
	Debug bool
	// Port where the gRPC API service will listen requests.
	Port int
	// TempPath with the path where temporal files may be created.
	TempPath string
	// ResourcesPath with the path where extra YAML or resources are stored for some operation.
	ResourcesPath string
}

func (conf *Config) Validate() derrors.Error {
	if conf.LaunchService && conf.Port <= 0 {
		return derrors.NewInvalidArgumentError("port must be valid")
	}
	return nil
}

func (conf *Config) Print() {
	log.Info().Str("app", version.AppVersion).Str("commit", version.Commit).Msg("Version")
	if conf.LaunchService {
		log.Info().Int("port", conf.Port).Msg("gRPC port")
	}
	log.Info().Str("path", conf.TempPath).Msg("Temporal files")
	log.Info().Str("path", conf.ResourcesPath).Msg("Resources")
}
