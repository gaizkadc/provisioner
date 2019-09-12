package provisioner

import (
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/rs/zerolog/log"
	"time"
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

	for ; ;  {
		log.Debug().Msg("waiting")
		time.Sleep(time.Minute)
	}

	return nil
}