use anyhow::{Context, Result};
use regex::Regex;
use std::fs;
use std::io::{Cursor, Write};
use std::path::Path;
use zip::write::SimpleFileOptions;
use zip::CompressionMethod;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ColAlign {
    Left,
    Center,
    Right,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum InlineStyle {
    Normal,
    Bold,
    Italic,
    Code,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TextRun {
    pub text: String,
    pub style: InlineStyle,
}

/// Tokenizes inline Markdown (`**bold**`, `*italic*`, `` `code` ``, `[text](url)`) into
/// typed `TextRun` items suitable for OpenXML `<w:r>` run generation.
pub fn parse_inline_runs(input: &str) -> Vec<TextRun> {
    let link_re = Regex::new(r"\[([^\]]+)\]\(([^)]+)\)").unwrap();
    let linked = link_re.replace_all(input, "$1 ($2)").to_string();

    let token_re = Regex::new(r"(\*\*(?P<bold>[^*]+?)\*\*|`(?P<code>[^`]+?)`|\*(?P<italic>[^*]+?)\*)").unwrap();
    let mut runs = Vec::new();
    let mut last_index = 0;

    for caps in token_re.captures_iter(&linked) {
        let whole_match = caps.get(0).unwrap();
        let start = whole_match.start();
        let end = whole_match.end();

        if start > last_index {
            runs.push(TextRun {
                text: linked[last_index..start].to_string(),
                style: InlineStyle::Normal,
            });
        }

        if let Some(bold) = caps.name("bold") {
            runs.push(TextRun {
                text: bold.as_str().to_string(),
                style: InlineStyle::Bold,
            });
        } else if let Some(code) = caps.name("code") {
            runs.push(TextRun {
                text: code.as_str().to_string(),
                style: InlineStyle::Code,
            });
        } else if let Some(italic) = caps.name("italic") {
            runs.push(TextRun {
                text: italic.as_str().to_string(),
                style: InlineStyle::Italic,
            });
        }

        last_index = end;
    }

    if last_index < linked.len() {
        runs.push(TextRun {
            text: linked[last_index..].to_string(),
            style: InlineStyle::Normal,
        });
    }

    if runs.is_empty() {
        runs.push(TextRun {
            text: String::new(),
            style: InlineStyle::Normal,
        });
    }

    runs
}

pub fn render_docx_runs(runs: &[TextRun]) -> String {
    let mut xml = String::new();
    for run in runs {
        if run.text.is_empty() {
            continue;
        }
        let escaped = escape_xml(&run.text);
        match run.style {
            InlineStyle::Normal => {
                xml.push_str(&format!(r#"<w:r><w:t xml:space="preserve">{escaped}</w:t></w:r>"#));
            }
            InlineStyle::Bold => {
                xml.push_str(&format!(r#"<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">{escaped}</w:t></w:r>"#));
            }
            InlineStyle::Italic => {
                xml.push_str(&format!(r#"<w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve">{escaped}</w:t></w:r>"#));
            }
            InlineStyle::Code => {
                xml.push_str(&format!(
                    r#"<w:r><w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/><w:sz w:val="19"/></w:rPr><w:t xml:space="preserve">{escaped}</w:t></w:r>"#
                ));
            }
        }
    }
    xml
}

fn is_table_row(line: &str) -> bool {
    let trimmed = line.trim();
    trimmed.starts_with('|') && trimmed.ends_with('|') && trimmed.len() >= 2
}

fn parse_table_cells(line: &str) -> Vec<String> {
    let trimmed = line.trim();
    if trimmed.len() < 2 {
        return Vec::new();
    }
    let inner = &trimmed[1..trimmed.len() - 1];
    inner.split('|').map(|c| c.trim().to_string()).collect()
}

fn is_table_separator(cells: &[String]) -> bool {
    if cells.is_empty() {
        return false;
    }
    cells.iter().all(|c| {
        let clean: String = c.chars().filter(|ch| *ch != ' ').collect();
        !clean.is_empty() && clean.chars().all(|ch| ch == '-' || ch == ':')
    })
}

fn parse_separator_alignments(cells: &[String]) -> Vec<ColAlign> {
    cells
        .iter()
        .map(|c| {
            let clean = c.trim();
            let starts = clean.starts_with(':');
            let ends = clean.ends_with(':');
            if starts && ends {
                ColAlign::Center
            } else if ends {
                ColAlign::Right
            } else {
                ColAlign::Left
            }
        })
        .collect()
}

fn render_table_html(rows: &[Vec<String>], alignments: &[ColAlign]) -> String {
    if rows.is_empty() {
        return String::new();
    }
    let is_two_col = rows[0].len() == 2;
    let table_class = if is_two_col {
        "table-borderless table-header-grid"
    } else {
        "table-matrix"
    };

    let mut html = format!(r#"<table class="{table_class}">"#);
    html.push('\n');

    for (row_idx, row) in rows.iter().enumerate() {
        html.push_str("  <tr>\n");
        let tag = if row_idx == 0 && !is_two_col { "th" } else { "td" };
        for (col_idx, cell) in row.iter().enumerate() {
            let align = if is_two_col && col_idx == 1 {
                alignments.get(col_idx).copied().unwrap_or(ColAlign::Right)
            } else {
                alignments.get(col_idx).copied().unwrap_or(ColAlign::Left)
            };
            let align_css = match align {
                ColAlign::Left => "text-align: left;",
                ColAlign::Center => "text-align: center;",
                ColAlign::Right => "text-align: right;",
            };
            let inner = format_inline(cell);
            html.push_str(&format!(r#"    <{tag} style="{align_css}">{inner}</{tag}>"#));
            html.push('\n');
        }
        html.push_str("  </tr>\n");
    }
    html.push_str("</table>\n");
    html
}

fn render_table_docx(rows: &[Vec<String>], alignments: &[ColAlign]) -> String {
    if rows.is_empty() {
        return String::new();
    }
    let col_count = rows[0].len();
    if col_count == 0 {
        return String::new();
    }

    let is_two_col = col_count == 2;
    let total_width = 10800; // 12240 - 720*2 available dxa

    let col_widths: Vec<usize> = if is_two_col {
        vec![7020, 3780] // 65% / 35% layout grid
    } else {
        let w = total_width / col_count;
        let mut widths = vec![w; col_count];
        let remainder = total_width - (w * col_count);
        if let Some(last) = widths.last_mut() {
            *last += remainder;
        }
        widths
    };

    let mut grid_cols = String::new();
    for w in &col_widths {
        grid_cols.push_str(&format!(r#"<w:gridCol w:w="{w}"/>"#));
    }

    let borders_xml = if is_two_col {
        r#"<w:tblBorders>
      <w:top w:val="none"/>
      <w:left w:val="none"/>
      <w:bottom w:val="none"/>
      <w:right w:val="none"/>
      <w:insideH w:val="none"/>
      <w:insideV w:val="none"/>
    </w:tblBorders>"#
    } else {
        r#"<w:tblBorders>
      <w:top w:val="single" w:sz="4" w:space="0" w:color="D1D5DB"/>
      <w:left w:val="none"/>
      <w:bottom w:val="single" w:sz="4" w:space="0" w:color="D1D5DB"/>
      <w:right w:val="none"/>
      <w:insideH w:val="single" w:sz="4" w:space="0" w:color="E5E7EB"/>
      <w:insideV w:val="none"/>
    </w:tblBorders>"#
    };

    let mut trs = String::new();
    for (row_idx, row) in rows.iter().enumerate() {
        let mut tcs = String::new();
        for (col_idx, cell) in row.iter().enumerate() {
            let width = col_widths.get(col_idx).copied().unwrap_or(2000);
            let align = if is_two_col && col_idx == 1 {
                alignments.get(col_idx).copied().unwrap_or(ColAlign::Right)
            } else {
                alignments.get(col_idx).copied().unwrap_or(ColAlign::Left)
            };

            let align_str = match align {
                ColAlign::Left => "left",
                ColAlign::Center => "center",
                ColAlign::Right => "right",
            };

            let runs = parse_inline_runs(cell);
            let mut runs_xml = render_docx_runs(&runs);
            if runs_xml.is_empty() {
                runs_xml = r#"<w:r><w:t xml:space="preserve"></w:t></w:r>"#.to_string();
            }

            let extra_p_pr = if !is_two_col && row_idx == 0 {
                format!(
                    r#"<w:pPr><w:jc w:val="{align_str}"/><w:spacing w:before="40" w:after="40" w:line="240" w:lineRule="auto"/></w:pPr>"#
                )
            } else {
                format!(
                    r#"<w:pPr><w:jc w:val="{align_str}"/><w:spacing w:before="20" w:after="20" w:line="240" w:lineRule="auto"/></w:pPr>"#
                )
            };

            tcs.push_str(&format!(
                r#"<w:tc><w:tcPr><w:tcW w:w="{width}" w:type="dxa"/></w:tcPr><w:p>{extra_p_pr}{runs_xml}</w:p></w:tc>"#
            ));
        }
        trs.push_str(&format!(r#"<w:tr><w:trPr><w:cantSplit/></w:trPr>{tcs}</w:tr>"#));
    }

    format!(
        r#"<w:tbl>
  <w:tblPr>
    <w:tblW w:w="{total_width}" w:type="dxa"/>
    {borders_xml}
    <w:tblCellMar>
      <w:top w:w="60" w:type="dxa"/>
      <w:bottom w:w="60" w:type="dxa"/>
      <w:left w:w="80" w:type="dxa"/>
      <w:right w:w="80" w:type="dxa"/>
    </w:tblCellMar>
  </w:tblPr>
  <w:tblGrid>{grid_cols}</w:tblGrid>
  {trs}
</w:tbl>"#
    )
}

/// Converts Resume Markdown into semantic HTML matching Pandoc's structure so existing
/// `assets/styles/resumes/*.css` stylesheets apply identically.
pub fn markdown_to_semantic_html(markdown: &str) -> String {
    let mut html = String::new();
    let mut in_list = false;

    let lines: Vec<&str> = markdown.lines().collect();
    let mut i = 0;

    while i < lines.len() {
        let raw_line = lines[i];
        let line = raw_line.trim();

        if line.is_empty() {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            i += 1;
            continue;
        }

        if line == "---" || line == "***" {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            html.push_str("<hr />\n");
            i += 1;
            continue;
        }

        if is_table_row(line) {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            let mut table_rows: Vec<Vec<String>> = Vec::new();
            let mut alignments: Vec<ColAlign> = Vec::new();

            while i < lines.len() && is_table_row(lines[i].trim()) {
                let cells = parse_table_cells(lines[i].trim());
                if is_table_separator(&cells) {
                    alignments = parse_separator_alignments(&cells);
                } else {
                    table_rows.push(cells);
                }
                i += 1;
            }

            html.push_str(&render_table_html(&table_rows, &alignments));
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
        } else if let Some(quote) = line.strip_prefix("> ").or_else(|| line.strip_prefix(">")) {
            if in_list {
                html.push_str("</ul>\n");
                in_list = false;
            }
            html.push_str(&format!(
                "<blockquote><p>{}</p></blockquote>\n",
                format_inline(quote.trim())
            ));
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

        i += 1;
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

fn markdown_to_word_document_xml(markdown: &str) -> String {
    let mut body_xml = String::new();
    let lines: Vec<&str> = markdown.lines().collect();
    let mut i = 0;

    while i < lines.len() {
        let raw_line = lines[i];
        let line = raw_line.trim();

        if line.is_empty() || line == "---" || line == "***" {
            i += 1;
            continue;
        }

        if is_table_row(line) {
            let mut table_rows: Vec<Vec<String>> = Vec::new();
            let mut alignments: Vec<ColAlign> = Vec::new();

            while i < lines.len() && is_table_row(lines[i].trim()) {
                let cells = parse_table_cells(lines[i].trim());
                if is_table_separator(&cells) {
                    alignments = parse_separator_alignments(&cells);
                } else {
                    table_rows.push(cells);
                }
                i += 1;
            }

            body_xml.push_str(&render_table_docx(&table_rows, &alignments));
            continue;
        }

        if let Some(h1) = line.strip_prefix("# ") {
            let runs = parse_inline_runs(h1.trim());
            let runs_xml = render_docx_runs(&runs);
            body_xml.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr>{runs_xml}</w:p>"#
            ));
        } else if let Some(h2) = line.strip_prefix("## ") {
            let runs = parse_inline_runs(h2.trim());
            let runs_xml = render_docx_runs(&runs);
            body_xml.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr>{runs_xml}</w:p>"#
            ));
        } else if let Some(h3) = line.strip_prefix("### ") {
            let runs = parse_inline_runs(h3.trim());
            let runs_xml = render_docx_runs(&runs);
            body_xml.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Heading3"/></w:pPr>{runs_xml}</w:p>"#
            ));
        } else if let Some(quote) = line.strip_prefix("> ").or_else(|| line.strip_prefix(">")) {
            let runs = parse_inline_runs(quote.trim());
            let runs_xml = render_docx_runs(&runs);
            body_xml.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Normal"/><w:pBdr><w:left w:val="single" w:sz="24" w:space="12" w:color="004D40"/></w:pBdr><w:ind w:left="360"/><w:spacing w:before="60" w:after="60" w:line="240" w:lineRule="auto"/></w:pPr><w:r><w:rPr><w:i/></w:rPr></w:r>{runs_xml}</w:p>"#
            ));
        } else if let Some(bullet) = line.strip_prefix("- ").or_else(|| line.strip_prefix("* ")) {
            let runs = parse_inline_runs(bullet.trim());
            let runs_xml = render_docx_runs(&runs);
            body_xml.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="ListBullet"/><w:ind w:left="360" w:hanging="180"/></w:pPr><w:r><w:t xml:space="preserve">• </w:t></w:r>{runs_xml}</w:p>"#
            ));
        } else {
            let runs = parse_inline_runs(line);
            let runs_xml = render_docx_runs(&runs);
            body_xml.push_str(&format!(
                r#"<w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr>{runs_xml}</w:p>"#
            ));
        }

        i += 1;
    }

    format!(
        r#"<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    {body_xml}
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
    use std::io::Read;
    use zip::ZipArchive;

    #[test]
    fn test_standalone_html_and_docx_generation() {
        let sample_md = "# Dr. Henry McCoy\n\nNew York, NY • hank@krakoa.org\n\n## Professional Summary\n\nPrincipal Architect with **20+ years** experience.\n\n## Professional Experience\n\n### Krakoan Gateway Systems\n\n- Accomplished 99.99% uptime by engineering fault-tolerant mesh relays.";
        let html = build_standalone_html(sample_md, "body { font-family: sans-serif; }", "Hank Resume");
        assert!(html.contains("<h1>Dr. Henry McCoy</h1>"));
        assert!(html.contains("<strong>20+ years</strong>"));
        assert!(html.contains("<li>Accomplished 99.99% uptime"));

        let docx_bytes = build_docx_bytes(sample_md).unwrap();
        assert!(docx_bytes.len() > 500);
        assert_eq!(&docx_bytes[0..2], b"PK");

        let mut archive = ZipArchive::new(Cursor::new(docx_bytes)).unwrap();
        let mut doc_file = archive.by_name("word/document.xml").unwrap();
        let mut doc_xml = String::new();
        doc_file.read_to_string(&mut doc_xml).unwrap();
        assert!(doc_xml.contains("<w:b/>"));
        assert!(doc_xml.contains("20+ years"));
    }

    #[test]
    fn test_docx_tables_and_callouts() {
        let sample_md = r#"# Jane Doe

> "An exceptional distributed systems architect with deep database expertise."

## Experience

| Role & Title | Dates & Location |
| :--- | ---: |
| **Principal Consultant** | *2022 – Present • Remote* |
| **Senior Staff Engineer** | *2018 – 2022 • San Francisco, CA* |

## Core Competencies

| Domain | Technologies | Experience Level |
| :--- | :--- | :---: |
| Systems | `Rust`, `Go`, `C++` | 10+ Years |
| Cloud | AWS, GCP, Docker, Kubernetes | 8+ Years |
"#;

        let html = build_standalone_html(sample_md, "body {}", "Jane Resume");
        assert!(html.contains("<blockquote><p>&quot;An exceptional distributed systems"));
        assert!(html.contains(r#"<table class="table-borderless table-header-grid">"#));
        assert!(html.contains(r#"<table class="table-matrix">"#));
        assert!(html.contains("<strong>Principal Consultant</strong>"));
        assert!(html.contains("<code>Rust</code>"));

        let docx_bytes = build_docx_bytes(sample_md).unwrap();
        assert_eq!(&docx_bytes[0..2], b"PK");

        let mut archive = ZipArchive::new(Cursor::new(docx_bytes)).unwrap();
        let mut doc_file = archive.by_name("word/document.xml").unwrap();
        let mut doc_xml = String::new();
        doc_file.read_to_string(&mut doc_xml).unwrap();

        // Verify callout border
        assert!(doc_xml.contains(r#"<w:pBdr><w:left w:val="single" w:sz="24" w:space="12" w:color="004D40"/>"#));

        // Verify borderless 2-column table grid widths (7020 and 3780 dxa)
        assert!(doc_xml.contains(r#"<w:gridCol w:w="7020"/>"#));
        assert!(doc_xml.contains(r#"<w:gridCol w:w="3780"/>"#));
        assert!(doc_xml.contains(r#"<w:top w:val="none"/>"#));

        // Verify 3-column table subtle borders and Consolas code runs
        assert!(doc_xml.contains(r#"<w:insideH w:val="single" w:sz="4" w:space="0" w:color="E5E7EB"/>"#));
        assert!(doc_xml.contains(r#"<w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/>"#));
        assert!(doc_xml.contains("<w:b/>"));
        assert!(doc_xml.contains("<w:i/>"));
    }
}
