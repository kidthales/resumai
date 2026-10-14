use anyhow::{Context, Result};
use regex::Regex;
use std::fs;
use std::io::{Cursor, Write};
use std::path::Path;
use zip::write::SimpleFileOptions;
use zip::CompressionMethod;

/// Converts Resume Markdown into semantic HTML matching Pandoc's structure so existing
/// `assets/styles/resumes/*.css` stylesheets apply identically.
pub fn markdown_to_semantic_html(markdown: &str) -> String {
    let mut html = String::new();
    let mut in_list = false;

    for raw_line in markdown.lines() {
        let line = raw_line.trim();
        if line.is_empty() {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            continue;
        }

        if line == "---" || line == "***" {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            html.push_str("<hr />\n");
            continue;
        }

        if let Some(h1) = line.strip_prefix("# ") {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            html.push_str(&format!("<h1>{}</h1>\n", format_inline(h1.trim())));
        } else if let Some(h2) = line.strip_prefix("## ") {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            html.push_str(&format!("<h2>{}</h2>\n", format_inline(h2.trim())));
        } else if let Some(h3) = line.strip_prefix("### ") {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            html.push_str(&format!("<h3>{}</h3>\n", format_inline(h3.trim())));
        } else if let Some(item) = line.strip_prefix("- ").or_else(|| line.strip_prefix("* ")) {
            if !in_list {
                html.push_str("<ul>\n");
                in_list = true;
            }
            html.push_str(&format!("<li>{}</li>\n", format_inline(item.trim())));
        } else {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            html.push_str(&format!("<p>{}</p>\n", format_inline(line)));
        }
    }

    if in_list {
        html.push_str("</ul>\n");
    }

    html
}

fn escape_html(input: &str) -> String {
    input
        .replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
}

fn format_inline(input: &str) -> String {
    let escaped = escape_html(input);
    let bold_re = Regex::new(r"\*\*(.+?)\*\*").unwrap();
    let italic_re = Regex::new(r"\*(.+?)\*").unwrap();
    let code_re = Regex::new(r"`(.+?)`").unwrap();
    let link_re = Regex::new(r"\[([^\]]+)\]\(([^)]+)\)").unwrap();

    let step1 = link_re.replace_all(&escaped, r#"<a href="$2">$1</a>"#);
    let step2 = bold_re.replace_all(&step1, "<strong>$1</strong>");
    let step3 = italic_re.replace_all(&step2, "<em>$1</em>");
    let step4 = code_re.replace_all(&step3, "<code>$1</code>");
    step4.into_owned()
}

pub fn build_standalone_html(markdown: &str, css: &str, title: &str) -> String {
    let body = markdown_to_semantic_html(markdown);
    let safe_title = escape_html(title);
    format!(
        r#"<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>{safe_title}</title>
  <style>
{css}
  </style>
</head>
<body>
{body}
</body>
</html>
"#
    )
}

/// Generates a genuine OpenXML `.docx` file in pure Rust from the given Resume Markdown.
pub fn build_docx_bytes(markdown: &str) -> Result<Vec<u8>> {
    let mut buf = Cursor::new(Vec::new());
    {
        let mut zip = zip::ZipWriter::new(&mut buf);
        let options = SimpleFileOptions::default()
            .compression_method(CompressionMethod::Deflated)
            .unix_permissions(0o644);

        zip.start_file("[Content_Types].xml", options)?;
        zip.write_all(CONTENT_TYPES_XML.as_bytes())?;

        zip.start_file("_rels/.rels", options)?;
        zip.write_all(RELS_XML.as_bytes())?;

        zip.start_file("word/_rels/document.xml.rels", options)?;
        zip.write_all(DOCUMENT_RELS_XML.as_bytes())?;

        zip.start_file("word/styles.xml", options)?;
        zip.write_all(STYLES_XML.as_bytes())?;

        let doc_xml = markdown_to_word_document_xml(markdown);
        zip.start_file("word/document.xml", options)?;
        zip.write_all(doc_xml.as_bytes())?;

        zip.finish()?;
    }
    Ok(buf.into_inner())
}

pub fn export_resume_bundle(
    markdown: &str,
    css: &str,
    output_stem: &Path,
) -> Result<(String, String)> {
    if let Some(parent) = output_stem.parent() {
        fs::create_dir_all(parent)?;
    }
    let html_path = output_stem.with_extension("html");
    let docx_path = output_stem.with_extension("docx");

    let html_doc = build_standalone_html(markdown, css, "Resume");
    fs::write(&html_path, html_doc).context("Failed to write exported HTML file")?;

    let docx_bytes = build_docx_bytes(markdown)?;
    fs::write(&docx_path, docx_bytes).context("Failed to write exported DOCX file")?;

    Ok((
        html_path.to_string_lossy().to_string(),
        docx_path.to_string_lossy().to_string(),
    ))
}

fn escape_xml(input: &str) -> String {
    input
        .replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
        .replace('\'', "&apos;")
}

fn strip_markdown_markers(input: &str) -> String {
    let link_re = Regex::new(r"\[([^\]]+)\]\(([^)]+)\)").unwrap();
    let linked = link_re.replace_all(input, "$1 ($2)");
    linked.replace("**", "").replace('`', "")
}

fn markdown_to_word_document_xml(markdown: &str) -> String {
    let mut paragraphs = String::new();

    for raw_line in markdown.lines() {
        let line = raw_line.trim();
        if line.is_empty() || line == "---" || line == "***" {
            continue;
        }

        if let Some(h1) = line.strip_prefix("# ") {
            let text = escape_xml(&strip_markdown_markers(h1.trim()));
            paragraphs.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>{text}</w:t></w:r></w:p>"#
            ));
        } else if let Some(h2) = line.strip_prefix("## ") {
            let text = escape_xml(&strip_markdown_markers(h2.trim()));
            paragraphs.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr><w:r><w:t>{text}</w:t></w:r></w:p>"#
            ));
        } else if let Some(h3) = line.strip_prefix("### ") {
            let text = escape_xml(&strip_markdown_markers(h3.trim()));
            paragraphs.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Heading3"/></w:pPr><w:r><w:t>{text}</w:t></w:r></w:p>"#
            ));
        } else if let Some(bullet) = line.strip_prefix("- ").or_else(|| line.strip_prefix("* ")) {
            let text = escape_xml(&strip_markdown_markers(bullet.trim()));
            paragraphs.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="ListBullet"/><w:ind w:left="360" w:hanging="180"/></w:pPr><w:r><w:t xml:space="preserve">• {text}</w:t></w:r></w:p>"#
            ));
        } else {
            let text = escape_xml(&strip_markdown_markers(line));
            paragraphs.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr><w:r><w:t xml:space="preserve">{text}</w:t></w:r></w:p>"#
            ));
        }
    }

    format!(
        r#"<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    {paragraphs}
    <w:sectPr>
      <w:pgSz w:w="12240" w:h="15840"/>
      <w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720" w:header="720" w:footer="720" w:gutter="0"/>
    </w:sectPr>
  </w:body>
</w:document>"#
    )
}

const CONTENT_TYPES_XML: &str = r#"<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>"#;

const RELS_XML: &str = r#"<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>"#;

const DOCUMENT_RELS_XML: &str = r#"<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>"#;

const STYLES_XML: &str = r#"<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:pPr><w:spacing w:after="80" w:line="250" w:lineRule="auto"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="21"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:pPr><w:spacing w:before="0" w:after="60"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="36"/><w:color w:val="111827"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:pPr>
      <w:pBdr><w:bottom w:val="single" w:sz="6" w:space="2" w:color="9CA3AF"/></w:pBdr>
      <w:spacing w:before="200" w:after="80"/>
    </w:pPr>
    <w:rPr><w:b/><w:caps/><w:sz w:val="23"/><w:color w:val="1F2937"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/>
    <w:pPr><w:spacing w:before="140" w:after="40"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="22"/><w:color w:val="111827"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListBullet">
    <w:name w:val="List Bullet"/>
    <w:pPr><w:spacing w:after="50" w:line="245" w:lineRule="auto"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="20"/></w:rPr>
  </w:style>
</w:styles>"#;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_standalone_html_and_docx_generation() {
        let sample_md = "# Dr. Henry McCoy\n\nNew York, NY • hank@krakoa.org\n\n## Professional Summary\n\nPrincipal Architect with **20+ years** experience.\n\n## Professional Experience\n\n### Krakoan Gateway Systems\n\n- Accomplished 99.99% uptime by engineering fault-tolerant mesh relays.";
        let html = build_standalone_html(sample_md, "body { font-family: sans-serif; }", "Hank Resume");
        assert!(html.contains("<h1>Dr. Henry McCoy</h1>"));
        assert!(html.contains("<strong>20+ years</strong>"));
        assert!(html.contains("<li>Accomplished 99.99% uptime"));

        let docx_bytes = build_docx_bytes(sample_md).unwrap();
        assert!(docx_bytes.len() > 500);
        // Verify PK zip magic header
        assert_eq!(&docx_bytes[0..2], b"PK");
    }
}
