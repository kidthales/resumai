use crate::vault::VaultManager;
use anyhow::{bail, Context, Result};
use futures_util::StreamExt;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::time::Duration;
use tauri::{AppHandle, Emitter};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LlmConfig {
    pub platform: String, // "ollama" | "gemini"
    pub ollama_base_url: String,
    pub ollama_model: String,
    pub gemini_api_key: String,
    pub gemini_flash_model: String,
    pub gemini_lite_model: String,
}

impl Default for LlmConfig {
    fn default() -> Self {
        Self {
            platform: "ollama".to_string(),
            ollama_base_url: std::env::var("OLLAMA_HTTP_CLIENT_BASE_URI")
                .unwrap_or_else(|_| "http://localhost:11434".to_string()),
            ollama_model: "qwen2.5:14b-instruct-q5_K_M".to_string(),
            gemini_api_key: std::env::var("GEMINI_API_KEY").unwrap_or_default(),
            gemini_flash_model: "gemini-3.5-flash".to_string(),
            gemini_lite_model: "gemini-3.5-flash-lite".to_string(),
        }
    }
}

#[derive(Debug, Clone, Copy)]
pub enum AgentStage {
    ArchetypeSelector,
    ResumeDrafter,
    FactChecker,
    JobAlignmentChecker,
    ResumeEditor,
    CoverLetter,
    PageFitOptimizer,
}

impl AgentStage {
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::ArchetypeSelector => "archetype_selector",
            Self::ResumeDrafter => "resume_drafter",
            Self::FactChecker => "fact_checker",
            Self::JobAlignmentChecker => "job_alignment_checker",
            Self::ResumeEditor => "resume_editor",
            Self::CoverLetter => "cover_letter",
            Self::PageFitOptimizer => "page_fit_optimizer",
        }
    }

    pub fn temperature(&self) -> f64 {
        match self {
            Self::ArchetypeSelector | Self::FactChecker => 0.0,
            Self::ResumeEditor | Self::PageFitOptimizer => 0.15,
            Self::JobAlignmentChecker | Self::CoverLetter => 0.3,
            Self::ResumeDrafter => 0.35,
        }
    }

    pub fn top_p(&self) -> f64 {
        match self {
            Self::ArchetypeSelector | Self::FactChecker => 0.1,
            Self::JobAlignmentChecker => 0.8,
            Self::ResumeEditor | Self::CoverLetter | Self::PageFitOptimizer => 0.85,
            Self::ResumeDrafter => 0.9,
        }
    }

    pub fn top_k(&self) -> u32 {
        match self {
            Self::ArchetypeSelector | Self::FactChecker => 20,
            Self::ResumeEditor | Self::PageFitOptimizer => 30,
            Self::JobAlignmentChecker | Self::CoverLetter => 35,
            Self::ResumeDrafter => 40,
        }
    }

    pub fn gemini_model<'a>(&self, cfg: &'a LlmConfig) -> &'a str {
        match self {
            Self::ArchetypeSelector | Self::ResumeDrafter | Self::JobAlignmentChecker => {
                &cfg.gemini_lite_model
            }
            Self::FactChecker | Self::ResumeEditor | Self::CoverLetter | Self::PageFitOptimizer => {
                &cfg.gemini_flash_model
            }
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StreamEventPayload {
    pub stage: String,
    pub kind: String, // "thinking" | "text" | "status" | "tool"
    pub chunk: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentExecutionOutput {
    pub result: String,
    pub thinking: String,
    pub model_used: String,
}

pub fn emit_progress(app: Option<&AppHandle>, stage: AgentStage, kind: &str, chunk: impl Into<String>) {
    if let Some(handle) = app {
        let _ = handle.emit(
            "agent:stream",
            StreamEventPayload {
                stage: stage.as_str().to_string(),
                kind: kind.to_string(),
                chunk: chunk.into(),
            },
        );
    }
}

pub async fn call_agent_streaming(
    app: Option<&AppHandle>,
    cfg: &LlmConfig,
    stage: AgentStage,
    system_prompt: &str,
    user_input: &str,
) -> Result<AgentExecutionOutput> {
    let platform = cfg.platform.trim().to_lowercase();
    match platform.as_str() {
        "gemini" => call_gemini_streaming(app, cfg, stage, system_prompt, user_input).await,
        _ => call_ollama_streaming(app, cfg, stage, system_prompt, user_input).await,
    }
}

pub async fn call_archetype_selector_with_tools(
    app: Option<&AppHandle>,
    vault: &VaultManager,
    cfg: &LlmConfig,
    system_prompt: &str,
    user_input: &str,
) -> Result<AgentExecutionOutput> {
    let platform = cfg.platform.trim().to_lowercase();
    emit_progress(
        app,
        AgentStage::ArchetypeSelector,
        "status",
        "Inspecting candidate archetypes via ArchetypeDirectoryTool...",
    );

    let active_files = vault.list_active_archetypes()?;
    emit_progress(
        app,
        AgentStage::ArchetypeSelector,
        "tool",
        format!("list_archetypes() -> {:?}", active_files),
    );

    let mut archetypes_bundle = String::new();
    for filename in &active_files {
        if let Ok(content) = vault.read_archetype_file(filename) {
            emit_progress(
                app,
                AgentStage::ArchetypeSelector,
                "tool",
                format!("read_archetype(\"{}\") ({} bytes)", filename, content.len()),
            );
            archetypes_bundle.push_str(&format!(
                "\n<archetype filename=\"{filename}\">\n{}\n</archetype>\n",
                content.trim()
            ));
        }
    }

    let enriched_user_input = format!(
        "{user_input}\n\nTool Execution Results (from `list_archetypes` and `read_archetype`):\n<available_archetypes>\n{archetypes_bundle}\n</available_archetypes>\n\nSelect the best matching `archetype_filename` from the list above and output only the valid JSON object."
    );

    match platform.as_str() {
        "gemini" => {
            call_gemini_streaming(
                app,
                cfg,
                AgentStage::ArchetypeSelector,
                system_prompt,
                &enriched_user_input,
            )
            .await
        }
        _ => {
            call_ollama_streaming(
                app,
                cfg,
                AgentStage::ArchetypeSelector,
                system_prompt,
                &enriched_user_input,
            )
            .await
        }
    }
}

async fn call_ollama_streaming(
    app: Option<&AppHandle>,
    cfg: &LlmConfig,
    stage: AgentStage,
    system_prompt: &str,
    user_input: &str,
) -> Result<AgentExecutionOutput> {
    let base = cfg.ollama_base_url.trim_end_matches('/');
    let url = format!("{base}/api/chat");
    let model = cfg.ollama_model.trim().to_string();

    let payload = json!({
        "model": model,
        "stream": true,
        "messages": [
            { "role": "system", "content": system_prompt },
            { "role": "user", "content": user_input }
        ],
        "options": {
            "temperature": stage.temperature(),
            "top_p": stage.top_p(),
            "top_k": stage.top_k(),
            "repeat_penalty": 1.1,
            "num_ctx": 16384
        }
    });

    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(600))
        .build()?;

    let resp = send_with_retry(&client, &url, None, &payload).await?;
    let mut stream = resp.bytes_stream();

    let mut raw_text = String::new();
    let mut thinking_text = String::new();
    let mut buffer = String::new();

    while let Some(item) = stream.next().await {
        let chunk = item.context("Error reading Ollama response stream")?;
        buffer.push_str(&String::from_utf8_lossy(&chunk));

        while let Some(pos) = buffer.find('\n') {
            let line = buffer[..pos].trim().to_string();
            buffer = buffer[pos + 1..].to_string();
            if line.is_empty() {
                continue;
            }
            if let Ok(val) = serde_json::from_str::<Value>(&line) {
                if let Some(err) = val.get("error").and_then(|e| e.as_str()) {
                    bail!("Ollama error: {err}");
                }
                if let Some(msg) = val.get("message") {
                    if let Some(think_delta) = msg.get("thinking").and_then(|t| t.as_str()) {
                        if !think_delta.is_empty() {
                            thinking_text.push_str(think_delta);
                            emit_progress(app, stage, "thinking", think_delta);
                        }
                    }
                    if let Some(content_delta) = msg.get("content").and_then(|c| c.as_str()) {
                        if !content_delta.is_empty() {
                            raw_text.push_str(content_delta);
                            emit_progress(app, stage, "text", content_delta);
                        }
                    }
                }
            }
        }
    }

    let (clean_result, inline_thinking) = split_inline_think_tags(&raw_text);
    if thinking_text.is_empty() && !inline_thinking.is_empty() {
        thinking_text = inline_thinking;
    }

    Ok(AgentExecutionOutput {
        result: clean_result,
        thinking: thinking_text,
        model_used: format!("ollama/{model}"),
    })
}

async fn call_gemini_streaming(
    app: Option<&AppHandle>,
    cfg: &LlmConfig,
    stage: AgentStage,
    system_prompt: &str,
    user_input: &str,
) -> Result<AgentExecutionOutput> {
    let api_key = if !cfg.gemini_api_key.trim().is_empty()
        && cfg.gemini_api_key.trim() != "!ChangeThisGeminiApiKey!"
    {
        cfg.gemini_api_key.trim().to_string()
    } else {
        std::env::var("GEMINI_API_KEY")
            .ok()
            .filter(|k| !k.trim().is_empty() && k != "!ChangeThisGeminiApiKey!")
            .context("Gemini API key is not configured. Set it in Provider Settings or GEMINI_API_KEY env var.")?
    };

    let model = stage.gemini_model(cfg);
    let url = format!(
        "https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent?alt=sse&key={api_key}"
    );

    let payload = json!({
        "systemInstruction": {
            "parts": [{ "text": system_prompt }]
        },
        "contents": [
            {
                "role": "user",
                "parts": [{ "text": user_input }]
            }
        ],
        "generationConfig": {
            "temperature": stage.temperature(),
            "topP": stage.top_p(),
            "topK": stage.top_k(),
            "maxOutputTokens": 65536
        }
    });

    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(300))
        .build()?;

    let resp = send_with_retry(&client, &url, None, &payload).await?;
    let mut stream = resp.bytes_stream();

    let mut result_text = String::new();
    let mut thinking_text = String::new();
    let mut buffer = String::new();

    while let Some(item) = stream.next().await {
        let chunk = item.context("Error reading Gemini SSE stream")?;
        buffer.push_str(&String::from_utf8_lossy(&chunk));

        while let Some(pos) = buffer.find('\n') {
            let line = buffer[..pos].trim().to_string();
            buffer = buffer[pos + 1..].to_string();
            if let Some(json_str) = line.strip_prefix("data:") {
                let trimmed = json_str.trim();
                if trimmed.is_empty() || trimmed == "[DONE]" {
                    continue;
                }
                if let Ok(val) = serde_json::from_str::<Value>(trimmed) {
                    if let Some(candidates) = val.get("candidates").and_then(|c| c.as_array()) {
                        for cand in candidates {
                            if let Some(parts) = cand
                                .get("content")
                                .and_then(|c| c.get("parts"))
                                .and_then(|p| p.as_array())
                            {
                                for part in parts {
                                    let is_thought = part
                                        .get("thought")
                                        .and_then(|t| t.as_bool())
                                        .unwrap_or(false);
                                    if let Some(text) = part.get("text").and_then(|t| t.as_str()) {
                                        if is_thought {
                                            thinking_text.push_str(text);
                                            emit_progress(app, stage, "thinking", text);
                                        } else {
                                            result_text.push_str(text);
                                            emit_progress(app, stage, "text", text);
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    Ok(AgentExecutionOutput {
        result: result_text,
        thinking: thinking_text,
        model_used: format!("gemini/{model}"),
    })
}

async fn send_with_retry(
    client: &reqwest::Client,
    url: &str,
    bearer: Option<&str>,
    payload: &Value,
) -> Result<reqwest::Response> {
    let max_attempts = 3;
    let mut delay = Duration::from_millis(1200);

    for attempt in 1..=max_attempts {
        let mut req = client.post(url).json(payload);
        if let Some(token) = bearer {
            req = req.bearer_auth(token);
        }

        match req.send().await {
            Ok(resp) => {
                let status = resp.status();
                if status.is_success() {
                    return Ok(resp);
                }
                let is_retryable = status.as_u16() == 429
                    || status.as_u16() == 502
                    || status.as_u16() == 503
                    || status.as_u16() == 504;
                let err_body = resp.text().await.unwrap_or_default();
                if is_retryable && attempt < max_attempts {
                    tokio::time::sleep(delay).await;
                    delay *= 2;
                    continue;
                }
                bail!("LLM HTTP {} error: {}", status.as_u16(), err_body);
            }
            Err(err) => {
                if attempt < max_attempts {
                    tokio::time::sleep(delay).await;
                    delay *= 2;
                    continue;
                }
                return Err(err).context(format!(
                    "Failed to connect to LLM endpoint ({url}). Is Ollama running or is your network connected?"
                ));
            }
        }
    }
    bail!("Exhausted retry attempts calling LLM endpoint.")
}

/// Splits `<think>...</think>` blocks if a reasoning model emits inline tags in its content stream.
pub fn split_inline_think_tags(raw: &str) -> (String, String) {
    let trimmed = raw.trim();
    if let Some(start) = trimmed.find("<think>") {
        if let Some(end) = trimmed.find("</think>") {
            if end > start {
                let thinking = trimmed[start + 7..end].trim().to_string();
                let mut remainder = String::new();
                remainder.push_str(&trimmed[..start]);
                remainder.push_str(&trimmed[end + 8..]);
                return (remainder.trim().to_string(), thinking);
            }
        }
    }
    (trimmed.to_string(), String::new())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_split_inline_think_tags() {
        let input = "<think>\nEvaluating seniority and stack...\n</think>\n# Final Resume\nContent";
        let (clean, think) = split_inline_think_tags(input);
        assert_eq!(think, "Evaluating seniority and stack...");
        assert_eq!(clean, "# Final Resume\nContent");
    }
}
