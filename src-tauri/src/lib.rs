pub mod export;
pub mod llm;
pub mod pipeline;
pub mod vault;

use export::export_resume_bundle;
use llm::LlmConfig;
use pipeline::{
    draft_resume, edit_resume, generate_cover_letter, load_application_workspace,
    run_full_pipeline, run_parallel_checks, select_archetype, ArchetypeSelectionResult,
    FullPipelineResult, LoadedApplicationWorkspace, ParallelChecksResult, StageArtifactResult,
};
use serde::{Deserialize, Serialize};
use tauri::AppHandle;
use vault::{VaultManager, VaultOverview};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExportResult {
    pub html_path: String,
    pub docx_path: String,
}

#[tauri::command]
fn get_vault_overview() -> Result<VaultOverview, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    vault.get_overview().map_err(|e| e.to_string())
}

#[tauri::command]
fn get_default_llm_config() -> LlmConfig {
    LlmConfig::default()
}

#[tauri::command]
fn read_vault_file(category: String, filename: String) -> Result<String, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    vault
        .read_vault_file(&category, &filename)
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn write_vault_file(category: String, filename: String, content: String) -> Result<String, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    vault
        .write_vault_file(&category, &filename, &content)
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn read_style_css(filename: Option<String>) -> Result<String, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    vault
        .read_style_file(filename.as_deref())
        .map_err(|e| e.to_string())
}

#[tauri::command]
fn append_fact_to_work_history(
    fact_bullet: String,
    role_context: Option<String>,
) -> Result<String, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    vault
        .append_fact_to_work_history(&fact_bullet, role_context.as_deref())
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn cmd_select_archetype(
    app: AppHandle,
    config: LlmConfig,
    job_description: String,
) -> Result<ArchetypeSelectionResult, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    select_archetype(Some(&app), &vault, &config, &job_description)
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn cmd_draft_resume(
    app: AppHandle,
    config: LlmConfig,
    job_description: Option<String>,
    archetype_filename: Option<String>,
) -> Result<StageArtifactResult, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    draft_resume(
        Some(&app),
        &vault,
        &config,
        job_description.as_deref(),
        archetype_filename.as_deref(),
    )
    .await
    .map_err(|e| e.to_string())
}

#[tauri::command]
async fn cmd_run_parallel_checks(
    app: AppHandle,
    config: LlmConfig,
    resume_markdown: String,
    job_description: Option<String>,
) -> Result<ParallelChecksResult, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    run_parallel_checks(
        Some(&app),
        &vault,
        &config,
        &resume_markdown,
        job_description.as_deref(),
    )
    .await
    .map_err(|e| e.to_string())
}

#[tauri::command]
async fn cmd_edit_resume(
    app: AppHandle,
    config: LlmConfig,
    resume_markdown: String,
    fact_check_report: String,
    job_alignment_report: Option<String>,
    user_notes: Option<String>,
) -> Result<StageArtifactResult, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    edit_resume(
        Some(&app),
        &vault,
        &config,
        &resume_markdown,
        &fact_check_report,
        job_alignment_report.as_deref(),
        user_notes.as_deref(),
    )
    .await
    .map_err(|e| e.to_string())
}

#[tauri::command]
async fn cmd_generate_cover_letter(
    app: AppHandle,
    config: LlmConfig,
    resume_markdown: String,
    job_description: Option<String>,
    archetype_filename: Option<String>,
    mode: String,
) -> Result<StageArtifactResult, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    generate_cover_letter(
        Some(&app),
        &vault,
        &config,
        &resume_markdown,
        job_description.as_deref(),
        archetype_filename.as_deref(),
        &mode,
    )
    .await
    .map_err(|e| e.to_string())
}

#[tauri::command]
async fn cmd_run_full_pipeline(
    app: AppHandle,
    config: LlmConfig,
    run_title: Option<String>,
    job_description: Option<String>,
    archetype_override: Option<String>,
    style_filename: Option<String>,
    user_notes: Option<String>,
) -> Result<FullPipelineResult, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    run_full_pipeline(
        Some(&app),
        &vault,
        &config,
        run_title.as_deref(),
        job_description.as_deref(),
        archetype_override.as_deref(),
        style_filename.as_deref(),
        user_notes.as_deref(),
    )
    .await
    .map_err(|e| e.to_string())
}

#[tauri::command]
fn cmd_export_resume(
    markdown: String,
    style_filename: Option<String>,
    output_name: Option<String>,
) -> Result<ExportResult, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    let css = vault
        .read_style_file(style_filename.as_deref())
        .map_err(|e| e.to_string())?;
    let safe_stem = output_name
        .as_deref()
        .map(str::trim)
        .filter(|s| !s.is_empty())
        .unwrap_or("resume_export");
    let clean_stem = VaultManager::validate_safe_filename(safe_stem).map_err(|e| e.to_string())?;
    let export_dir = vault.root_dir().join("var").join("exports");
    let stem_path = export_dir.join(clean_stem);
    let (html_path, docx_path) =
        export_resume_bundle(&markdown, &css, &stem_path).map_err(|e| e.to_string())?;
    Ok(ExportResult {
        html_path,
        docx_path,
    })
}

#[tauri::command]
fn cmd_load_application(slug: String) -> Result<LoadedApplicationWorkspace, String> {
    let vault = VaultManager::discover().map_err(|e| e.to_string())?;
    load_application_workspace(&vault, &slug).map_err(|e| e.to_string())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            get_vault_overview,
            get_default_llm_config,
            read_vault_file,
            write_vault_file,
            read_style_css,
            append_fact_to_work_history,
            cmd_select_archetype,
            cmd_draft_resume,
            cmd_run_parallel_checks,
            cmd_edit_resume,
            cmd_generate_cover_letter,
            cmd_run_full_pipeline,
            cmd_export_resume,
            cmd_load_application,
        ])
        .run(tauri::generate_context!())
        .expect("error while running ResumAI Studio Tauri application");
}
