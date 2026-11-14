-- Migration 0002: Persist JobApplication FSM Artifacts & Submission Lock Fields

ALTER TABLE applications ADD COLUMN dossier_json TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN draft_json TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN audits_json TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN export_json TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN bundle_sha256 TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN interview_brief_path TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN submitted_at TIMESTAMP;
