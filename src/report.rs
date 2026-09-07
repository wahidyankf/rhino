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

    fn render(&self, category: &str) -> String {
        let position = match self.line {
            Some(line) => format!("{}:{line}", self.path),
            None => self.path.clone(),
        };
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
    findings: Vec<Finding>,
}

impl Report {
    pub fn new(category: &'static str, subject: &'static str) -> Self {
        Self {
            category,
            subject,
            inspected: 0,
            findings: Vec::new(),
        }
    }

    pub fn inspected(&mut self, count: usize) -> &mut Self {
        self.inspected = count;
        self
    }

    pub fn found(&mut self, finding: Finding) -> &mut Self {
        self.findings.push(finding);
        self
    }

    /// Exit `0` when clean and `1` when there are findings. Never `2`: a
    /// validator that reached the point of reporting has a readable repository
    /// and a readable configuration, so whatever it says is about the
    /// repository's policy rather than about the invocation.
    pub fn finish(&self) -> Outcome {
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

        let stderr: String = self
            .findings
            .iter()
            .map(|finding| finding.render(self.category))
            .collect();

        Outcome {
            exit_code: u8::from(!self.findings.is_empty()),
            stdout: summary,
            stderr,
        }
    }

    /// A run that could not happen: an unreadable repository or an unusable
    /// configuration. stdout stays empty, because a summary line here would
    /// claim something was checked when nothing was.
    pub fn refused(category: &str, message: impl AsRef<str>) -> Outcome {
        Outcome {
            exit_code: 2,
            stdout: String::new(),
            stderr: format!("[{category}] {}\n", message.as_ref()),
        }
    }
}

fn plural(subject: &str, count: usize) -> String {
    if count == 1 {
        subject.to_string()
    } else {
        format!("{subject}s")
    }
}
