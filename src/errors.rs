//! The closed error-code vocabulary, and the machine-readable body that carries it.
//!
//! The exit status says what a caller should do; the code says what happened.
//! Keeping them in separate layers is what lets the status vocabulary stay
//! small enough to be universal while the reasons stay specific enough to act
//! on. A caller that only branches reads the status; a caller that reports
//! reads the code.
//!
//! The set is **closed**. A new reason adds a member here, is published by
//! `--help`, and is covered by the unit adapter that asserts every member is
//! namespaced and unique. Nothing may emit a code that is not in this list,
//! because a caller matching on an open vocabulary is matching on nothing.

use crate::cli::Format;

/// Every reason this build refuses, in `rhino.area.reason` form.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum ErrorCode {
    /// An option or command this build does not recognize.
    ArgsUnrecognized,
    /// A recognized option given without the value or the partner it needs.
    ArgsIncomplete,
    /// Standard input was selected and could not be read.
    InputUnreadable,
    /// The repository root could not be established or is unusable.
    RepositoryUnusable,
    /// The configuration file is missing, unparseable, or refuses this command.
    ConfigUnusable,
    /// The configuration parses and declares nothing this command can act on.
    ConfigUndeclared,
    /// A declared path leaves the repository root, or is not repository-relative.
    PathEscapesRoot,
    /// A declared file the command needs is not there.
    FileMissing,
    /// A declared file is there and could not be read.
    FileUnreadable,
    /// A declared target exists and the command was not authorized to replace it.
    FileExists,
    /// A gate child was named and does not exist.
    GateChildNotFound,
    /// A gate child exists and could not be executed.
    GateChildNotExecutable,
    /// A gate child could not be run for any other reason.
    GateChildRefused,
    /// A gate declares no runnable command.
    GateUndeclared,
    /// A declared toolchain could not be probed or provisioned.
    ToolchainFailed,
    /// A declared toolchain has no declaration for this platform.
    ToolchainUndeclared,
    /// A canonical harness adapter could not be validated or written.
    HarnessRefused,
}

impl ErrorCode {
    /// Every member, in the order `--help` publishes them.
    pub const ALL: &'static [Self] = &[
        Self::ArgsUnrecognized,
        Self::ArgsIncomplete,
        Self::InputUnreadable,
        Self::RepositoryUnusable,
        Self::ConfigUnusable,
        Self::ConfigUndeclared,
        Self::PathEscapesRoot,
        Self::FileMissing,
        Self::FileUnreadable,
        Self::FileExists,
        Self::GateChildNotFound,
        Self::GateChildNotExecutable,
        Self::GateChildRefused,
        Self::GateUndeclared,
        Self::ToolchainFailed,
        Self::ToolchainUndeclared,
        Self::HarnessRefused,
    ];

    pub fn as_str(self) -> &'static str {
        match self {
            Self::ArgsUnrecognized => "rhino.args.unrecognized",
            Self::ArgsIncomplete => "rhino.args.incomplete",
            Self::InputUnreadable => "rhino.input.unreadable",
            Self::RepositoryUnusable => "rhino.repository.unusable",
            Self::ConfigUnusable => "rhino.config.unusable",
            Self::ConfigUndeclared => "rhino.config.undeclared",
            Self::PathEscapesRoot => "rhino.path.escapes-root",
            Self::FileMissing => "rhino.file.missing",
            Self::FileUnreadable => "rhino.file.unreadable",
            Self::FileExists => "rhino.file.exists",
            Self::GateChildNotFound => "rhino.gate.child-not-found",
            Self::GateChildNotExecutable => "rhino.gate.child-not-executable",
            Self::GateChildRefused => "rhino.gate.child-refused",
            Self::GateUndeclared => "rhino.gate.undeclared",
            Self::ToolchainFailed => "rhino.toolchain.failed",
            Self::ToolchainUndeclared => "rhino.toolchain.undeclared",
            Self::HarnessRefused => "rhino.harness.refused",
        }
    }
}

/// What a refusal writes to standard error, in the format the caller asked for.
///
/// Both forms carry the same two facts. The text form is one line beginning
/// with the program name, which is what a person reads and what a floor-tier
/// wrapper can `grep`; the JSON form adds the code, which is what a program
/// matches on. Neither reaches standard output: a caller parsing a result has
/// to be able to trust that standard output holds a result or holds nothing.
pub fn body(format: Format, code: ErrorCode, message: &str) -> String {
    let message = message.trim_end_matches('\n');
    match format {
        Format::Text => format!("rhino: {message}\n"),
        Format::Json => format!(
            "{{\"schemaVersion\":1,\"error\":{{\"code\":{},\"message\":{}}}}}\n",
            quote(code.as_str()),
            quote(message)
        ),
    }
}

/// A JSON string, escaped for the two characters that can appear in a message
/// this build constructs, plus the control range a path or a child's reason
/// could carry.
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

#[cfg(test)]
mod tests {
    use super::*;

    /// The vocabulary is only closed if something checks the whole of it.
    ///
    /// A table of eighteen arms that nothing reads is eighteen chances for a
    /// typo to reach a caller who is matching on the string.
    #[test]
    fn every_code_is_namespaced_distinct_and_published() {
        let published = include_str!("../docs/reference/error-codes.md");
        let mut seen: Vec<&str> = Vec::new();
        for code in ErrorCode::ALL {
            let text = code.as_str();
            let parts: Vec<&str> = text.split('.').collect();
            assert_eq!(parts.len(), 3, "{text} is not `tool.area.reason`");
            assert_eq!(parts[0], "rhino", "{text} does not name the tool first");
            assert!(!parts[1].is_empty() && !parts[2].is_empty(), "{text}");
            assert!(!seen.contains(&text), "{text} appears twice");
            assert!(
                published.contains(&format!("`{text}`")),
                "{text} is not in the published vocabulary"
            );
            seen.push(text);
        }
        // The other direction: a page that names a code the crate cannot emit
        // is as misleading as a code the page omits.
        for piece in published.split('`') {
            // `rhino.<area>.<reason>` is the shape, not a member.
            if piece.starts_with("rhino.") && !piece.contains('<') {
                assert!(
                    seen.contains(&piece),
                    "{piece} is published and unreachable"
                );
            }
        }
    }

    #[test]
    fn the_text_body_is_one_line_and_the_json_body_carries_the_code() {
        let text = body(Format::Text, ErrorCode::ArgsUnrecognized, "no such flag\n");
        assert_eq!(text, "rhino: no such flag\n");

        let json = body(Format::Json, ErrorCode::GateChildNotFound, "gate `a`: gone");
        assert_eq!(
            json,
            "{\"schemaVersion\":1,\"error\":{\"code\":\"rhino.gate.child-not-found\",\
             \"message\":\"gate `a`: gone\"}}\n"
        );
    }

    #[test]
    fn a_message_never_breaks_the_document_it_travels_in() {
        let json = body(
            Format::Json,
            ErrorCode::FileUnreadable,
            "a \"quoted\" \\ path\twith\r\nbreaks\u{1}",
        );
        assert!(json.contains("a \\\"quoted\\\" \\\\ path\\twith\\r\\nbreaks\\u0001"));
        assert_eq!(json.lines().count(), 1);
    }
}
