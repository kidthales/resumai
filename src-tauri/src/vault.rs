use anyhow::{bail, Context, Result};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};

pub const PII_CATEGORIES: [&str; 4] = [
    "personal_info",
    "education_history",
    "work_history",
    "extras",
];

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PiiEntry {
    pub category: String,
    pub active_filename: String,
    pub custom_filename: String,
    pub sample_filename: String,
    pub is_custom: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArchetypeEntry {
    pub filename: String,
    pub is_sample: bool,
    pub is_active_in_selector: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ApplicationRunSummary {
    pub slug: String,
    pub created_at: String,
    pub title: String,
    pub archetype_filename: Option<String>,
    pub match_score: Option<u32>,
    pub status: String,
    pub has_final_resume: bool,
    pub has_cover_letter: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VaultOverview {
    pub root_path: String,
    pub pii_files: Vec<PiiEntry>,
    pub archetype_files: Vec<ArchetypeEntry>,
    pub prompt_files: Vec<String>,
    pub style_files: Vec<String>,
    pub applications: Vec<ApplicationRunSummary>,
}

#[derive(Debug, Clone)]
pub struct VaultManager {
    root_dir: PathBuf,
}

impl VaultManager {
    pub fn discover() -> Result<Self> {
        let cwd = std::env::current_dir().context("Failed to determine current directory")?;
        if cwd.join("content").join("prompts").is_dir() {
            return Ok(Self { root_dir: cwd });
        }
        if let Some(parent) = cwd.parent() {
            if parent.join("content").join("prompts").is_dir() {
                return Ok(Self {
                    root_dir: parent.to_path_buf(),
                });
            }
        }
        Ok(Self { root_dir: cwd })
    }

    pub fn with_root(root_dir: impl Into<PathBuf>) -> Self {
        Self {
            root_dir: root_dir.into(),
        }
    }

    pub fn root_dir(&self) -> &Path {
        &self.root_dir
    }

    pub fn pii_dir(&self) -> PathBuf {
        self.root_dir.join("content").join("pii")
    }

    pub fn archetypes_dir(&self) -> PathBuf {
        self.root_dir.join("content").join("archetypes")
    }

    pub fn prompts_dir(&self) -> PathBuf {
        self.root_dir.join("content").join("prompts")
    }

    pub fn styles_dir(&self) -> PathBuf {
        self.root_dir.join("assets").join("styles").join("resumes")
    }

    pub fn applications_dir(&self) -> PathBuf {
        self.root_dir.join("var").join("applications")
    }

    pub fn validate_safe_filename(filename: &str) -> Result<String> {
        let trimmed = filename.trim();
        if trimmed.is_empty() {
            bail!("Filename cannot be empty.");
        }
        if trimmed.contains("..")
            || trimmed.contains('/')
            || trimmed.contains('\\')
            || trimmed.contains('\0')
        {
            bail!("Invalid filename \"{}\": path traversal characters are not allowed.", filename);
        }
        Ok(trimmed.to_string())
    }

    pub fn get_overview(&self) -> Result<VaultOverview> {
        let mut pii_files = Vec::new();
        for cat in PII_CATEGORIES {
            let custom_filename = format!("{cat}.md");
            let sample_filename = format!("{cat}.sample.md");
            let custom_path = self.pii_dir().join(&custom_filename);
            let is_custom = custom_path.is_file()
                && fs::read_to_string(&custom_path)
                    .map(|s| !s.trim().is_empty())
                    .unwrap_or(false);
            let active_filename = if is_custom {
                custom_filename.clone()
            } else {
                sample_filename.clone()
            };
            pii_files.push(PiiEntry {
                category: cat.to_string(),
                active_filename,
                custom_filename,
                sample_filename,
                is_custom,
            });
        }

        let all_archetypes = self.list_all_archetype_filenames()?;
        let has_non_sample = all_archetypes.iter().any(|f| !f.ends_with(".sample.md"));
        let archetype_files = all_archetypes
            .into_iter()
            .map(|filename| {
                let is_sample = filename.ends_with(".sample.md");
                let is_active_in_selector = if has_non_sample { !is_sample } else { true };
                ArchetypeEntry {
                    filename,
                    is_sample,
                    is_active_in_selector,
                }
            })
            .collect();

        let prompt_files = Self::list_dir_files(&self.prompts_dir(), ".md")?;
        let style_files = Self::list_dir_files(&self.styles_dir(), ".css")?;
        let applications = self.list_applications()?;

        Ok(VaultOverview {
            root_path: self.root_dir.to_string_lossy().to_string(),
            pii_files,
            archetype_files,
            prompt_files,
            style_files,
            applications,
        })
    }

    fn list_dir_files(dir: &Path, extension: &str) -> Result<Vec<String>> {
        if !dir.is_dir() {
            return Ok(Vec::new());
        }
        let mut names = Vec::new();
        for entry in fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();
            if path.is_file() {
                if let Some(name) = path.file_name().and_then(|n| n.to_str()) {
                    if !name.starts_with('.') && name.ends_with(extension) {
                        names.push(name.to_string());
                    }
                }
            }
        }
        names.sort();
        Ok(names)
    }

    pub fn list_all_archetype_filenames(&self) -> Result<Vec<String>> {
        Self::list_dir_files(&self.archetypes_dir(), ".md")
    }

    /// Mirrors v1 `ArchetypeDirectoryTool::listArchetypes()`:
    /// If at least one non-sample `.md` file exists, all `.sample.md` files are excluded.
    pub fn list_active_archetypes(&self) -> Result<Vec<String>> {
        let all = self.list_all_archetype_filenames()?;
        let mut samples = Vec::new();
        let mut non_samples = Vec::new();
        for f in all {
            if f.ends_with(".sample.md") {
                samples.push(f);
            } else {
                non_samples.push(f);
            }
        }
        if !non_samples.is_empty() {
            Ok(non_samples)
        } else {
            Ok(samples)
        }
    }

    pub fn read_archetype_file(&self, filename: &str) -> Result<String> {
        let clean = Self::validate_safe_filename(filename)?;
        let candidate = if clean.ends_with(".md") {
            clean
        } else {
            format!("{clean}.md")
        };
        let path = self.archetypes_dir().join(&candidate);
        if !path.is_file() {
            bail!("Archetype file \"{}\" does not exist.", candidate);
        }
        fs::read_to_string(&path).with_context(|| format!("Failed to read archetype {}", candidate))
    }

    pub fn read_pii_category(&self, category: &str) -> Result<String> {
        let clean = Self::validate_safe_filename(category)?;
        let custom_path = self.pii_dir().join(format!("{clean}.md"));
        if custom_path.is_file() {
            let content = fs::read_to_string(&custom_path)?;
            if !content.trim().is_empty() {
                return Ok(content.trim().to_string());
            }
        }
        let sample_path = self.pii_dir().join(format!("{clean}.sample.md"));
        if sample_path.is_file() {
            let content = fs::read_to_string(&sample_path)?;
            return Ok(content.trim().to_string());
        }
        Ok("content_not_found".to_string())
    }

    /// Compiles a system prompt by reading `content/prompts/<prompt_name>.md` and safely
    /// replacing `%pii.*%` placeholders in a single pass so literal `%` signs inside PII
    /// (e.g. "45% latency reduction") never cause recursive parameter errors.
    pub fn compile_prompt(&self, prompt_name: &str) -> Result<String> {
        let clean = Self::validate_safe_filename(prompt_name)?;
        let filename = if clean.ends_with(".md") {
            clean
        } else {
            format!("{clean}.md")
        };
        let prompt_path = self.prompts_dir().join(&filename);
        let raw_prompt = fs::read_to_string(&prompt_path)
            .with_context(|| format!("Failed to read prompt file {}", filename))?;

        let personal_info = self.read_pii_category("personal_info")?;
        let education_history = self.read_pii_category("education_history")?;
        let work_history = self.read_pii_category("work_history")?;
        let extras = self.read_pii_category("extras")?;

        let compiled = raw_prompt
            .replace("%pii.personal_info%", &personal_info)
            .replace("%pii.education_history%", &education_history)
            .replace("%pii.work_history%", &work_history)
            .replace("%pii.extras%", &extras);

        Ok(compiled)
    }

    pub fn read_vault_file(&self, category: &str, filename: &str) -> Result<String> {
        let clean_file = Self::validate_safe_filename(filename)?;
        let base_dir = self.category_dir(category)?;
        let target = base_dir.join(&clean_file);
        if !target.is_file() {
            bail!("File \"{}\" not found in category \"{}\".", clean_file, category);
        }
        fs::read_to_string(&target).context("Failed to read vault file")
    }

    pub fn write_vault_file(&self, category: &str, filename: &str, content: &str) -> Result<String> {
        let clean_file = Self::validate_safe_filename(filename)?;
        let base_dir = self.category_dir(category)?;
        fs::create_dir_all(&base_dir)?;
        let target = base_dir.join(&clean_file);
        fs::write(&target, content).context("Failed to write vault file")?;
        Ok(target.to_string_lossy().to_string())
    }

    /// Knowledge-Base Write-Back Loop: appends a newly verified metric/bullet directly
    /// to `content/pii/work_history.md` (initializing from `work_history.sample.md` if needed).
    pub fn append_fact_to_work_history(&self, fact_bullet: &str, role_context: Option<&str>) -> Result<String> {
        let trimmed = fact_bullet.trim();
        if trimmed.is_empty() {
            bail!("Fact bullet cannot be empty.");
        }
        fs::create_dir_all(self.pii_dir())?;
        let custom_path = self.pii_dir().join("work_history.md");
        let existing = if custom_path.is_file() {
            fs::read_to_string(&custom_path)?
        } else {
            self.read_pii_category("work_history")?
        };

        let formatted_bullet = if trimmed.starts_with('-') || trimmed.starts_with('*') {
            trimmed.to_string()
        } else {
            format!("- {trimmed}")
        };

        let entry = match role_context.map(str::trim).filter(|s| !s.is_empty()) {
            Some(ctx) => format!("\n\n<!-- Added via ResumAI Knowledge Write-Back ({ctx}) -->\n{formatted_bullet}\n"),
            None => format!("\n{formatted_bullet}\n"),
        };

        let updated = format!("{}{}", existing.trim_end(), entry);
        fs::write(&custom_path, &updated)?;
        Ok(custom_path.to_string_lossy().to_string())
    }

    pub fn read_style_file(&self, filename: Option<&str>) -> Result<String> {
        let target_name = match filename.map(str::trim).filter(|s| !s.is_empty()) {
            Some(name) => Self::validate_safe_filename(name)?,
            None => "resume.sample.css".to_string(),
        };
        let path = self.styles_dir().join(&target_name);
        if path.is_file() {
            return fs::read_to_string(&path).context("Failed to read CSS file");
        }
        let fallback = self.styles_dir().join("resume.sample.css");
        fs::read_to_string(&fallback).context("Failed to read fallback resume.sample.css")
    }

    fn category_dir(&self, category: &str) -> Result<PathBuf> {
        match category.trim() {
            "pii" => Ok(self.pii_dir()),
            "archetypes" => Ok(self.archetypes_dir()),
            "prompts" => Ok(self.prompts_dir()),
            "styles" => Ok(self.styles_dir()),
            other => bail!("Unknown vault category \"{}\".", other),
        }
    }

    pub fn list_applications(&self) -> Result<Vec<ApplicationRunSummary>> {
        let apps_dir = self.applications_dir();
        if !apps_dir.is_dir() {
            return Ok(Vec::new());
        }
        let mut list = Vec::new();
        for entry in fs::read_dir(&apps_dir)? {
            let entry = entry?;
            let path = entry.path();
            if path.is_dir() {
                let manifest_path = path.join("manifest.json");
                if manifest_path.is_file() {
                    if let Ok(raw) = fs::read_to_string(&manifest_path) {
                        if let Ok(summary) = serde_json::from_str::<ApplicationRunSummary>(&raw) {
                            list.push(summary);
                            continue;
                        }
                    }
                }
                if let Some(slug) = path.file_name().and_then(|s| s.to_str()) {
                    list.push(ApplicationRunSummary {
                        slug: slug.to_string(),
                        created_at: String::new(),
                        title: slug.to_string(),
                        archetype_filename: None,
                        match_score: None,
                        status: "Drafted".to_string(),
                        has_final_resume: path.join("resume_final.md").is_file(),
                        has_cover_letter: path.join("cover_letter.md").is_file(),
                    });
                }
            }
        }
        list.sort_by(|a, b| b.slug.cmp(&a.slug));
        Ok(list)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn create_temp_vault() -> PathBuf {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let dir = std::env::temp_dir().join(format!("resumai_test_vault_{nanos}"));
        fs::create_dir_all(dir.join("content").join("pii")).unwrap();
        fs::create_dir_all(dir.join("content").join("archetypes")).unwrap();
        fs::create_dir_all(dir.join("content").join("prompts")).unwrap();
        fs::create_dir_all(dir.join("assets").join("styles").join("resumes")).unwrap();
        dir
    }

    #[test]
    fn test_prompt_interpolation_preserves_percent_signs_in_pii() {
        let dir = create_temp_vault();
        let vault = VaultManager::with_root(&dir);

        fs::write(
            vault.pii_dir().join("personal_info.sample.md"),
            "Name: Jane Doe (Top 1% Engineer)",
        )
        .unwrap();
        fs::write(
            vault.pii_dir().join("work_history.md"),
            "- Reduced p99 latency by 45% and lowered cloud spend by 30% across 100% of regions.",
        )
        .unwrap();
        fs::write(
            vault.pii_dir().join("education_history.sample.md"),
            "B.Sc. Computer Science",
        )
        .unwrap();
        fs::write(vault.pii_dir().join("extras.sample.md"), "Open Source").unwrap();

        fs::write(
            vault.prompts_dir().join("resume_drafter.md"),
            "Profile:\n%pii.personal_info%\nWork:\n%pii.work_history%\nEdu:\n%pii.education_history%\nExtras:\n%pii.extras%",
        )
        .unwrap();

        let compiled = vault.compile_prompt("resume_drafter").unwrap();
        assert!(compiled.contains("Top 1% Engineer"));
        assert!(compiled.contains("Reduced p99 latency by 45% and lowered cloud spend by 30%"));
        assert!(!compiled.contains("%pii."));

        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_archetype_sample_exclusion_rule() {
        let dir = create_temp_vault();
        let vault = VaultManager::with_root(&dir);

        fs::write(
            vault.archetypes_dir().join("senior_backend.sample.md"),
            "# Sample",
        )
        .unwrap();
        fs::write(
            vault.archetypes_dir().join("staff_backend.sample.md"),
            "# Sample 2",
        )
        .unwrap();

        // Only samples exist -> returns samples
        let active_initial = vault.list_active_archetypes().unwrap();
        assert_eq!(active_initial.len(), 2);

        // Add one custom archetype -> all .sample.md files are excluded
        fs::write(
            vault.archetypes_dir().join("game_studio_architect.md"),
            "# Custom Archetype",
        )
        .unwrap();

        let active_after = vault.list_active_archetypes().unwrap();
        assert_eq!(active_after, vec!["game_studio_architect.md".to_string()]);

        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_path_traversal_rejected() {
        assert!(VaultManager::validate_safe_filename("../secret.md").is_err());
        assert!(VaultManager::validate_safe_filename("sub/file.md").is_err());
        assert!(VaultManager::validate_safe_filename("sub\\file.md").is_err());
        assert!(VaultManager::validate_safe_filename("valid_file.md").is_ok());
    }
}
