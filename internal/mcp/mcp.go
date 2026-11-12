package mcp

import (
	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/vault"
)

// Server coordinates MCP tool calls against domain logic and vault storage.
type Server struct {
	vault vault.Vault
}

// NewServer initializes a new MCP Server instance.
func NewServer(v vault.Vault) *Server {
	return &Server{
		vault: v,
	}
}

// HandleTriage validates and records initial triage for a role.
func (s *Server) HandleTriage(rawJD string) (*domain.JobApplication, error) {
	if rawJD == "" {
		return nil, domain.ErrInvalidTriageCitation
	}
	return &domain.JobApplication{
		RawJD: rawJD,
		State: domain.StateTriaged,
	}, nil
}
