-- Migration 0001: Initial CareerVault Schema

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Air-Gapped Contact PII (CLI-only, never exposed over MCP)
CREATE TABLE IF NOT EXISTS pii_header (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    full_name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    linkedin_url TEXT NOT NULL DEFAULT '',
    github_url TEXT NOT NULL DEFAULT '',
    portfolio_url TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Candidate Logistics & Preferences (Non-PII, safe for planning)
CREATE TABLE IF NOT EXISTS candidate_logistics (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    target_title TEXT NOT NULL DEFAULT '',
    work_authorization TEXT NOT NULL DEFAULT '',
    remote_preference TEXT NOT NULL DEFAULT '',
    willing_to_relocate INTEGER NOT NULL DEFAULT 0,
    target_compensation TEXT NOT NULL DEFAULT '',
    notice_period_days INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Organizations (Companies, Institutions, Non-profits)
CREATE TABLE IF NOT EXISTS organizations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    location TEXT NOT NULL DEFAULT '',
    website TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Roles per Organization
CREATE TABLE IF NOT EXISTS roles (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    title TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'work',
    location TEXT NOT NULL DEFAULT '',
    duration TEXT NOT NULL DEFAULT '',
    tech_stack TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_roles_org_id ON roles(org_id);
CREATE INDEX IF NOT EXISTS idx_roles_category ON roles(category);

-- Career Facts (Ground truth accomplishment atoms with epistemic states)
CREATE TABLE IF NOT EXISTS career_facts (
    id TEXT PRIMARY KEY,
    role_id TEXT NOT NULL,
    headline TEXT NOT NULL DEFAULT '',
    details TEXT NOT NULL,
    epistemic_state TEXT NOT NULL DEFAULT 'raw_note',
    tech_tags TEXT NOT NULL DEFAULT '[]',
    verified INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_career_facts_role_id ON career_facts(role_id);
CREATE INDEX IF NOT EXISTS idx_career_facts_epistemic ON career_facts(epistemic_state);

-- Education & Credentials
CREATE TABLE IF NOT EXISTS education (
    id TEXT PRIMARY KEY,
    institution TEXT NOT NULL,
    degree TEXT NOT NULL DEFAULT '',
    field_of_study TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    duration TEXT NOT NULL DEFAULT '',
    honors TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Resume Archetypes (Personas)
CREATE TABLE IF NOT EXISTS archetypes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    profile_overview TEXT NOT NULL DEFAULT '',
    core_competencies TEXT NOT NULL DEFAULT '[]',
    strategic_impact TEXT NOT NULL DEFAULT '[]',
    target_roles TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Job Applications
CREATE TABLE IF NOT EXISTS applications (
    id TEXT PRIMARY KEY,
    role_title TEXT NOT NULL,
    company TEXT NOT NULL,
    raw_jd TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'Triaged',
    rejection_rationale TEXT NOT NULL DEFAULT '',
    archetype_id TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (archetype_id) REFERENCES archetypes(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_applications_state ON applications(state);

-- Application Citations (Fact citations cited in applications)
CREATE TABLE IF NOT EXISTS application_citations (
    application_id TEXT NOT NULL,
    fact_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (application_id, fact_id),
    FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE,
    FOREIGN KEY (fact_id) REFERENCES career_facts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_citations_fact_id ON application_citations(fact_id);
