package render_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/render"
)

// mockResolver is a test double for render.FactResolver.
type mockResolver struct {
	facts map[string]*domain.CareerFact
}

func newMockResolver() *mockResolver {
	return &mockResolver{facts: make(map[string]*domain.CareerFact)}
}

func (m *mockResolver) addFact(f *domain.CareerFact) {
	m.facts[f.ID] = f
}

func (m *mockResolver) GetCareerFact(id string) (*domain.CareerFact, error) {
	fact, ok := m.facts[id]
	if !ok {
		return nil, fmt.Errorf("fact %q: %w", id, domain.ErrNotFound)
	}
	return fact, nil
}

func TestContainsMetricPlaceholder(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Spearheaded initiative reducing latency by [METRIC:latency].", true},
		{"Cut infrastructure cost by [metric:35%].", true},
		{"[Metric: cloud_spend] savings achieved.", true},
		{"[METRIC:throughput_gain]", true},
		{"Increased revenue by 45% year-over-year.", false},
		{"Normal bullet text without placeholders.", false},
		{"", false},
	}

	for _, tt := range tests {
		actual := render.ContainsMetricPlaceholder(tt.input)
		if actual != tt.expected {
			t.Errorf("ContainsMetricPlaceholder(%q) = %v; expected %v", tt.input, actual, tt.expected)
		}
	}
}

func TestValidateBullet_Placeholders(t *testing.T) {
	resolver := newMockResolver()
	fact := &domain.CareerFact{
		ID:             "fact-1",
		Headline:       "Reduced latency",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	resolver.addFact(fact)

	validator := render.NewDocumentValidator(resolver)

	// Case 1: Placeholder in bullet text
	b1 := render.NewBullet("b1", "Improved latency by [METRIC:latency] in production", []string{"fact-1"}, true)
	err1 := validator.ValidateBullet(&b1)
	if err1 == nil {
		t.Fatal("expected error on bullet with [METRIC:...] placeholder, got nil")
	}
	if !errors.Is(err1, domain.ErrUnverifiedFactOrPlaceholder) {
		t.Errorf("expected ErrUnverifiedFactOrPlaceholder, got %v", err1)
	}

	// Case 2: Placeholder in inline runs
	b2 := render.Bullet{
		ID:      "b2",
		Text:    "Clean text",
		Runs:    []render.InlineRun{{Text: "Reduced cost by [metric:20%]"}},
		FactIDs: []string{"fact-1"},
	}
	err2 := validator.ValidateBullet(&b2)
	if err2 == nil {
		t.Fatal("expected error on bullet with placeholder in inline run, got nil")
	}
	if !errors.Is(err2, domain.ErrUnverifiedFactOrPlaceholder) {
		t.Errorf("expected ErrUnverifiedFactOrPlaceholder, got %v", err2)
	}
}

func TestValidateBullet_Ungrounded(t *testing.T) {
	resolver := newMockResolver()
	validator := render.NewDocumentValidator(resolver)

	// Case 1: Zero fact IDs
	b1 := render.NewBullet("b1", "Authored design document", []string{}, true)
	err1 := validator.ValidateBullet(&b1)
	if err1 == nil {
		t.Fatal("expected error on ungrounded bullet with 0 fact IDs")
	}
	if !errors.Is(err1, domain.ErrUngroundedBullet) {
		t.Errorf("expected ErrUngroundedBullet, got %v", err1)
	}

	// Case 2: Whitespace-only fact IDs
	b2 := render.NewBullet("b2", "Authored design document", []string{"  ", ""}, true)
	err2 := validator.ValidateBullet(&b2)
	if err2 == nil {
		t.Fatal("expected error on bullet with empty/whitespace fact IDs")
	}
	if !errors.Is(err2, domain.ErrUngroundedBullet) {
		t.Errorf("expected ErrUngroundedBullet, got %v", err2)
	}

	// Case 3: Fact ID not in vault
	b3 := render.NewBullet("b3", "Authored design document", []string{"fact-missing-404"}, true)
	err3 := validator.ValidateBullet(&b3)
	if err3 == nil {
		t.Fatal("expected error on bullet with non-existent fact ID")
	}
	if !errors.Is(err3, domain.ErrUngroundedBullet) {
		t.Errorf("expected ErrUngroundedBullet, got %v", err3)
	}
}

func TestValidateBullet_UnverifiedFact(t *testing.T) {
	resolver := newMockResolver()

	// Fact exists but is marked Verified = false
	unverifiedFact := &domain.CareerFact{
		ID:             "fact-unverified",
		Headline:       "Draft accomplishment",
		EpistemicState: domain.EpistemicVerified,
		Verified:       false,
	}
	resolver.addFact(unverifiedFact)

	// Fact exists and Verified = true, but epistemic state is metric_pending
	metricPendingFact := &domain.CareerFact{
		ID:             "fact-pending",
		Headline:       "Metric pending accomplishment",
		EpistemicState: domain.EpistemicMetricPending,
		Verified:       true,
	}
	resolver.addFact(metricPendingFact)

	// Fact exists and Verified = true, but epistemic state is raw_note
	rawNoteFact := &domain.CareerFact{
		ID:             "fact-raw",
		Headline:       "Raw note",
		EpistemicState: domain.EpistemicRawNote,
		Verified:       true,
	}
	resolver.addFact(rawNoteFact)

	validator := render.NewDocumentValidator(resolver)

	cases := []struct {
		name   string
		factID string
	}{
		{"verified=false", "fact-unverified"},
		{"epistemic_state=metric_pending", "fact-pending"},
		{"epistemic_state=raw_note", "fact-raw"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := render.NewBullet("b-test", "Valid bullet description", []string{tc.factID}, true)
			err := validator.ValidateBullet(&b)
			if err == nil {
				t.Fatalf("expected validation failure for %s, got nil", tc.name)
			}
			if !errors.Is(err, domain.ErrUnverifiedFactOrPlaceholder) {
				t.Errorf("expected ErrUnverifiedFactOrPlaceholder, got %v", err)
			}
		})
	}
}

func TestValidateBullet_VerifiedSuccess(t *testing.T) {
	resolver := newMockResolver()
	fact := &domain.CareerFact{
		ID:             "fact-verified-100",
		Headline:       "Scaled messaging cluster to 1M rps",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	}
	resolver.addFact(fact)

	validator := render.NewDocumentValidator(resolver)

	b := render.NewBullet("b-success", "Scaled **Kafka** cluster to `1,000,000` msgs/sec", []string{"fact-verified-100"}, true)
	if err := validator.ValidateBullet(&b); err != nil {
		t.Fatalf("expected successful validation for verified bullet, got %v", err)
	}
}

func TestValidateDocument_MultiErrorAggregation(t *testing.T) {
	resolver := newMockResolver()
	// Add one valid verified fact
	resolver.addFact(&domain.CareerFact{
		ID:             "fact-good",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	})

	validator := render.NewDocumentValidator(resolver)

	doc := render.NewResumeDocument()
	doc.Header.FullName = "Barbara Liskov"
	doc.Header.Headline = "Architect of [METRIC:systems] at scale" // Placeholder violation 1

	doc.Summary = &render.SummarySection{
		Text: "Delivered [METRIC:efficiency] across cloud systems.", // Placeholder violation 2
	}

	doc.Experience = []render.ExperienceItem{
		{
			RoleTitle:    "Lead Architect",
			Organization: "MIT",
			Highlights: []render.Bullet{
				render.NewBullet("b1", "Unresolved metric [METRIC:speed]", []string{"fact-good"}, true), // Placeholder violation 3
				render.NewBullet("b2", "Missing fact citation", []string{}, true),                       // Ungrounded violation 1
				render.NewBullet("b3", "Cites non-existent fact", []string{"fact-ghost"}, true),         // Ungrounded violation 2
			},
		},
	}

	err := validator.ValidateDocument(doc)
	if err == nil {
		t.Fatal("expected multi-error validation failure, got nil")
	}

	// Verify both sentinels are wrapped and detectable
	if !errors.Is(err, domain.ErrUnverifiedFactOrPlaceholder) {
		t.Error("expected report to wrap ErrUnverifiedFactOrPlaceholder")
	}
	if !errors.Is(err, domain.ErrUngroundedBullet) {
		t.Error("expected report to wrap ErrUngroundedBullet")
	}

	var report *render.ValidationReport
	if !errors.As(err, &report) {
		t.Fatalf("expected error to cast to *ValidationReport, got %T", err)
	}

	if len(report.Errors) != 5 {
		t.Errorf("expected 5 distinct validation errors, got %d:\n%s", len(report.Errors), report.Error())
	}
}

func TestValidateReadyToRender(t *testing.T) {
	resolver := newMockResolver()
	resolver.addFact(&domain.CareerFact{
		ID:             "fact-render-1",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	})

	validator := render.NewDocumentValidator(resolver)

	t.Run("rejects nil document", func(t *testing.T) {
		err := validator.ValidateReadyToRender(nil)
		if err == nil {
			t.Error("expected error for nil document")
		}
	})

	t.Run("rejects empty candidate name", func(t *testing.T) {
		doc := render.NewResumeDocument()
		doc.Experience = []render.ExperienceItem{
			{
				RoleTitle:    "Staff Engineer",
				Organization: "Google",
				Highlights: []render.Bullet{
					render.NewBullet("b1", "Engineered distributed store", []string{"fact-render-1"}, true),
				},
			},
		}
		err := validator.ValidateReadyToRender(doc)
		if err == nil {
			t.Error("expected error for missing full_name in header")
		}
	})

	t.Run("rejects empty sections", func(t *testing.T) {
		doc := render.NewResumeDocument()
		doc.Header.FullName = "Leslie Lamport"
		err := validator.ValidateReadyToRender(doc)
		if err == nil {
			t.Error("expected error for empty resume body")
		}
	})

	t.Run("accepts fully verified document", func(t *testing.T) {
		doc := render.NewResumeDocument()
		doc.Header.FullName = "Leslie Lamport"
		doc.Header.Headline = "Turing Award Laureate & Distributed Systems Pioneer"
		doc.Experience = []render.ExperienceItem{
			{
				RoleTitle:    "Principal Researcher",
				Organization: "Microsoft Research",
				Highlights: []render.Bullet{
					render.NewBullet("b1", "Authored **Paxos** consensus algorithm", []string{"fact-render-1"}, true),
				},
			},
		}

		if err := validator.ValidateReadyToRender(doc); err != nil {
			t.Fatalf("expected successful pre-render validation, got: %v", err)
		}
	})
}
