/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package provisioner

import (
	"fmt"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/decomissioner"
	"github.com/nalej/provisioner/internal/app/provisioner/provisioner"
	"github.com/nalej/provisioner/internal/app/provisioner/scaler"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

type Service struct {
	Configuration config.Config
}

// NewService creates a new system model service.
func NewService(conf config.Config) *Service {
	return &Service{
		conf,
	}
}

func (s *Service) Run() error {
	vErr := s.Configuration.Validate()
	if vErr != nil {
		log.Fatal().Str("err", vErr.DebugReport()).Msg("invalid configuration")
	}
	s.Configuration.Print()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Configuration.Port))
	if err != nil {
		log.Fatal().Errs("failed to listen: %v", []error{err})
	}

	provisionerManager := provisioner.NewManager(s.Configuration)
	provisionerHandler := provisioner.NewHandler(provisionerManager)

	decomissionManager := decomissioner.NewManager(s.Configuration)
	decomissionHandler := decomissioner.NewHandler(decomissionManager)

	scaleManager := scaler.NewManager(s.Configuration)
	scaleHandler := scaler.NewHandler(scaleManager)

	grpcServer := grpc.NewServer()
	grpc_provisioner_go.RegisterProvisionServer(grpcServer, provisionerHandler)
	grpc_provisioner_go.RegisterDecomissionServer(grpcServer, decomissionHandler)
	grpc_provisioner_go.RegisterScaleServer(grpcServer, scaleHandler)

	if s.Configuration.Debug {
		log.Info().Msg("Enabling gRPC server reflection")
		// Register reflection service on gRPC server.
		reflection.Register(grpcServer)
	}
	log.Info().Msg("Launching gRPC server")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Errs("failed to serve: %v", []error{err})
	}
	return nil
}
