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

use crate::errors::ErrorCode;
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
    /// An explicit repository-relative destination for a backup plan.
    BackupDirectory,
    /// Authorize a declared mutation after its default plan has been reviewed.
    Apply,
    /// Replace an existing target after the operation has planned its backup.
    Force,
    /// The one surface a gate dispatch is selected by.
    Surface,
    /// The exact path Git handed to the commit-msg hook.
    MessageFile,
    /// Read Git's pre-push update records from standard input.
    PushUpdatesStdin,
    /// The explicit immutable base/head range for a pull-request run.
    Range,
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
        accepts: &[
            Accepts::Surface,
            Accepts::MessageFile,
            Accepts::PushUpdatesStdin,
            Accepts::Range,
        ],
        summary: "Run the gates the declared surface selects, in declaration order.",
    },
    Leaf {
        path: &["gate", "list"],
        category: "gate-list",
        accepts: &[],
        summary: "List the declared v0.4 lifecycle gates for each closed surface.",
    },
    Leaf {
        path: &["gate", "validate"],
        category: "gate-validate",
        accepts: &[],
        summary: "Validate v0.4 lifecycle membership and pull-request composition.",
    },
    Leaf {
        path: &["governance", "vendor", "validate"],
        category: "vendor",
        accepts: &[],
        summary: "Check declared roots for declared vendor terms and exact exceptions.",
    },
    Leaf {
        path: &["governance", "layers", "validate"],
        category: "layers",
        accepts: &[],
        summary: "Check the declared governance layer and category structure.",
    },
    Leaf {
        path: &["governance", "traceability", "validate"],
        category: "traceability",
        accepts: &[],
        summary: "Check declared artifacts and their declared local links.",
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
        path: &["harness", "adapters", "validate"],
        category: "harness-adapters-validate",
        accepts: &[],
        summary: "Validate canonical harness adapters without writing them.",
    },
    Leaf {
        path: &["harness", "adapters", "generate"],
        category: "harness-adapters-generate",
        accepts: &[],
        summary: "Generate canonical harness adapters in one declared transaction.",
    },
    Leaf {
        path: &["env", "backup"],
        category: "environment-backup",
        accepts: &[Accepts::BackupDirectory],
        summary: "Copy declared environment targets into an explicit backup destination.",
    },
    Leaf {
        path: &["env", "validate"],
        category: "environment-validate",
        accepts: &[],
        summary: "Validate declared environment policy without reporting values.",
    },
    Leaf {
        path: &["env", "init"],
        category: "environment-init",
        accepts: &[Accepts::Apply],
        summary: "Initialize declared environment targets only with explicit authorization.",
    },
    Leaf {
        path: &["env", "restore"],
        category: "environment-restore",
        accepts: &[Accepts::BackupDirectory, Accepts::Force],
        summary: "Restore declared environment targets from an explicit backup directory.",
    },
    Leaf {
        path: &["toolchain", "validate"],
        category: "toolchain-validate",
        accepts: &[],
        summary: "Probe declared toolchains without installing or reporting their output.",
    },
    Leaf {
        path: &["toolchain", "provision"],
        category: "toolchain-provision",
        accepts: &[Accepts::Apply],
        summary: "Provision declared toolchains only with explicit authorization.",
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
        path: &["convention", "emoji", "validate"],
        category: "emoji",
        accepts: &[],
        summary: "Check that no declared file carries an emoji code point.",
    },
    Leaf {
        path: &["convention", "license", "validate"],
        category: "license",
        accepts: &[],
        summary: "Check configured license paths, identifiers, and digests.",
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
    pub backup_directory: Option<String>,
    pub surface: Option<String>,
    pub message_file: Option<String>,
    pub push_updates_stdin: bool,
    pub base: Option<String>,
    pub head: Option<String>,
    pub apply: bool,
    pub force: bool,
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
#[derive(Debug)]
pub struct Refusal {
    pub code: ErrorCode,
    pub message: String,
    /// The output format as far as the parse got before refusing.
    ///
    /// Carried because a caller who asked for JSON asked for it about the whole
    /// run, including the part where the run does not happen. A refusal
    /// rendered as prose to a caller that was promised a document is a parse
    /// error in the caller rather than a diagnostic.
    pub format: Format,
}

impl Refusal {
    /// A refusal from inside the scan, before the command line has been read to
    /// the end.
    ///
    /// The format is whatever has been seen so far. A caller who wrote
    /// `--output json` before the mistake asked for a document and gets one; a
    /// caller who wrote it after gets prose, which is the most that can be
    /// known at the point the scan stops.
    fn new(seen: PartialFormat, code: ErrorCode, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
            format: seen.resolve(),
        }
    }
}

impl fmt::Display for Refusal {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(formatter, "rhino: {}", self.message)
    }
}

pub fn parse(arguments: &[String]) -> Result<Parsed, Refusal> {
    let mut path: Vec<String> = Vec::new();
    let mut wants_help = false;
    let mut wants_version = false;
    let mut root: Option<String> = None;
    let mut format: Option<String> = None;
    let mut files: Vec<String> = Vec::new();
    let mut directory: Option<String> = None;
    let mut backup_directory: Option<String> = None;
    let mut surface: Option<String> = None;
    let mut message_file: Option<String> = None;
    let mut push_updates_stdin = false;
    let mut base: Option<String> = None;
    let mut head: Option<String> = None;
    let mut apply = false;
    let mut force = false;
    let mut json_flag = false;

    let mut rest = arguments.iter();
    while let Some(argument) = rest.next() {
        // A flag may appear before or after the command path: a caller who
        // knows the root should not have to remember where the parser wants it.
        //
        // `seen` is resolved by the caller rather than read here, because at
        // the `--output` site the format being learned is the very slot this
        // closure writes to. A caller who already said `--output json` and then
        // left a later flag bare is owed the document it asked for.
        let value = |name: &str,
                     held: &mut Option<String>,
                     rest: &mut std::slice::Iter<'_, String>,
                     seen: Format| {
            match rest.next() {
                Some(value) => {
                    *held = Some(value.clone());
                    Ok(())
                }
                None => Err(Refusal {
                    code: ErrorCode::ArgsIncomplete,
                    message: format!("`{name}` needs a value"),
                    format: seen,
                }),
            }
        };

        // Whatever the scan has learned about the format so far, resolved
        // before the match so the `--output` arm may take `&mut format`.
        let seen = PartialFormat {
            output: format.as_ref(),
            json_flag,
        }
        .resolve();

        match argument.as_str() {
            "--help" | "-h" => wants_help = true,
            "--version" | "-V" => wants_version = true,
            "--root" => value("--root", &mut root, &mut rest, seen)?,
            "--output" => value("--output", &mut format, &mut rest, seen)?,
            "--directory" => value("--directory", &mut directory, &mut rest, seen)?,
            "--dir" => value("--dir", &mut backup_directory, &mut rest, seen)?,
            "--surface" => value("--surface", &mut surface, &mut rest, seen)?,
            "--message-file" => value("--message-file", &mut message_file, &mut rest, seen)?,
            "--push-updates-stdin" => push_updates_stdin = true,
            "--base" => value("--base", &mut base, &mut rest, seen)?,
            "--head" => value("--head", &mut head, &mut rest, seen)?,
            "--apply" => apply = true,
            "--force" => force = true,
            // The end of the options, which is what `--` means everywhere else
            // a shell is involved: everything after it is an operand, whatever
            // it starts with.
            //
            // It used to mean "forward the rest to a child". Nothing ever
            // consumed those arguments -- the one command that could have
            // refused them outright -- and no caller passed any, so the feature
            // existed only to make `--` mean something a caller would not
            // expect.
            "--" => {
                path.extend(rest.by_ref().cloned());
            }
            "--file" => {
                let mut held = None;
                value("--file", &mut held, &mut rest, seen)?;
                files.extend(held);
            }
            "--json" => json_flag = true,
            // Accepted everywhere and acted on nowhere. Presentation is not
            // policy, and a caller scripting around this tool should not have
            // to know which leaf understands which of them.
            "--quiet" | "--verbose" | "--no-color" => {}
            other if other.starts_with('-') => {
                return Err(Refusal::new(
                    PartialFormat {
                        output: format.as_ref(),
                        json_flag,
                    },
                    ErrorCode::ArgsUnrecognized,
                    format!("unrecognized option `{other}`"),
                ));
            }
            segment => path.push(segment.to_string()),
        }
    }

    // `help` is a command as well as a flag, because a caller reaching for one
    // of the two spellings has no way to know which this tool chose, and being
    // told `unrecognized command \`help\`` is a usage mistake reported to
    // someone in the middle of asking how to avoid one.
    if path.first().is_some_and(|first| first == "help") {
        path.remove(0);
        wants_help = true;
    }

    // Answered as the leaf it duplicates rather than as a special case, so the
    // flag and the command cannot drift apart.
    if wants_version && !wants_help {
        path = vec!["version".to_string()];
    }

    let segments: Vec<&str> = path.iter().map(String::as_str).collect();

    if wants_help {
        // Answered for any prefix of the tree, because `rhino md --help` is how
        // a reader finds out what `md` contains.
        return match help_for(&segments) {
            Some(text) => Ok(Parsed::Help(Help(text))),
            None => Err(Refusal::new(
                PartialFormat {
                    output: format.as_ref(),
                    json_flag,
                },
                ErrorCode::ArgsUnrecognized,
                unrecognized(&segments),
            )),
        };
    }

    let Some(leaf) = LEAVES.iter().find(|leaf| leaf.path == segments.as_slice()) else {
        return Err(Refusal::new(
            PartialFormat {
                output: format.as_ref(),
                json_flag,
            },
            ErrorCode::ArgsUnrecognized,
            unrecognized(&segments),
        ));
    };

    let format_given = format.is_some();
    let format = match format.as_deref() {
        None | Some("text") => Format::Text,
        Some("json") => Format::Json,
        Some(other) => {
            return Err(Refusal::new(
                PartialFormat {
                    output: None,
                    json_flag,
                },
                ErrorCode::ArgsUnrecognized,
                format!(
                    "`--output {other}` is not a format this build writes; expected `text` or `json`"
                ),
            ));
        }
    };

    // A flag the leaf has no use for is refused rather than dropped. Silently
    // ignoring a narrowed request would report a whole-tree result to a caller
    // who asked for a narrower one.
    let accepted: BTreeSet<Accepts> = leaf.accepts.iter().copied().collect();
    for (given, flag, name) in [
        (!files.is_empty(), Accepts::File, "--file"),
        (directory.is_some(), Accepts::Directory, "--directory"),
        (
            backup_directory.is_some(),
            Accepts::BackupDirectory,
            "--dir",
        ),
        (surface.is_some(), Accepts::Surface, "--surface"),
        (
            message_file.is_some(),
            Accepts::MessageFile,
            "--message-file",
        ),
        (
            push_updates_stdin,
            Accepts::PushUpdatesStdin,
            "--push-updates-stdin",
        ),
        (
            base.is_some() || head.is_some(),
            Accepts::Range,
            "--base/--head",
        ),
        (json_flag, Accepts::Json, "--json"),
        (apply, Accepts::Apply, "--apply"),
        (force, Accepts::Force, "--force"),
    ] {
        if given && !accepted.contains(&flag) {
            return Err(refusing(
                format,
                ErrorCode::ArgsUnrecognized,
                format!("`{name}` is not accepted by `{}`", leaf.path.join(" ")),
            ));
        }
    }

    // A selected directory is repository-relative by construction. An empty,
    // absolute, or escaping value is a fault in the command line rather than a
    // repository with nothing in it, and reporting it as the latter would print
    // a clean result for a directory nobody inspected.
    if let Some(selected) = &directory
        && let Some(reason) = not_repository_relative(selected)
    {
        return Err(refusing(
            format,
            ErrorCode::PathEscapesRoot,
            format!("`--directory {selected}` {reason}"),
        ));
    }
    // The same rule for `--file`, and for the same reason. An absolute path
    // silently reinterpreted as a repository-relative one inspects a file the
    // caller did not name.
    for selected in &files {
        if selected != STDIN
            && let Some(reason) = not_repository_relative(selected)
        {
            return Err(refusing(
                format,
                ErrorCode::PathEscapesRoot,
                format!("`--file {selected}` {reason}"),
            ));
        }
    }
    // A contradiction is refused rather than resolved by declaration order,
    // which would make the same two flags mean different things depending on
    // how a script happened to assemble them.
    if json_flag && format_given && format == Format::Text {
        return Err(refusing(
            format,
            ErrorCode::ArgsIncomplete,
            "`--json` and `--output text` ask for different things",
        ));
    }
    if base.is_some() != head.is_some() {
        return Err(refusing(
            format,
            ErrorCode::ArgsIncomplete,
            "`--base` and `--head` must be supplied together",
        ));
    }
    if leaf.category == "gate" {
        let selected = surface.as_deref();
        if message_file.is_some() && selected != Some("commit-msg") {
            return Err(refusing(
                format,
                ErrorCode::ArgsIncomplete,
                "`--message-file` is accepted only with `--surface commit-msg`",
            ));
        }
        if push_updates_stdin && selected != Some("pre-push") {
            return Err(refusing(
                format,
                ErrorCode::ArgsIncomplete,
                "`--push-updates-stdin` is accepted only with `--surface pre-push`",
            ));
        }
        if base.is_some() && selected != Some("pull-request") {
            return Err(refusing(
                format,
                ErrorCode::ArgsIncomplete,
                "`--base` and `--head` are accepted only with `--surface pull-request`",
            ));
        }
    }

    Ok(Parsed::Run(Box::new(Invocation {
        category: leaf.category,
        root,
        // `version --json` predates `--output` and means the same thing.
        format: if json_flag { Format::Json } else { format },
        files,
        directory,
        backup_directory,
        surface,
        message_file,
        push_updates_stdin,
        base,
        head,
        apply,
        force,
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
    text.push_str("  -h, --help             Show this help and exit.\n");
    text.push_str("  -V, --version          Show the release identity and exit.\n");
    text.push_str("  --quiet, --verbose, --no-color\n");
    text.push_str("                         Accepted everywhere; presentation only.\n");

    let flags: BTreeSet<Accepts> = matching
        .iter()
        .flat_map(|leaf| leaf.accepts.iter().copied())
        .collect();
    for flag in flags {
        // This block is the union of every matching leaf's options, which reads
        // as a set the tool accepts everywhere. That reading is how `--json`
        // came to be advertised globally while only `version` accepts it, and a
        // caller who believed the help got exit 2 for their trouble. Where a
        // flag is narrower than the block it appears in, the block says so.
        text.push_str(match flag {
            Accepts::File => {
                "  --file <path>          Inspect these paths instead of the declared surface.\n                         Repeatable; `-` reads standard input.\n"
            }
            Accepts::Directory => {
                "  --directory <path>     Inspect this directory instead of the declared trees.\n"
            }
            Accepts::BackupDirectory => {
                "  --dir <path>           Explicit repository-relative backup destination.\n"
            }
            Accepts::Apply => {
                "  --apply                Authorize this declared mutation.\n"
            }
            Accepts::Force => {
                "  --force                Replace an existing target after backup planning.\n"
            }
            Accepts::Surface => {
                "  --surface <name>       Which closed lifecycle surface is dispatching.\n"
            }
            Accepts::MessageFile => {
                "  --message-file <path>  Exact commit-msg hook path, read only for that surface.\n"
            }
            Accepts::PushUpdatesStdin => {
                "  --push-updates-stdin   Read Git pre-push update records from standard input.\n"
            }
            Accepts::Range => {
                "  --base <sha> --head <sha>\n                         Explicit immutable pull-request range.\n"
            }
            Accepts::Json => "  --json                 Shorthand for --output json.\n",
        });
        text.push_str(&scope_note(&matching, flag));
    }
    text.push_str(
        "\nExit codes:\n  \
         0    checked and clean\n  \
         1    the repository violates its declared policy\n  \
         2    the invocation, root, or configuration was unusable\n  \
         126  a gate child was found and could not be executed\n  \
         127  a gate child was not found\n  \
         128+N  ended by signal N; 130 is an interrupt, 141 a closed pipe\n",
    );
    Some(text)
}

/// What the scan has learned about the requested format so far.
///
/// Both spellings count. `--json` is a documented shorthand for
/// `--output json`, so a caller who used the shorthand and then made a mistake
/// is owed the same document as one who used the long form.
#[derive(Clone, Copy)]
struct PartialFormat<'a> {
    output: Option<&'a String>,
    json_flag: bool,
}

impl PartialFormat<'_> {
    fn resolve(self) -> Format {
        if self.json_flag || self.output.map(String::as_str) == Some("json") {
            Format::Json
        } else {
            Format::Text
        }
    }
}

/// A refusal that knows which format the caller asked for.
///
/// Separate from [`Refusal::new`] because the format is only known after the
/// whole command line has been scanned, and half the refusals happen before
/// that point.
fn refusing(format: Format, code: ErrorCode, message: impl Into<String>) -> Refusal {
    Refusal {
        code,
        message: message.into(),
        format,
    }
}

/// One line naming which leaves accept a flag, or nothing when they all do.
fn scope_note(matching: &[&Leaf], flag: Accepts) -> String {
    let accepting: Vec<String> = matching
        .iter()
        .filter(|leaf| leaf.accepts.contains(&flag))
        .map(|leaf| format!("`{}`", leaf.path.join(" ")))
        .collect();
    // Nothing to warn about when every command in the block takes it.
    if accepting.len() == matching.len() {
        return String::new();
    }
    format!("  (accepted by {} only)\n", accepting.join(", "))
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

#[cfg(test)]
mod tests {
    use super::*;

    fn arguments(words: &[&str]) -> Vec<String> {
        words.iter().map(|word| (*word).to_string()).collect()
    }

    /// Three spellings of the same two questions.
    ///
    /// `--help`, `-h`, and `help` all ask what this tool does; `--version` and
    /// `-V` all ask which one this is. A caller should not have to discover
    /// which spelling this particular tool chose.
    #[test]
    fn help_and_version_answer_to_every_published_spelling() {
        for words in [
            &["--help"][..],
            &["-h"][..],
            &["help"][..],
            &["gate", "--help"][..],
            &["help", "gate"][..],
        ] {
            assert!(matches!(parse(&arguments(words)), Ok(Parsed::Help(_))));
        }
        for words in [&["--version"][..], &["-V"][..], &["version"][..]] {
            assert!(matches!(
                parse(&arguments(words)),
                Ok(Parsed::Run(invocation)) if invocation.category == "version"
            ));
        }
    }

    /// The help block says which commands accept a flag, and stays short.
    ///
    /// A note listing every command is noise, and a note listing twenty is
    /// worse than none: both leave a reader scanning names they did not ask
    /// about.
    #[test]
    fn a_help_block_scopes_a_flag_without_reciting_the_whole_tree() {
        let all: Vec<&Leaf> = LEAVES.iter().collect();
        let accepting = |flag: Accepts| -> Vec<&Leaf> {
            all.iter()
                .copied()
                .filter(|leaf| leaf.accepts.contains(&flag))
                .collect()
        };

        // Every leaf in the slice accepts it, so there is nothing to warn about.
        assert_eq!(
            scope_note(&accepting(Accepts::Json), Accepts::Json),
            String::new()
        );

        // One leaf out of the whole tree: named outright.
        assert_eq!(accepting(Accepts::Json).len(), 1);
        assert!(scope_note(&all, Accepts::Json).contains("accepted by `version` only"));

        // Two out of many: both named, because no flag in this tree is taken
        // by enough commands for a list to be worse than a count.
        assert!(scope_note(&all, Accepts::Apply).contains(", "));
    }

    /// A caller who asked for a document gets one even when refused.
    #[test]
    fn a_refusal_carries_the_format_the_caller_had_already_asked_for() {
        for words in [
            &["--output", "json", "--no-such-flag"][..],
            &["--json", "--no-such-flag"][..],
            &["--output", "json", "--root"][..],
        ] {
            assert!(matches!(
                parse(&arguments(words)),
                Err(refusal) if refusal.format == Format::Json
            ));
        }

        // Nothing was said before the mistake, so prose is the most that can be
        // known at the point the scan stops.
        let refusal = parse(&arguments(&["--output"])).err().expect("it refuses");
        assert_eq!(refusal.format, Format::Text);
        assert_eq!(refusal.to_string(), "rhino: `--output` needs a value");
    }

    #[test]
    fn gate_flag_contract_refuses_cross_surface_and_incomplete_range_combinations() {
        for invocation in [
            &[
                "gate",
                "run",
                "--surface",
                "pull-request",
                "--base",
                "abcdef1",
            ][..],
            &[
                "gate",
                "run",
                "--surface",
                "pre-commit",
                "--message-file",
                "message",
            ],
            &[
                "gate",
                "run",
                "--surface",
                "pre-commit",
                "--push-updates-stdin",
            ],
            &[
                "gate",
                "run",
                "--surface",
                "pre-commit",
                "--base",
                "abcdef1",
                "--head",
                "abcdef2",
            ],
        ] {
            assert!(parse(&arguments(invocation)).is_err());
        }
    }

    #[test]
    fn grouped_adapter_leaves_refuse_the_retired_harness_selector() {
        for leaf in ["validate", "generate"] {
            let refusal = parse(&arguments(&[
                "harness",
                "adapters",
                leaf,
                "--harness",
                "alpha",
            ]))
            .err()
            .expect("grouped adapters do not select one profile");
            assert!(refusal.message.contains("--harness"));
        }
    }

    #[test]
    fn parser_covers_help_projection_and_every_flag_family() {
        assert!(matches!(
            parse(&arguments(&["--help"])),
            Ok(Parsed::Help(Help(text))) if text.contains("repo-config validate")
        ));
        assert!(matches!(
            parse(&arguments(&["gate", "--help"])),
            Ok(Parsed::Help(Help(text))) if text.contains("--surface <name>")
        ));

        let invocation = parse(&arguments(&[
            "gate",
            "run",
            "--root",
            "repo",
            "--output",
            "json",
            "--surface",
            "pull-request",
            "--base",
            "base",
            "--head",
            "head",
            "--quiet",
        ]))
        .expect("valid pull-request gate");
        assert!(matches!(
            invocation,
            Parsed::Run(invocation)
                if invocation.category == "gate"
                    && invocation.format == Format::Json
        ));

        // `--` ends the options. The operand after it reaches the command path
        // rather than being read as a flag, which is the whole of the rule.
        let after_double_dash = parse(&arguments(&["--", "version"])).expect("valid version leaf");
        assert!(matches!(
            after_double_dash,
            Parsed::Run(invocation) if invocation.category == "version"
        ));

        let directory = parse(&arguments(&[
            "governance",
            "directory-map",
            "validate",
            "--directory",
            "docs",
        ]))
        .expect("valid directory selection");
        assert!(matches!(
            directory,
            Parsed::Run(invocation) if invocation.directory.as_deref() == Some("docs")
        ));

        let mermaid = parse(&arguments(&[
            "md",
            "mermaid",
            "validate",
            "--file",
            "docs/a.md",
            "--file",
            "-",
        ]))
        .expect("valid file selection");
        assert!(matches!(
            mermaid,
            Parsed::Run(invocation) if invocation.files == vec!["docs/a.md", "-"]
        ));

        let restore = parse(&arguments(&[
            "env", "restore", "--dir", "backup", "--force",
        ]))
        .expect("valid restore flags");
        assert!(matches!(
            restore,
            Parsed::Run(invocation) if invocation.force && !invocation.apply
        ));
        let initialization = parse(&arguments(&["env", "init", "--apply"]))
            .expect("valid initialization authorization");
        assert!(matches!(
            initialization,
            Parsed::Run(invocation) if invocation.apply
        ));
    }

    #[test]
    fn parser_refuses_unknown_incompatible_and_escaping_input_without_guessing() {
        for invocation in [
            &["unknown"][..],
            &["--help", "unknown"],
            &["repo-config", "validate", "--output", "xml"],
            &["repo-config", "validate", "--file", "docs/a.md"],
            &["md", "mermaid", "validate", "--file", "/tmp/a.md"],
            &[
                "governance",
                "directory-map",
                "validate",
                "--directory",
                "../outside",
            ],
            &["version", "--json", "--output", "text"],
            &["env", "restore", "--dir"],
            &["version", "--unknown"],
        ] {
            assert!(
                parse(&arguments(invocation)).is_err(),
                "{}",
                invocation.join(" ")
            );
        }
        assert_eq!(not_repository_relative(""), Some("names nothing"));
        assert!(not_repository_relative("/absolute").is_some());
        assert!(not_repository_relative("a/../../outside").is_some());
        assert!(not_repository_relative("docs/./a").is_none());
        assert_eq!(unrecognized(&[]), "no command given; try `rhino --help`");
        assert_eq!(
            unrecognized(&["not", "a", "command"]),
            "unrecognized command `not a command`"
        );
    }
}
