use crate::export::export_resume_bundle;
use crate::llm::{
    call_agent_streaming, call_archetype_selector_with_tools, emit_progress, AgentStage, LlmConfig,
};
use crate::vault::{ApplicationRunSummary, VaultManager};
use anyhow::{bail, Context, Result};
use chrono::Local;
use regex::Regex;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::fs;
use tauri::AppHandle;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ArchetypeSelectionResult {
    pub archetype_filename: String,
    pub rationale: String,
    pub thinking: String,
    pub model_used: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StageArtifactResult {
    pub markdown: String,
    pub thinking: String,
    pub model_used: String,
    pub metric_placeholders: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AlignmentCheckResult {
    pub markdown: String,
    pub thinking: String,
    pub model_used: String,
    pub match_score: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ParallelChecksResult {
    pub fact_check: StageArtifactResult,
    pub alignment_check: Option<AlignmentCheckResult>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FullPipelineResult {
    pub slug: String,
    pub output_dir: String,
    pub archetype: Option<ArchetypeSelectionResult>,
    pub draft: StageArtifactResult,
    pub fact_check: StageArtifactResult,
    pub alignment_check: Option<AlignmentCheckResult>,
    pub final_resume: StageArtifactResult,
    pub exported_html_path: String,
    pub exported_docx_path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoadedApplicationWorkspace {
    pub summary: ApplicationRunSummary,
    pub job_description: String,
    pub archetype_selection: Option<ArchetypeSelectionResult>,
    pub draft_markdown: String,
    pub fact_check_markdown: String,
    pub alignment_check_markdown: String,
    pub final_resume_markdown: String,
    pub cover_letter_markdown: String,
}

pub fn sanitize_markdown_fence(raw: &str) -> String {
    let trimmed = raw.trim();
    let re = Regex::new(r"(?s)^```(?:[a-zA-Z0-9_-]+)?\s*\r?\n?(.*?)\r?\n?```$").unwrap();
    if let Some(caps) = re.captures(trimmed) {
        if let Some(inner) = caps.get(1) {
            return inner.as_str().trim().to_string();
        }
    }
    trimmed.to_string()
}

pub fn parse_archetype_selection_json(raw: &str) -> Result<(String, String)> {
    let sanitized = sanitize_markdown_fence(raw);
    let candidate_json = if let (Some(start), Some(end)) = (sanitized.find('{'), sanitized.rfind('}'))
    {
        if end > start {
            &sanitized[start..=end]
        } else {
            sanitized.as_str()
        }
    } else {
        sanitized.as_str()
    };

    let parsed: Value = serde_json::from_str(candidate_json).with_context(|| {
        format!("Failed to parse resume_archetype_selector result as JSON. Output: {raw}")
    })?;

    let filename = parsed
        .get("archetype_filename")
        .or_else(|| parsed.get("archetypeFilename"))
        .and_then(|v| v.as_str())
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .context("Missing or invalid \"archetype_filename\" in archetype selector JSON.")?;

    let rationale = parsed
        .get("rationale")
        .and_then(|v| v.as_str())
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .context("Missing or invalid \"rationale\" in archetype selector JSON.")?;

    Ok((filename.to_string(), rationale.to_string()))
}

pub fn extract_match_score(alignment_markdown: &str) -> Option<u32> {
    let re = Regex::new(r"(?i)Overall\s+Match\s+Score\s*:\**\s*\[?(\d{1,3})\]?\s*%").unwrap();
    if let Some(caps) = re.captures(alignment_markdown) {
        if let Some(m) = caps.get(1) {
            if let Ok(score) = m.as_str().parse::<u32>() {
                return Some(score.min(100));
            }
        }
    }
    None
}

pub fn extract_metric_placeholders(markdown: &str) -> Vec<String> {
    let re = Regex::new(r"\[METRIC:[^\]]+\]").unwrap();
    let mut list = Vec::new();
    for m in re.find_iter(markdown) {
        let token = m.as_str().to_string();
        if !list.contains(&token) {
            list.push(token);
        }
    }
    list
}

pub async fn select_archetype(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    job_description: &str,
) -> Result<ArchetypeSelectionResult> {
    let trimmed_jd = job_description.trim();
    if trimmed_jd.is_empty() {
        bail!("Job description cannot be empty for archetype selection.");
    }

    let system_prompt = vault.compile_prompt("resume_archetype_selector")?;
    let user_input = format!(
        "Determine the best matching candidate archetype for the following job description:\n\n<job_description>\n{trimmed_jd}\n</job_description>\n\nOutput only the final JSON object. Do not include introductory text, explanations, or enclosing code block fences."
    );

    let out = call_archetype_selector_with_tools(app, vault, cfg, &system_prompt, &user_input).await?;
    let (archetype_filename, rationale) = parse_archetype_selection_json(&out.result)?;

    Ok(ArchetypeSelectionResult {
        archetype_filename,
        rationale,
        thinking: out.thinking,
        model_used: out.model_used,
    })
}

pub async fn draft_resume(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    job_description: Option<&str>,
    archetype_filename: Option<&str>,
) -> Result<StageArtifactResult> {
    let system_prompt = vault.compile_prompt("resume_drafter")?;
    let mut parts = vec![
        "Synthesize a complete, high-impact resume in Markdown format based on the candidate profile."
            .to_string(),
    ];

    if let Some(jd) = job_description.map(str::trim).filter(|s| !s.is_empty()) {
        parts.push(format!(
            "Target the resume specifically to the following Job Description:\n\n<job_description>\n{jd}\n</job_description>"
        ));
    }

    if let Some(arch_file) = archetype_filename.map(str::trim).filter(|s| !s.is_empty()) {
        let arch_content = vault.read_archetype_file(arch_file)?;
        parts.push(format!(
            "Position the candidate according to the following Candidate Archetype:\n\n<candidate_archetype>\n{}\n</candidate_archetype>",
            arch_content.trim()
        ));
    }

    parts.push(
        "Output only the final Markdown resume. Do not include introductory text, explanations, or enclosing code block fences."
            .to_string(),
    );

    let user_input = parts.join("\n\n");
    let out = call_agent_streaming(app, cfg, AgentStage::ResumeDrafter, &system_prompt, &user_input).await?;
    let markdown = sanitize_markdown_fence(&out.result);
    let metric_placeholders = extract_metric_placeholders(&markdown);

    Ok(StageArtifactResult {
        markdown,
        thinking: out.thinking,
        model_used: out.model_used,
        metric_placeholders,
    })
}

pub async fn fact_check_resume(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    resume_markdown: &str,
) -> Result<StageArtifactResult> {
    let trimmed_resume = resume_markdown.trim();
    if trimmed_resume.is_empty() {
        bail!("Resume markdown cannot be empty for fact-checking.");
    }

    let system_prompt = vault.compile_prompt("resume_fact_checker")?;
    let user_input = format!(
        "Perform a comprehensive fact-check and coherence audit on the following resume draft against the verified source facts:\n\n<candidate_resume>\n{trimmed_resume}\n</candidate_resume>\n\nOutput only the final Markdown audit report. Do not include introductory text, explanations, or enclosing code block fences."
    );

    let out = call_agent_streaming(app, cfg, AgentStage::FactChecker, &system_prompt, &user_input).await?;
    let markdown = sanitize_markdown_fence(&out.result);

    Ok(StageArtifactResult {
        markdown,
        thinking: out.thinking,
        model_used: out.model_used,
        metric_placeholders: Vec::new(),
    })
}

pub async fn job_alignment_check_resume(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    resume_markdown: &str,
    job_description: &str,
) -> Result<AlignmentCheckResult> {
    let trimmed_resume = resume_markdown.trim();
    let trimmed_jd = job_description.trim();
    if trimmed_resume.is_empty() {
        bail!("Resume markdown cannot be empty for job-alignment checking.");
    }
    if trimmed_jd.is_empty() {
        bail!("Job description cannot be empty for job-alignment checking.");
    }

    let system_prompt = vault.compile_prompt("resume_job_alignment_checker")?;
    let user_input = format!(
        "Perform a job-alignment-check and analysis for the following candidate resume and job description:\n\n<candidate_resume>\n{trimmed_resume}\n</candidate_resume>\n\n<job_description>\n{trimmed_jd}\n</job_description>\n\nOutput only the final Markdown executive summary. Do not include introductory text, explanations, or enclosing code block fences."
    );

    let out = call_agent_streaming(
        app,
        cfg,
        AgentStage::JobAlignmentChecker,
        &system_prompt,
        &user_input,
    )
    .await?;
    let markdown = sanitize_markdown_fence(&out.result);
    let match_score = extract_match_score(&markdown);

    Ok(AlignmentCheckResult {
        markdown,
        thinking: out.thinking,
        model_used: out.model_used,
        match_score,
    })
}

/// Runs Stage 3a (Fact-Checker) and Stage 3b (Job-Alignment Checker) concurrently via `tokio::join!`.
pub async fn run_parallel_checks(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    resume_markdown: &str,
    job_description: Option<&str>,
) -> Result<ParallelChecksResult> {
    let has_jd = job_description
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .is_some();

    if let Some(jd) = job_description.map(str::trim).filter(|s| !s.is_empty()) {
        let (fc_res, jac_res) = tokio::join!(
            fact_check_resume(app, vault, cfg, resume_markdown),
            job_alignment_check_resume(app, vault, cfg, resume_markdown, jd)
        );
        Ok(ParallelChecksResult {
            fact_check: fc_res?,
            alignment_check: Some(jac_res?),
        })
    } else {
        debug_assert!(!has_jd);
        let fc = fact_check_resume(app, vault, cfg, resume_markdown).await?;
        Ok(ParallelChecksResult {
            fact_check: fc,
            alignment_check: None,
        })
    }
}

pub async fn edit_resume(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    resume_markdown: &str,
    fact_check_report: &str,
    job_alignment_report: Option<&str>,
    user_notes_and_metrics: Option<&str>,
) -> Result<StageArtifactResult> {
    let trimmed_resume = resume_markdown.trim();
    let trimmed_fc = fact_check_report.trim();
    if trimmed_resume.is_empty() {
        bail!("Baseline resume cannot be empty for editing.");
    }
    if trimmed_fc.is_empty() {
        bail!("Fact-check report cannot be empty for editing.");
    }

    let system_prompt = vault.compile_prompt("resume_editor")?;
    let mut parts = vec![format!(
        "Synthesize an edited, factually accurate, high-impact resume in Markdown format based on the following baseline resume and audit reports:\n\n<candidate_resume>\n{trimmed_resume}\n</candidate_resume>\n\n<fact_check_report>\n{trimmed_fc}\n</fact_check_report>"
    )];

    if let Some(jac) = job_alignment_report.map(str::trim).filter(|s| !s.is_empty()) {
        parts.push(format!(
            "<job_alignment_report>\n{jac}\n</job_alignment_report>"
        ));
    }

    if let Some(notes) = user_notes_and_metrics.map(str::trim).filter(|s| !s.is_empty()) {
        parts.push(format!(
            "Apply the following explicit candidate metric values and editorial instructions:\n<user_notes_and_metrics>\n{notes}\n</user_notes_and_metrics>"
        ));
    }

    parts.push(
        "Output only the final edited Markdown resume. Do not include introductory text, explanations, or enclosing code block fences."
            .to_string(),
    );

    let user_input = parts.join("\n\n");
    let out = call_agent_streaming(app, cfg, AgentStage::ResumeEditor, &system_prompt, &user_input).await?;
    let markdown = sanitize_markdown_fence(&out.result);
    let metric_placeholders = extract_metric_placeholders(&markdown);

    Ok(StageArtifactResult {
        markdown,
        thinking: out.thinking,
        model_used: out.model_used,
        metric_placeholders,
    })
}

pub async fn generate_cover_letter(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    resume_markdown: &str,
    job_description: Option<&str>,
    archetype_filename: Option<&str>,
    mode: &str, // "cover_letter" | "consulting_pitch"
) -> Result<StageArtifactResult> {
    let system_prompt = vault.compile_prompt("resume_cover_letter")?;
    let doc_label = if mode.trim().eq_ignore_ascii_case("consulting_pitch") {
        "Consulting Proposal & Technical Capability Brief"
    } else {
        "Executive Cover Letter"
    };

    let mut parts = vec![format!(
        "Generate a high-impact **{doc_label}** in Markdown format grounded strictly in the candidate's verified profile and resume:\n\n<candidate_resume>\n{}\n</candidate_resume>",
        resume_markdown.trim()
    )];

    if let Some(jd) = job_description.map(str::trim).filter(|s| !s.is_empty()) {
        parts.push(format!(
            "Target the document specifically to the following Job Description / Client Brief:\n\n<job_description>\n{jd}\n</job_description>"
        ));
    }

    if let Some(arch) = archetype_filename.map(str::trim).filter(|s| !s.is_empty()) {
        if let Ok(arch_content) = vault.read_archetype_file(arch) {
            parts.push(format!(
                "<candidate_archetype>\n{}\n</candidate_archetype>",
                arch_content.trim()
            ));
        }
    }

    parts.push(
        "Output only the final Markdown document. Do not include conversational preamble or enclosing code block fences."
            .to_string(),
    );

    let user_input = parts.join("\n\n");
    let out = call_agent_streaming(app, cfg, AgentStage::CoverLetter, &system_prompt, &user_input).await?;
    let markdown = sanitize_markdown_fence(&out.result);

    Ok(StageArtifactResult {
        markdown,
        thinking: out.thinking,
        model_used: out.model_used,
        metric_placeholders: Vec::new(),
    })
}

#[allow(clippy::too_many_arguments)]
pub async fn run_full_pipeline(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    run_title: Option<&str>,
    job_description: Option<&str>,
    archetype_override: Option<&str>,
    style_filename: Option<&str>,
    user_notes: Option<&str>,
) -> Result<FullPipelineResult> {
    let trimmed_jd = job_description.map(str::trim).filter(|s| !s.is_empty());

    // Step 1: Select archetype (either override or auto-select if JD is provided)
    let archetype_res = match archetype_override.map(str::trim).filter(|s| !s.is_empty()) {
        Some(manual_arch) => Some(ArchetypeSelectionResult {
            archetype_filename: manual_arch.to_string(),
            rationale: "Manually selected by candidate.".to_string(),
            thinking: String::new(),
            model_used: "manual".to_string(),
        }),
        None => {
            if let Some(jd) = trimmed_jd {
                Some(select_archetype(app, vault, cfg, jd).await?)
            } else {
                None
            }
        }
    };

    let selected_arch_file = archetype_res
        .as_ref()
        .map(|a| a.archetype_filename.as_str());

    // Step 2: Draft Resume
    let draft = draft_resume(app, vault, cfg, trimmed_jd, selected_arch_file).await?;

    // Step 3a & 3b: Parallel Checks (Fact-Check + Job-Alignment Check)
    let checks = run_parallel_checks(app, vault, cfg, &draft.markdown, trimmed_jd).await?;

    // Step 4: Edit Resume
    let jac_md = checks
        .alignment_check
        .as_ref()
        .map(|a| a.markdown.as_str());
    let final_resume = edit_resume(
        app,
        vault,
        cfg,
        &draft.markdown,
        &checks.fact_check.markdown,
        jac_md,
        user_notes,
    )
    .await?;

    // Step 5: Persist workspace & export standalone HTML + DOCX
    let now = Local::now();
    let title_str = run_title
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .unwrap_or("General Application");
    let slug_suffix: String = title_str
        .chars()
        .map(|c| {
            if c.is_ascii_alphanumeric() {
                c.to_ascii_lowercase()
            } else {
                '-'
            }
        })
        .collect::<String>()
        .split('-')
        .filter(|s| !s.is_empty())
        .take(5)
        .collect::<Vec<_>>()
        .join("-");
    let slug = format!(
        "{}-{}",
        now.format("%Y%m%d-%H%M%S"),
        if slug_suffix.is_empty() {
            "application"
        } else {
            &slug_suffix
        }
    );

    let out_dir = vault.applications_dir().join(&slug);
    fs::create_dir_all(&out_dir)?;

    if let Some(jd) = trimmed_jd {
        fs::write(out_dir.join("jd.txt"), jd)?;
    }
    if let Some(arch) = &archetype_res {
        fs::write(
            out_dir.join("archetype_selection.json"),
            serde_json::to_string_pretty(arch)?,
        )?;
        if !arch.thinking.is_empty() {
            fs::write(out_dir.join("archetype_selection.json.thonk"), &arch.thinking)?;
        }
    }

    fs::write(out_dir.join("resume_draft.md"), &draft.markdown)?;
    fs::write(out_dir.join("resume_draft.md.thonk"), &draft.thinking)?;

    fs::write(
        out_dir.join("resume_fact_check.md"),
        &checks.fact_check.markdown,
    )?;
    fs::write(
        out_dir.join("resume_fact_check.md.thonk"),
        &checks.fact_check.thinking,
    )?;

    if let Some(jac) = &checks.alignment_check {
        fs::write(out_dir.join("resume_job_alignment_check.md"), &jac.markdown)?;
        fs::write(
            out_dir.join("resume_job_alignment_check.md.thonk"),
            &jac.thinking,
        )?;
    }

    fs::write(out_dir.join("resume_final.md"), &final_resume.markdown)?;
    fs::write(out_dir.join("resume_final.md.thonk"), &final_resume.thinking)?;

    let css = vault.read_style_file(style_filename)?;
    let (exported_html_path, exported_docx_path) =
        export_resume_bundle(&final_resume.markdown, &css, &out_dir.join("resume_final"))?;

    let summary = ApplicationRunSummary {
        slug: slug.clone(),
        created_at: now.format("%Y-%m-%d %H:%M").to_string(),
        title: title_str.to_string(),
        archetype_filename: archetype_res.as_ref().map(|a| a.archetype_filename.clone()),
        match_score: checks.alignment_check.as_ref().and_then(|a| a.match_score),
        status: "Ready".to_string(),
        has_final_resume: true,
        has_cover_letter: false,
    };
    fs::write(
        out_dir.join("manifest.json"),
        serde_json::to_string_pretty(&summary)?,
    )?;

    emit_progress(
        app,
        AgentStage::ResumeEditor,
        "status",
        format!("Full pipeline completed and saved to var/applications/{slug}"),
    );

    Ok(FullPipelineResult {
        slug,
        output_dir: out_dir.to_string_lossy().to_string(),
        archetype: archetype_res,
        draft,
        fact_check: checks.fact_check,
        alignment_check: checks.alignment_check,
        final_resume,
        exported_html_path,
        exported_docx_path,
    })
}

pub fn load_application_workspace(
    vault: &VaultManager,
    slug: &str,
) -> Result<LoadedApplicationWorkspace> {
    let clean_slug = VaultManager::validate_safe_filename(slug)?;
    let dir = vault.applications_dir().join(&clean_slug);
    if !dir.is_dir() {
        bail!("Application workspace \"{}\" does not exist.", clean_slug);
    }

    let summary = if dir.join("manifest.json").is_file() {
        serde_json::from_str(&fs::read_to_string(dir.join("manifest.json"))?)?
    } else {
        ApplicationRunSummary {
            slug: clean_slug.clone(),
            created_at: String::new(),
            title: clean_slug,
            archetype_filename: None,
            match_score: None,
            status: "Drafted".to_string(),
            has_final_resume: dir.join("resume_final.md").is_file(),
            has_cover_letter: dir.join("cover_letter.md").is_file(),
        }
    };

    let read_opt = |name: &str| -> String {
        fs::read_to_string(dir.join(name)).unwrap_or_default()
    };

    let archetype_selection = if dir.join("archetype_selection.json").is_file() {
        serde_json::from_str(&read_opt("archetype_selection.json")).ok()
    } else {
        None
    };

    Ok(LoadedApplicationWorkspace {
        summary,
        job_description: read_opt("jd.txt"),
        archetype_selection,
        draft_markdown: read_opt("resume_draft.md"),
        fact_check_markdown: read_opt("resume_fact_check.md"),
        alignment_check_markdown: read_opt("resume_job_alignment_check.md"),
        final_resume_markdown: read_opt("resume_final.md"),
        cover_letter_markdown: read_opt("cover_letter.md"),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_archetype_selection_with_code_fences() {
        let raw = "```json\n{\n  \"archetype_filename\": \"technical_consultant.sample.md\",\n  \"rationale\": \"Matches principal architecture consulting scope.\"\n}\n```";
        let (file, rationale) = parse_archetype_selection_json(raw).unwrap();
        assert_eq!(file, "technical_consultant.sample.md");
        assert_eq!(rationale, "Matches principal architecture consulting scope.");
    }

    #[test]
    fn test_extract_match_score_and_metrics() {
        let report = "## Executive Summary\n\n- **Overall Match Score:** 92% (High Match)\n- **Summary:** Strong backend alignment.";
        assert_eq!(extract_match_score(report), Some(92));

        let draft = "- Accomplished latency reduction, as measured by [METRIC: e.g. % latency reduction], by rewriting gateway in Rust.\n- Saved [METRIC: e.g. $X monthly savings] in cloud spend.";
        let metrics = extract_metric_placeholders(draft);
        assert_eq!(metrics.len(), 2);
        assert_eq!(metrics[0], "[METRIC: e.g. % latency reduction]");
    }
}
