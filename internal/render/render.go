package render

import (
	"github.com/kidthales/resumai/internal/domain"
)

// Renderer defines the contract for rendering validated resume documents.
type Renderer interface {
	RenderHTML(app *domain.JobApplication) ([]byte, error)
	RenderDocx(app *domain.JobApplication) ([]byte, error)
	RenderPDF(app *domain.JobApplication) ([]byte, error)
}
