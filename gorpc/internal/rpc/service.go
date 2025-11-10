package rpc

import (
	"time"

	"github.com/stashapp/stash/pkg/plugin/common"
	"github.com/stashapp/stash/pkg/plugin/common/log"
)

// NewService creates a new RPC service instance
func NewService() *Service {
	return &Service{}
}

// Stop handles graceful shutdown of the plugin
func (s *Service) Stop(input struct{}, output *bool) error {
	log.Info("Stopping Compreface plugin...")
	s.stopping = true
	*output = true
	return nil
}

// applyCooldown applies the configured cooldown period
func (s *Service) applyCooldown() {
	if s.config.CooldownSeconds > 0 {
		log.Infof("Cooling down for %d seconds to prevent hardware stress...", s.config.CooldownSeconds)
		time.Sleep(time.Duration(s.config.CooldownSeconds) * time.Second)
	}
}

// errorOutput creates an error output for RPC response
func (s *Service) errorOutput(output *common.PluginOutput, err error) error {
	errStr := err.Error()
	*output = common.PluginOutput{
		Error: &errStr,
	}
	return nil
}
