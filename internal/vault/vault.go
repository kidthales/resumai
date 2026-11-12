package vault

import (
	"github.com/kidthales/resumai/internal/domain"
)

// Vault defines the contract for persistent career facts and job applications.
type Vault interface {
	GetCareerFact(id string) (*domain.CareerFact, error)
	GetApplication(id string) (*domain.JobApplication, error)
}
