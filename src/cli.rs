//! The command line: what an invocation says, and whether it says it legally.
//!
//! Parsing is separated from doing so that "this invocation is not usable" is
//! decided before any file is opened. That ordering is what keeps exit `2`
//! meaning *the invocation, root, or configuration was unusable* and exit `1`
//! meaning *the repository violates its declared policy*, with nothing able to
//! borrow the other's code.
//!
//! The command tree is data. A leaf is a path plus the flags it accepts, so a
//! flag reaching a command that has no use for it is refused here rather than
//! ignored there -- a spelling mistake that silently narrowed nothing would
//! report a clean repository that was never fully checked.

use crate::scan::STDIN;
use std::collections::BTreeSet;
use std::fmt;

/// How a leaf renders its result.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum Format {
    #[default]
    Text,
    Json,
}

/// A flag a leaf is willing to be given.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum Accepts {
    /// Repeated paths to inspect instead of the declared surface. `-` is
    /// standard input.
    File,
    /// One directory to inspect instead of the declared trees.
    Directory,
    /// One harness to reconcile instead of the whole roster.
    Harness,
    /// The one surface a gate dispatch is selected by.
    Surface,
    /// `version --json`, which predates `--output` and is kept because a
    /// release script already reads it.
    Json,
}

/// One command the tool answers to.
pub struct Leaf {
    pub path: &'static [&'static str],
    /// The atomic output prefix. Stated rather than assembled from the path, so
    /// what a consumer greps for cannot change because a command was renamed.
    pub category: &'static str,
    pub accepts: &'static [Accepts],
    pub summary: &'static str,
}

pub const LEAVES: &[Leaf] = &[
    Leaf {
        path: &["repo-config", "validate"],
        category: "repo-config",
        accepts: &[],
        summary: "Check that repo-config.yml is complete and usable.",
    },
    Leaf {
        path: &["gate", "run"],
        category: "gate",
        accepts: &[Accepts::Surface],
        summary: "Run the gates the declared surface selects, in declaration order.",
    },
    Leaf {
        path: &["governance", "word-budget", "validate"],
        category: "word-budget",
        accepts: &[],
        summary: "Check every declared surface against its declared word budget.",
    },
    Leaf {
        path: &["governance", "directory-map", "validate"],
        category: "directory-map",
        accepts: &[Accepts::Directory],
        summary: "Check that every mapped tree's READMEs list their siblings.",
    },
    Leaf {
        path: &["harness", "parity", "validate"],
        category: "harness-parity",
        accepts: &[Accepts::Harness],
        summary: "Reconcile the canon against every declared coding harness.",
    },
    Leaf {
        path: &["md", "frontmatter", "validate"],
        category: "frontmatter",
        accepts: &[],
        summary: "Check every declared surface's front matter against its declared schema.",
    },
    Leaf {
        path: &["md", "heading-hierarchy", "validate"],
        category: "heading-hierarchy",
        accepts: &[],
        summary: "Check every declared surface's heading structure.",
    },
    Leaf {
        path: &["metadata", "validate"],
        category: "metadata",
        accepts: &[],
        summary: "Check every declared surface against its path-selected metadata schema.",
    },
    Leaf {
        path: &["md", "internal-link", "validate"],
        category: "internal-link",
        accepts: &[],
        summary: "Check that every local Markdown link resolves inside the repository.",
    },
    Leaf {
        path: &["md", "mermaid", "validate"],
        category: "mermaid",
        accepts: &[Accepts::File],
        summary: "Check Mermaid diagrams for label length and colour contrast.",
    },
    Leaf {
        path: &["md", "naming", "validate"],
        category: "naming",
        accepts: &[],
        summary: "Check every declared surface's filenames against its declared style.",
    },
    Leaf {
        path: &["md", "readme-index", "validate"],
        category: "readme-index",
        accepts: &[],
        summary: "Check that every directory in a declared tree carries a README.",
    },
    Leaf {
        path: &["md", "word-count", "inspect"],
        category: "word-count",
        accepts: &[Accepts::File],
        summary: "Report a file's word count. Never reports findings.",
    },
    Leaf {
        path: &["convention", "emoji", "validate"],
        category: "emoji",
        accepts: &[],
        summary: "Check that no declared file carries an emoji code point.",
    },
    Leaf {
        path: &["version"],
        category: "version",
        accepts: &[Accepts::Json],
        summary: "Report this build's release identity.",
    },
];

/// What one invocation asks for, once it is known to be askable.
#[derive(Debug, Clone, Default)]
pub struct Invocation {
    pub category: &'static str,
    pub root: Option<String>,
    pub format: Format,
    pub files: Vec<String>,
    pub directory: Option<String>,
    pub harness: Option<String>,
    pub surface: Option<String>,
    /// Everything after `--`: the hook's own arguments, forwarded unmodified.
    pub forwarded: Vec<String>,
}

/// A request for help, which is answered rather than refused.
pub struct Help(pub String);

pub enum Parsed {
    Run(Box<Invocation>),
    Help(Help),
}

/// Why an invocation cannot be run.
///
/// Rendered without a category prefix, because the fault is in the command line
/// and there may be no command to attribute it to.
pub struct Refusal(pub String);

impl fmt::Display for Refusal {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(formatter, "rhino: {}", self.0)
    }
}

pub fn parse(arguments: &[String]) -> Result<Parsed, Refusal> {
    let mut path: Vec<String> = Vec::new();
    let mut wants_help = false;
    let mut root: Option<String> = None;
    let mut format: Option<String> = None;
    let mut files: Vec<String> = Vec::new();
    let mut directory: Option<String> = None;
    let mut harness: Option<String> = None;
    let mut surface: Option<String> = None;
    let mut forwarded: Vec<String> = Vec::new();
    let mut json_flag = false;

    let mut rest = arguments.iter();
    while let Some(argument) = rest.next() {
        // A flag may appear before or after the command path: a caller who
        // knows the root should not have to remember where the parser wants it.
        let value =
            |name: &str, held: &mut Option<String>, rest: &mut std::slice::Iter<'_, String>| {
                match rest.next() {
                    Some(value) => {
                        *held = Some(value.clone());
                        Ok(())
                    }
                    None => Err(Refusal(format!("`{name}` needs a value"))),
                }
            };

        match argument.as_str() {
            "--help" | "-h" => wants_help = true,
            "--root" => value("--root", &mut root, &mut rest)?,
            "--output" => value("--output", &mut format, &mut rest)?,
            "--directory" => value("--directory", &mut directory, &mut rest)?,
            "--harness" => value("--harness", &mut harness, &mut rest)?,
            "--surface" => value("--surface", &mut surface, &mut rest)?,
            // Everything past this point belongs to the child, not to RHINO. A
            // hook argument that happens to start with a hyphen is payload, and
            // a parser that read it as a flag would refuse the invocation Git
            // itself assembled.
            "--" => {
                forwarded.extend(rest.by_ref().cloned());
            }
            "--file" => {
                let mut held = None;
                value("--file", &mut held, &mut rest)?;
                files.extend(held);
            }
            "--json" => json_flag = true,
            // Accepted everywhere and acted on nowhere. Presentation is not
            // policy, and a caller scripting around this tool should not have
            // to know which leaf understands which of them.
            "--quiet" | "--verbose" | "--no-color" => {}
            other if other.starts_with('-') => {
                return Err(Refusal(format!("unrecognized option `{other}`")));
            }
            segment => path.push(segment.to_string()),
        }
    }

    let segments: Vec<&str> = path.iter().map(String::as_str).collect();

    if wants_help {
        // Answered for any prefix of the tree, because `rhino md --help` is how
        // a reader finds out what `md` contains.
        return match help_for(&segments) {
            Some(text) => Ok(Parsed::Help(Help(text))),
            None => Err(Refusal(unrecognized(&segments))),
        };
    }

    let Some(leaf) = LEAVES.iter().find(|leaf| leaf.path == segments.as_slice()) else {
        return Err(Refusal(unrecognized(&segments)));
    };

    let format_given = format.is_some();
    let format = match format.as_deref() {
        None | Some("text") => Format::Text,
        Some("json") => Format::Json,
        Some(other) => {
            return Err(Refusal(format!(
                "`--output {other}` is not a format this build writes; expected `text` or `json`"
            )));
        }
    };

    // A flag the leaf has no use for is refused rather than dropped. Silently
    // ignoring `--harness` on the word-budget leaf would report a whole-tree
    // result to a caller who asked for a narrowed one.
    let accepted: BTreeSet<Accepts> = leaf.accepts.iter().copied().collect();
    for (given, flag, name) in [
        (!files.is_empty(), Accepts::File, "--file"),
        (directory.is_some(), Accepts::Directory, "--directory"),
        (harness.is_some(), Accepts::Harness, "--harness"),
        (surface.is_some(), Accepts::Surface, "--surface"),
        (json_flag, Accepts::Json, "--json"),
    ] {
        if given && !accepted.contains(&flag) {
            return Err(Refusal(format!(
                "`{name}` is not accepted by `{}`",
                leaf.path.join(" ")
            )));
        }
    }

    // A selected directory is repository-relative by construction. An empty,
    // absolute, or escaping value is a fault in the command line rather than a
    // repository with nothing in it, and reporting it as the latter would print
    // a clean result for a directory nobody inspected.
    if let Some(selected) = &directory
        && let Some(reason) = not_repository_relative(selected)
    {
        return Err(Refusal(format!("`--directory {selected}` {reason}")));
    }
    // The same rule for `--file`, and for the same reason. An absolute path
    // silently reinterpreted as a repository-relative one inspects a file the
    // caller did not name.
    for selected in &files {
        if selected != STDIN
            && let Some(reason) = not_repository_relative(selected)
        {
            return Err(Refusal(format!("`--file {selected}` {reason}")));
        }
    }
    // A contradiction is refused rather than resolved by declaration order,
    // which would make the same two flags mean different things depending on
    // how a script happened to assemble them.
    if json_flag && format_given && format == Format::Text {
        return Err(Refusal(
            "`--json` and `--output text` ask for different things".to_string(),
        ));
    }

    Ok(Parsed::Run(Box::new(Invocation {
        category: leaf.category,
        root,
        // `version --json` predates `--output` and means the same thing.
        format: if json_flag { Format::Json } else { format },
        files,
        directory,
        harness,
        surface,
        forwarded,
    })))
}

fn unrecognized(segments: &[&str]) -> String {
    if segments.is_empty() {
        "no command given; try `rhino --help`".to_string()
    } else {
        format!("unrecognized command `{}`", segments.join(" "))
    }
}

/// Usage for a path, or `None` when the path names nothing.
///
/// Built from the same table the dispatcher reads, so a leaf cannot exist
/// without being documented or be documented without existing.
fn help_for(segments: &[&str]) -> Option<String> {
    let matching: Vec<&Leaf> = LEAVES
        .iter()
        .filter(|leaf| leaf.path.starts_with(segments))
        .collect();
    if matching.is_empty() {
        return None;
    }

    let mut text = String::new();
    if segments.is_empty() {
        text.push_str("rhino -- Repository Hygiene & INtegration Orchestrator\n\n");
        text.push_str(
            "Every value it enforces is declared in the repository's repo-config.yml.\n\n",
        );
    } else {
        text.push_str(&format!("rhino {}\n\n", segments.join(" ")));
    }
    text.push_str("Commands:\n");
    for leaf in &matching {
        text.push_str(&format!(
            "  rhino {:<38}{}\n",
            leaf.path.join(" "),
            leaf.summary
        ));
    }
    text.push_str("\nOptions:\n");
    text.push_str(
        "  --root <path>          The repository to inspect. Defaults to the working directory.\n",
    );
    text.push_str("  --output <text|json>   How to render the result. Defaults to text.\n");
    text.push_str("  --quiet, --verbose, --no-color\n");
    text.push_str("                         Accepted everywhere; presentation only.\n");

    let flags: BTreeSet<Accepts> = matching
        .iter()
        .flat_map(|leaf| leaf.accepts.iter().copied())
        .collect();
    for flag in flags {
        text.push_str(match flag {
            Accepts::File => {
                "  --file <path>          Inspect these paths instead of the declared surface.\n                         Repeatable; `-` reads standard input.\n"
            }
            Accepts::Directory => {
                "  --directory <path>     Inspect this directory instead of the declared trees.\n"
            }
            Accepts::Harness => {
                "  --harness <name>       Reconcile this harness instead of the whole roster.\n"
            }
            Accepts::Surface => {
                "  --surface <name>       Which surface is dispatching: commit-msg, pre-commit,\n                         pre-push, or ci. Arguments after `--` reach every child.\n"
            }
            Accepts::Json => "  --json                 Shorthand for --output json.\n",
        });
    }
    text.push_str("\nExit codes:\n  0  checked and clean\n  1  the repository violates its declared policy\n  2  the invocation, root, or configuration was unusable\n  3  a gate child could not be started\n");
    Some(text)
}

/// Why a value cannot address something inside the repository.
fn not_repository_relative(value: &str) -> Option<&'static str> {
    if value.trim().is_empty() {
        return Some("names nothing");
    }
    if value.starts_with('/') {
        return Some("is absolute, and a selection is relative to the repository root");
    }
    let mut depth: isize = 0;
    for segment in value.split('/') {
        match segment {
            ".." => depth -= 1,
            "." | "" => {}
            _ => depth += 1,
        }
        if depth < 0 {
            return Some("escapes the repository root");
        }
    }
    None
}
