package render

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kidthales/resumai/internal/domain"
)

// FactResolver retrieves career facts by ID to verify provenance.
type FactResolver interface {
	GetCareerFact(id string) (*domain.CareerFact, error)
}

// FactResolverFunc allows ordinary functions to act as FactResolvers.
type FactResolverFunc func(id string) (*domain.CareerFact, error)

// GetCareerFact calls the underlying function to resolve a career fact.
func (f FactResolverFunc) GetCareerFact(id string) (*domain.CareerFact, error) {
	return f(id)
}

// ValidationError records a specific invariant violation on a document or bullet.
type ValidationError struct {
	BulletID string
	FactID   string
	Message  string
	Err      error
}

func (e ValidationError) Error() string {
	if e.BulletID != "" && e.FactID != "" {
		return fmt.Sprintf("bullet %q fact %q: %s", e.BulletID, e.FactID, e.Message)
	}
	if e.BulletID != "" {
		return fmt.Sprintf("bullet %q: %s", e.BulletID, e.Message)
	}
	if e.FactID != "" {
		return fmt.Sprintf("fact %q: %s", e.FactID, e.Message)
	}
	return e.Message
}

// Unwrap returns the underlying domain sentinel error.
func (e ValidationError) Unwrap() error {
	return e.Err
}

// ValidationReport aggregates all invariant violations discovered during document validation.
type ValidationReport struct {
	Errors []ValidationError
}

func (r *ValidationReport) Error() string {
	if len(r.Errors) == 0 {
		return "no validation errors"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d validation failure(s):\n", len(r.Errors)))
	for i, err := range r.Errors {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return strings.TrimSpace(sb.String())
}

// Unwrap returns the list of wrapped errors so errors.Is and errors.As work on the report.
func (r *ValidationReport) Unwrap() []error {
	errs := make([]error, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = e.Err
	}
	return errs
}

// HasErrors returns true if any validation violations were recorded.
func (r *ValidationReport) HasErrors() bool {
	return len(r.Errors) > 0
}

// Add appends a new ValidationError to the report.
func (r *ValidationReport) Add(bulletID, factID, message string, sentinel error) {
	r.Errors = append(r.Errors, ValidationError{
		BulletID: bulletID,
		FactID:   factID,
		Message:  message,
		Err:      sentinel,
	})
}

// ContainsMetricPlaceholder checks whether a string contains any unresolved [METRIC: placeholder.
func ContainsMetricPlaceholder(text string) bool {
	return strings.Contains(strings.ToUpper(text), "[METRIC:")
}

// DocumentValidator validates ResumeDocument AST invariants prior to rendering.
type DocumentValidator struct {
	resolver FactResolver
}

// NewDocumentValidator creates a validator using the provided FactResolver.
func NewDocumentValidator(resolver FactResolver) *DocumentValidator {
	return &DocumentValidator{resolver: resolver}
}

// ValidateBullet validates a single bullet node for placeholder absence and fact provenance.
func (v *DocumentValidator) ValidateBullet(bullet *Bullet) error {
	var report ValidationReport
	v.validateBulletInto(bullet, &report)
	if report.HasErrors() {
		return &report
	}
	return nil
}

func (v *DocumentValidator) validateBulletInto(bullet *Bullet, report *ValidationReport) {
	if bullet == nil {
		report.Add("", "", "nil bullet pointer encountered", domain.ErrUngroundedBullet)
		return
	}

	// 1. Placeholder check: [METRIC: tokens are strictly prohibited before render
	hasMetric := false
	if ContainsMetricPlaceholder(bullet.Text) {
		report.Add(bullet.ID, "", fmt.Sprintf("contains unresolved metric placeholder in text %q", bullet.Text), domain.ErrUnverifiedFactOrPlaceholder)
		hasMetric = true
	}
	if !hasMetric {
		for _, run := range bullet.Runs {
			if ContainsMetricPlaceholder(run.Text) {
				report.Add(bullet.ID, "", fmt.Sprintf("contains unresolved metric placeholder in inline run %q", run.Text), domain.ErrUnverifiedFactOrPlaceholder)
				break
			}
		}
	}

	// 2. Fact grounding check: at least one non-empty fact_id must be cited
	validFactIDs := make([]string, 0, len(bullet.FactIDs))
	for _, id := range bullet.FactIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			validFactIDs = append(validFactIDs, trimmed)
		}
	}

	if len(validFactIDs) == 0 {
		report.Add(bullet.ID, "", "bullet has zero cited career fact IDs", domain.ErrUngroundedBullet)
		return
	}

	// 3. Provenance & verification check: 100% cited facts must be verified
	if v.resolver != nil {
		for _, factID := range validFactIDs {
			fact, err := v.resolver.GetCareerFact(factID)
			if err != nil || fact == nil {
				report.Add(bullet.ID, factID, fmt.Sprintf("cited career fact %q could not be resolved from vault: %v", factID, err), domain.ErrUngroundedBullet)
				continue
			}
			if !fact.Verified || fact.EpistemicState != domain.EpistemicVerified {
				report.Add(bullet.ID, factID, fmt.Sprintf("cited career fact %q is not verified (verified=%t, epistemic_state=%q)", factID, fact.Verified, fact.EpistemicState), domain.ErrUnverifiedFactOrPlaceholder)
			}
		}
	} else {
		// Fallback check on bullet's own Verified flag when operating without an external vault resolver
		if !bullet.Verified {
			report.Add(bullet.ID, "", "bullet is not marked verified", domain.ErrUnverifiedFactOrPlaceholder)
		}
	}
}

// ValidateDocument validates the full ResumeDocument AST.
func (v *DocumentValidator) ValidateDocument(doc *ResumeDocument) error {
	if doc == nil {
		return errors.New("cannot validate nil ResumeDocument")
	}

	var report ValidationReport

	// Check header and summary placeholders
	if ContainsMetricPlaceholder(doc.Header.Headline) {
		report.Add("", "", fmt.Sprintf("headline contains unresolved metric placeholder %q", doc.Header.Headline), domain.ErrUnverifiedFactOrPlaceholder)
	}
	if doc.Summary != nil {
		hasMetric := false
		if ContainsMetricPlaceholder(doc.Summary.Text) {
			report.Add("", "", fmt.Sprintf("summary text contains unresolved metric placeholder %q", doc.Summary.Text), domain.ErrUnverifiedFactOrPlaceholder)
			hasMetric = true
		}
		if !hasMetric {
			for _, run := range doc.Summary.Runs {
				if ContainsMetricPlaceholder(run.Text) {
					report.Add("", "", fmt.Sprintf("summary run contains unresolved metric placeholder %q", run.Text), domain.ErrUnverifiedFactOrPlaceholder)
					break
				}
			}
		}
	}

	// Validate all bullets across all sections
	bullets := doc.AllBullets()
	for _, b := range bullets {
		v.validateBulletInto(b, &report)
	}

	if report.HasErrors() {
		return &report
	}
	return nil
}

// ValidateReadyToRender enforces strict pre-render invariants:
// 1. Document must be non-nil with a non-empty FullName.
// 2. Document must contain at least one Experience or Education item.
// 3. Document must contain at least one bullet.
// 4. 100% cited facts must be verified in vault.
// 5. Zero [METRIC: placeholders across all sections and runs.
func (v *DocumentValidator) ValidateReadyToRender(doc *ResumeDocument) error {
	if doc == nil {
		return errors.New("cannot render nil ResumeDocument")
	}

	var report ValidationReport

	if strings.TrimSpace(doc.Header.FullName) == "" {
		report.Add("", "", "document header missing full_name", errors.New("missing candidate name"))
	}

	if len(doc.Experience) == 0 && len(doc.Education) == 0 {
		report.Add("", "", "document contains neither experience nor education sections", errors.New("empty resume body"))
	}

	bullets := doc.AllBullets()
	if len(bullets) == 0 {
		report.Add("", "", "document contains zero accomplishment bullets", domain.ErrUngroundedBullet)
	}

	// Run full AST provenance and placeholder validation
	if err := v.ValidateDocument(doc); err != nil {
		var vr *ValidationReport
		if errors.As(err, &vr) {
			report.Errors = append(report.Errors, vr.Errors...)
		} else {
			report.Add("", "", err.Error(), err)
		}
	}

	if report.HasErrors() {
		return &report
	}
	return nil
}

// ValidateDocument is a convenience package-level function for validating a document with a resolver.
func ValidateDocument(doc *ResumeDocument, resolver FactResolver) error {
	return NewDocumentValidator(resolver).ValidateDocument(doc)
}

// ValidateReadyToRender is a convenience package-level function for pre-render validation.
func ValidateReadyToRender(doc *ResumeDocument, resolver FactResolver) error {
	return NewDocumentValidator(resolver).ValidateReadyToRender(doc)
}
