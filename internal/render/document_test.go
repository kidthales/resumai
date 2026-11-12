package render_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/render"
)

func TestNewResumeDocument(t *testing.T) {
	doc := render.NewResumeDocument()
	if doc == nil {
		t.Fatal("expected non-nil ResumeDocument")
	}
	if doc.Metadata.PageBudget != 1 {
		t.Errorf("expected default PageBudget=1, got %d", doc.Metadata.PageBudget)
	}
	if doc.Experience == nil || doc.Education == nil || doc.Skills == nil {
		t.Error("expected non-nil slices for experience, education, and skills")
	}
}

func TestInjectPII(t *testing.T) {
	doc := render.NewResumeDocument()
	doc.Header.Headline = "Principal Distributed Systems Engineer"

	pii := &domain.PIIHeader{
		FullName:     "Ada Lovelace",
		Email:        "ada@analytical.engine",
		Phone:        "+1-555-0199",
		Location:     "London, UK",
		LinkedInURL:  "https://linkedin.com/in/adalovelace",
		GitHubURL:    "https://github.com/adalovelace",
		PortfolioURL: "https://adalovelace.dev",
		UpdatedAt:    time.Now().UTC(),
	}

	// Verify header is clean before injection
	if doc.Header.FullName != "" || doc.Header.Email != "" {
		t.Fatal("expected empty PII fields before injection")
	}

	doc.InjectPII(pii)

	if doc.Header.FullName != "Ada Lovelace" {
		t.Errorf("expected FullName 'Ada Lovelace', got %q", doc.Header.FullName)
	}
	if doc.Header.Email != "ada@analytical.engine" {
		t.Errorf("expected Email 'ada@analytical.engine', got %q", doc.Header.Email)
	}
	if doc.Header.Phone != "+1-555-0199" {
		t.Errorf("expected Phone '+1-555-0199', got %q", doc.Header.Phone)
	}
	if doc.Header.Location != "London, UK" {
		t.Errorf("expected Location 'London, UK', got %q", doc.Header.Location)
	}
	if doc.Header.LinkedInURL != "https://linkedin.com/in/adalovelace" {
		t.Errorf("expected LinkedInURL 'https://linkedin.com/in/adalovelace', got %q", doc.Header.LinkedInURL)
	}
	if doc.Header.Headline != "Principal Distributed Systems Engineer" {
		t.Errorf("expected Headline to remain unchanged, got %q", doc.Header.Headline)
	}

	// Nil injection should safely no-op
	doc.InjectPII(nil)
	if doc.Header.FullName != "Ada Lovelace" {
		t.Error("nil InjectPII altered document state")
	}
}

func TestAllBullets(t *testing.T) {
	doc := render.NewResumeDocument()

	doc.Experience = []render.ExperienceItem{
		{
			RoleTitle:    "Staff Engineer",
			Organization: "Acme Corp",
			Highlights: []render.Bullet{
				{ID: "exp-1", Text: "Architected event mesh", FactIDs: []string{"fact-1"}, Verified: true},
				{ID: "exp-2", Text: "Reduced latency", FactIDs: []string{"fact-2"}, Verified: true},
			},
		},
	}

	doc.Education = []render.EducationItem{
		{
			Institution: "MIT",
			Degree:      "B.S. CS",
			Highlights: []render.Bullet{
				{ID: "edu-1", Text: "Published paper on Paxos", FactIDs: []string{"fact-3"}, Verified: true},
			},
		},
	}

	doc.Projects = []render.ProjectItem{
		{
			Name: "ResumAI",
			Highlights: []render.Bullet{
				{ID: "proj-1", Text: "Built pure-Go engine", FactIDs: []string{"fact-4"}, Verified: true},
			},
		},
	}

	doc.CustomSections = []render.CustomSection{
		{
			Title: "Open Source",
			Highlights: []render.Bullet{
				{ID: "cust-1", Text: "Maintainer of sqlite driver", FactIDs: []string{"fact-5"}, Verified: true},
			},
		},
	}

	bullets := doc.AllBullets()
	if len(bullets) != 5 {
		t.Fatalf("expected 5 bullets across sections, got %d", len(bullets))
	}

	expectedIDs := []string{"exp-1", "exp-2", "edu-1", "proj-1", "cust-1"}
	for i, b := range bullets {
		if b.ID != expectedIDs[i] {
			t.Errorf("bullet %d ID mismatch: expected %q, got %q", i, expectedIDs[i], b.ID)
		}
	}
}

func TestCloneAndJSONRoundtrip(t *testing.T) {
	doc := render.NewResumeDocument()
	doc.Header.FullName = "Claude Shannon"
	doc.Experience = []render.ExperienceItem{
		{
			RoleTitle:    "Research Mathematician",
			Organization: "Bell Labs",
			Highlights: []render.Bullet{
				render.NewBullet("b1", "Founded **information theory**", []string{"f1"}, true),
			},
		},
	}

	clone, err := doc.Clone()
	if err != nil {
		t.Fatalf("failed to clone document: %v", err)
	}

	if clone.Header.FullName != doc.Header.FullName {
		t.Errorf("cloned FullName mismatch: got %q, expected %q", clone.Header.FullName, doc.Header.FullName)
	}

	// Mutate clone to ensure deep copy
	clone.Header.FullName = "Alan Turing"
	if doc.Header.FullName == "Alan Turing" {
		t.Error("expected mutation of clone to not affect original document")
	}

	// JSON roundtrip
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("failed to marshal doc: %v", err)
	}

	var roundtrip render.ResumeDocument
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal doc: %v", err)
	}

	if roundtrip.Header.FullName != "Claude Shannon" {
		t.Errorf("roundtrip FullName mismatch: got %q", roundtrip.Header.FullName)
	}
}

func TestNewBulletFromNode(t *testing.T) {
	node := domain.BulletNode{
		ID:       "node-42",
		Text:     "Optimized **Go** runtime with `PGO`",
		FactIDs:  []string{"fact-100"},
		Verified: true,
	}

	bullet := render.NewBulletFromNode(node)
	if bullet.ID != "node-42" {
		t.Errorf("expected ID 'node-42', got %q", bullet.ID)
	}
	if bullet.Text != node.Text {
		t.Errorf("expected Text %q, got %q", node.Text, bullet.Text)
	}
	if len(bullet.FactIDs) != 1 || bullet.FactIDs[0] != "fact-100" {
		t.Errorf("expected FactIDs ['fact-100'], got %v", bullet.FactIDs)
	}
	if !bullet.Verified {
		t.Error("expected bullet to be verified")
	}
	if len(bullet.Runs) == 0 {
		t.Error("expected parsed runs in bullet, got 0")
	}
}
