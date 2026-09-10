//! What every validator reports, and how.
//!
//! One type, shared. A finding means the same thing whichever validator
//! produced it, prints the same way, and maps to the same exit code -- which is
//! what stops five validators from growing five nearly-identical result types
//! and five nearly-identical printers that drift apart one fix at a time.
//!
//! Every line carries its command category in square brackets. The prefix is
//! atomic rather than assembled from the command path, so a caller grepping
//! `[word-budget]` gets that validator's output and nothing else regardless of
//! how the command is spelled on the way in.

use crate::Outcome;
use crate::cli::Format;

/// A measured value attached to a finding.
///
/// Kept as a small closed enum rather than a string so a count stays a number
/// all the way to the output: a consumer filtering on a measurement should not
/// have to parse it back out of prose.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Detail {
    Text(String),
    Count(usize),
}

impl std::fmt::Display for Detail {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Text(value) => formatter.write_str(value),
            Self::Count(value) => write!(formatter, "{value}"),
        }
    }
}

/// One thing a validator found wrong.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Finding {
    /// A stable identifier for the rule, not prose. It is what a consumer
    /// filters on, so it may not be reworded to improve a message.
    pub kind: &'static str,
    /// Repository-relative, so output does not depend on where the tool ran.
    pub path: String,
    pub line: Option<usize>,
    /// Set only by the structural validators, whose diagnostic format the
    /// standardization contract fixes as `path:line:column rule field message`.
    ///
    /// Its presence is what selects that format. The validators that shipped
    /// before it keep `path:line: message`, because a consumer's stored output
    /// may not change because a later command arrived with a different taste
    /// in diagnostics.
    pub column: Option<usize>,
    /// The metadata field a structural finding is about, when it is about one.
    pub field: Option<String>,
    pub message: String,
    /// Ordered, because the order is what a reader sees and what a machine
    /// consumer's first field is.
    pub details: Vec<(&'static str, Detail)>,
}

impl Finding {
    pub fn new(kind: &'static str, path: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            kind,
            path: path.into(),
            line: None,
            column: None,
            field: None,
            message: message.into(),
            details: Vec::new(),
        }
    }

    /// Attach the document a finding was made in.
    ///
    /// A validator that walks blocks inside a file builds its findings before
    /// it needs the path, and threading the path through every construction
    /// site only for one of them to forget it is the failure worth avoiding.
    #[must_use]
    pub fn rename(mut self, path: impl Into<String>) -> Self {
        self.path = path.into();
        self
    }

    /// Attach a measured value, in the order it should be read.
    #[must_use]
    pub fn with(mut self, key: &'static str, value: Detail) -> Self {
        self.details.push((key, value));
        self
    }

    #[must_use]
    pub fn at_line(mut self, line: usize) -> Self {
        self.line = Some(line);
        self
    }

    /// Place a structural finding, which selects the structural format.
    #[must_use]
    pub fn at(mut self, line: usize, column: usize) -> Self {
        self.line = Some(line);
        self.column = Some(column);
        self
    }

    /// Name the field a structural finding is about.
    #[must_use]
    pub fn about(mut self, field: impl Into<String>) -> Self {
        self.field = Some(field.into());
        self
    }

    fn render(&self, category: &str) -> String {
        let details = if self.details.is_empty() {
            String::new()
        } else {
            let pairs: Vec<String> = self
                .details
                .iter()
                .map(|(key, value)| format!("{key}={value}"))
                .collect();
            format!(" ({})", pairs.join(", "))
        };
        if let Some(column) = self.column {
            let line = self.line.unwrap_or(1);
            let field = match &self.field {
                Some(field) => format!(" {field}"),
                None => String::new(),
            };
            return format!(
                "[{category}] {}:{line}:{column} {}{field} {}{details}\n",
                self.path, self.kind, self.message
            );
        }
        let position = match self.line {
            Some(line) => format!("{}:{line}", self.path),
            None => self.path.clone(),
        };
        format!("[{category}] {position}: {}{details}\n", self.message)
    }
}

/// The result of one validator run.
pub struct Report {
    category: &'static str,
    /// What was looked at, singular: `file`, `diagram`, `link`. Reported even
    /// when it is zero, because a repository with no diagrams has a correct and
    /// permanent answer of zero, and a reader must not be able to confuse it
    /// with a walk that silently found nothing.
    subject: &'static str,
    inspected: usize,
    /// The paths that were looked at, when the thing inspected has one.
    ///
    /// Reported rather than merely counted because "which files did you read?"
    /// is a question a maintainer has to be able to answer without reading the
    /// validator: a surface that silently matched nothing and a surface that
    /// matched everything both produce a clean run.
    scanned: Vec<String>,
    notes: Vec<String>,
    /// Named quantities a leaf establishes about the whole run, rendered as
    /// top-level JSON members. Distinct from a finding's details, which belong
    /// to one violation, and from a note, which is prose.
    measurements: Vec<(&'static str, usize)>,
    findings: Vec<Finding>,
    /// Set when the run could not happen at all. Held on the report rather
    /// than short-circuiting to an `Outcome`, so that one place decides how a
    /// result is rendered and a refusal cannot quietly acquire a shape of its
    /// own under `--output json`.
    refusal: Option<String>,
}

impl Report {
    pub fn new(category: &'static str, subject: &'static str) -> Self {
        Self {
            category,
            subject,
            inspected: 0,
            scanned: Vec::new(),
            notes: Vec::new(),
            measurements: Vec::new(),
            findings: Vec::new(),
            refusal: None,
        }
    }

    pub fn inspected(&mut self, count: usize) -> &mut Self {
        self.inspected = count;
        self
    }

    /// Count one inspected subject that has no path worth listing.
    pub fn inspected_one(&mut self) -> &mut Self {
        self.inspected += 1;
        self
    }

    pub fn scanned(&mut self, path: impl Into<String>) -> &mut Self {
        self.scanned.push(path.into());
        self.inspected += 1;
        self
    }

    /// An extra line of stdout, under the same category prefix.
    ///
    /// For facts a run establishes that are not findings -- a digest, a count
    /// of what the canon holds -- and that a caller has to be able to read
    /// without parsing the validator's internals.
    pub fn note(&mut self, text: impl AsRef<str>) -> &mut Self {
        self.notes.push(text.as_ref().to_string());
        self
    }

    /// A quantity this run established, named as a caller will read it.
    pub fn measure(&mut self, key: &'static str, value: usize) -> &mut Self {
        self.measurements.push((key, value));
        self
    }

    pub fn found(&mut self, finding: Finding) -> &mut Self {
        self.findings.push(finding);
        self
    }

    /// Render as the caller asked. Exit `0` when clean, `1` when there are
    /// findings, and `2` only for a refusal: a validator that reached the point
    /// of reporting has a readable repository and a readable configuration, so
    /// whatever it says is about the repository's policy rather than about the
    /// invocation.
    pub fn render(&self, format: Format) -> Outcome {
        // A refusal renders the same way in both formats, and always with an
        // empty stdout. A caller parsing JSON has to be able to trust that
        // stdout either holds a result or holds nothing.
        if let Some(reason) = &self.refusal {
            return Outcome {
                exit_code: 2,
                stdout: String::new(),
                stderr: format!("[{}] {reason}\n", self.category),
            };
        }
        match format {
            Format::Text => self.finish(),
            Format::Json => self.as_json(),
        }
    }

    /// One object on one line.
    ///
    /// Line-delimited rather than pretty-printed so a caller can pipe the
    /// output through `grep` and `jq` alike, and so "every stdout line is a
    /// result" stays true whatever the leaf reports.
    fn as_json(&self) -> Outcome {
        let mut scanned = self.scanned.clone();
        scanned.sort();
        let mut findings: Vec<&Finding> = self.findings.iter().collect();
        findings.sort_by_key(|finding| {
            (
                finding.path.clone(),
                finding.line,
                finding.column,
                finding.kind,
                finding.field.clone(),
            )
        });

        let violations: Vec<String> = findings
            .iter()
            .map(|finding| {
                let mut fields = vec![
                    format!("\"kind\":{}", quote(finding.kind)),
                    format!("\"path\":{}", quote(&finding.path)),
                    format!("\"message\":{}", quote(&finding.message)),
                ];
                if let Some(line) = finding.line {
                    fields.push(format!("\"line\":{line}"));
                }
                if let Some(column) = finding.column {
                    fields.push(format!("\"column\":{column}"));
                }
                if let Some(field) = &finding.field {
                    fields.push(format!("\"field\":{}", quote(field)));
                }
                for (key, value) in &finding.details {
                    fields.push(match value {
                        Detail::Count(count) => format!("{}:{count}", quote(key)),
                        Detail::Text(text) => format!("{}:{}", quote(key), quote(text)),
                    });
                }
                format!("{{{}}}", fields.join(","))
            })
            .collect();

        let body = format!(
            "{{\"schemaVersion\":1,\"command\":{},\"exitCode\":{},\"subject\":{},\"inspected\":{},\"scanned\":[{}],\"notes\":[{}],\"violations\":[{}]{}}}\n",
            quote(self.category),
            u8::from(!self.findings.is_empty()),
            quote(self.subject),
            self.inspected,
            scanned
                .iter()
                .map(|path| quote(path))
                .collect::<Vec<_>>()
                .join(","),
            self.notes
                .iter()
                .map(|note| quote(note))
                .collect::<Vec<_>>()
                .join(","),
            violations.join(","),
            self.measurements
                .iter()
                .map(|(key, value)| format!(",{}:{value}", quote(key)))
                .collect::<String>()
        );

        Outcome {
            exit_code: u8::from(!self.findings.is_empty()),
            stdout: body,
            stderr: String::new(),
        }
    }

    fn finish(&self) -> Outcome {
        let summary = format!(
            "[{}] checked {} {}, {}\n",
            self.category,
            self.inspected,
            plural(self.subject, self.inspected),
            match self.findings.len() {
                0 => "no findings".to_string(),
                1 => "1 finding".to_string(),
                count => format!("{count} findings"),
            }
        );

        let mut summary = summary;
        for note in &self.notes {
            summary.push_str(&format!("[{}] {note}\n", self.category));
        }
        let mut scanned = self.scanned.clone();
        scanned.sort();
        for path in scanned {
            summary.push_str(&format!("[{}] scanned {path}\n", self.category));
        }

        // Sorted, so two runs over one repository produce byte-identical
        // output and a diff between two repositories is a diff about the
        // repositories.
        //
        // Structural findings sort by the key their contract states -- path,
        // line, column, rule, field -- which is not what sorting the rendered
        // text would give: `:10:` precedes `:2:` as a string. Everything else
        // keeps the rendered-text order it shipped with, because a consumer's
        // stored output may not be reordered by a release it did not ask for.
        let structural =
            !self.findings.is_empty() && self.findings.iter().all(|f| f.column.is_some());
        let stderr: String = if structural {
            let mut ordered: Vec<&Finding> = self.findings.iter().collect();
            ordered.sort_by(|left, right| {
                (&left.path, left.line, left.column, left.kind, &left.field).cmp(&(
                    &right.path,
                    right.line,
                    right.column,
                    right.kind,
                    &right.field,
                ))
            });
            ordered
                .iter()
                .map(|finding| finding.render(self.category))
                .collect()
        } else {
            let mut rendered: Vec<String> = self
                .findings
                .iter()
                .map(|finding| finding.render(self.category))
                .collect();
            rendered.sort();
            rendered.concat()
        };

        Outcome {
            exit_code: u8::from(!self.findings.is_empty()),
            stdout: summary,
            stderr,
        }
    }

    /// A run that could not happen: an unreadable repository or an unusable
    /// configuration. stdout stays empty, because a summary line here would
    /// claim something was checked when nothing was.
    pub fn refused(category: &'static str, message: impl AsRef<str>) -> Self {
        let mut report = Self::new(category, "subject");
        report.refusal = Some(message.as_ref().to_string());
        report
    }
}

/// A JSON string literal.
///
/// Hand-written rather than serialised through a derive because a report is a
/// handful of strings and numbers, and the alternative is a mirror type per
/// validator kept in step with the type it mirrors.
fn quote(value: &str) -> String {
    let mut quoted = String::with_capacity(value.len() + 2);
    quoted.push('"');
    for character in value.chars() {
        match character {
            '"' => quoted.push_str("\\\""),
            '\\' => quoted.push_str("\\\\"),
            '\n' => quoted.push_str("\\n"),
            '\r' => quoted.push_str("\\r"),
            '\t' => quoted.push_str("\\t"),
            control if control < ' ' => {
                quoted.push_str(&format!("\\u{:04x}", control as u32));
            }
            other => quoted.push(other),
        }
    }
    quoted.push('"');
    quoted
}

fn plural(subject: &str, count: usize) -> String {
    if count == 1 {
        subject.to_string()
    } else if let Some(stem) = subject.strip_suffix('y') {
        // `directory` -> `directories`, not `directorys`.
        format!("{stem}ies")
    } else if subject.ends_with('s') {
        // `harness` is one of the subjects, and "4 harnesss" reads as a defect
        // in the tool rather than as a count of four.
        format!("{subject}es")
    } else {
        format!("{subject}s")
    }
}
