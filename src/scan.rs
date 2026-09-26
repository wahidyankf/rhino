//! Choosing what to look at, and reading it once.
//!
//! Every validator that walks the repository walks it the same way, so the
//! answer to "was this file inspected?" does not depend on which command asked.
//! The exclusion list is the consuming repository's, matched by directory
//! *name* at any depth rather than by prefix, because a build output directory
//! is a build output directory wherever it appears.
//!
//! Reading is centralised for a second reason beyond cost: an unreadable file
//! has to refuse the whole run with exit `2`, and one implementation of that
//! refusal is one place for it to be right. A validator that quietly skipped a
//! file it could not open would report a clean repository it never read.

use crate::config::Config;
use crate::errors::ErrorCode;
use crate::report::Report;
use crate::runtime::{LinkTarget, Tree, TreeError};
use globset::{GlobBuilder, GlobSet, GlobSetBuilder};
use std::collections::BTreeSet;

/// One Markdown file, read.
pub struct Document {
    pub path: String,
    pub text: String,
}

/// Every Markdown file a validator may look at, read once.
pub struct Corpus {
    documents: Vec<Document>,
}

impl Corpus {
    /// Read the repository's Markdown, in sorted path order.
    ///
    /// Order is part of the contract rather than an accident of the walk:
    /// findings are reported in the order files are read, and a report whose
    /// order changed between runs could not be diffed.
    pub fn read(tree: &dyn Tree, config: &Config) -> Result<Self, String> {
        let mut documents = Vec::new();

        for path in markdown_files(tree, config) {
            match tree.read(&path) {
                Ok(text) => documents.push(Document { path, text }),
                // A file that vanished between the walk and the read is not
                // this repository's policy being violated.
                Err(TreeError::NotFound) => {}
                Err(TreeError::Unreadable(reason)) => return Err(format!("{path}: {reason}")),
                // Refusal follows the claim. This walk selected the file
                // because its name says Markdown; a name that says text over
                // bytes that are not is a fault to report, not to work around.
                Err(TreeError::NotText) => return Err(format!("{path}: holds no text")),
            }
        }

        Ok(Self { documents })
    }

    /// The repository's Markdown, plus every Markdown file a declared surface
    /// reaches through a link whose target stays inside the root.
    ///
    /// A followed file is named by the path the surface matched and read from
    /// the path its bytes live at, so a finding points where the repository
    /// declared it would.
    pub fn reached(
        tree: &dyn Tree,
        config: &Config,
        surfaces: &Surfaces,
    ) -> Result<Self, Unreachable> {
        let mut documents = Vec::new();

        for reached in surfaces.files(tree, config)? {
            if !is_markdown(&reached.path) {
                continue;
            }
            match tree.read(&reached.source) {
                Ok(text) => documents.push(Document {
                    path: reached.path,
                    text,
                }),
                Err(TreeError::NotFound) => {}
                Err(TreeError::Unreadable(reason)) => {
                    return Err(Unreachable::unreadable(format!(
                        "{}: {reason}",
                        reached.path
                    )));
                }
                Err(TreeError::NotText) => {
                    return Err(Unreachable::unreadable(format!(
                        "{}: holds no text",
                        reached.path
                    )));
                }
            }
        }

        Ok(Self { documents })
    }

    /// The documents, owned, for a caller that has to hold them beside a
    /// selection built some other way.
    pub fn into_documents(self) -> Vec<Document> {
        self.documents
    }

    pub fn documents(&self) -> &[Document] {
        &self.documents
    }

    /// The documents a validator treats as sources, given what it excludes.
    ///
    /// Exclusion is a per-validator question and never removes a file from the
    /// corpus: a document excluded as a *source* is still a perfectly valid
    /// link target and still a file another validator may inspect.
    pub fn sources<'a>(&'a self, excluded: &'a GlobSet) -> impl Iterator<Item = &'a Document> {
        self.documents
            .iter()
            .filter(move |document| !excluded.is_match(&document.path))
    }
}

/// Every path a validator may look at.
///
/// Filesystem links are already absent: the port never reports one, because
/// following one can leave the repository.
pub fn files(tree: &dyn Tree, config: &Config) -> Vec<String> {
    tree.files()
        .into_iter()
        .filter(|path| !is_excluded(path, config.excluded()))
        .collect()
}

/// Every Markdown path a validator may look at.
pub fn markdown_files(tree: &dyn Tree, config: &Config) -> Vec<String> {
    tree.files()
        .into_iter()
        .filter(|path| is_markdown(path))
        .filter(|path| !is_excluded(path, config.excluded()))
        .collect()
}

/// Whether a path names a Markdown file.
///
/// Case-insensitive on the extension, because `README.MD` is a Markdown file
/// that a repository will eventually contain and that a reader would be
/// surprised to see skipped.
pub fn is_markdown(path: &str) -> bool {
    path.rsplit_once('.')
        .is_some_and(|(_, extension)| extension.eq_ignore_ascii_case("md"))
}

/// Compile a declared list of globs, naming the configuration key in any
/// failure so a bad pattern is a configuration fault a reader can locate.
///
/// Matching is case-insensitive for the same reason the extension check is: a
/// surface declared as `rules/**/*.md` is meant to cover the Markdown under
/// `rules`, and whether one file shouts its extension is not a policy decision
/// the repository made.
pub fn glob_set<'a>(
    key: &str,
    patterns: impl IntoIterator<Item = &'a str>,
) -> Result<GlobSet, String> {
    let mut builder = GlobSetBuilder::new();
    for pattern in patterns {
        // Every path a walk yields is repository-relative, so an absolute or
        // climbing pattern can never match: it would be a policy that quietly
        // governs nothing.
        if leaves_root(pattern) {
            return Err(format!(
                "{key}: `{pattern}` must be relative to the repository root"
            ));
        }
        let glob = GlobBuilder::new(pattern)
            .case_insensitive(true)
            .build()
            .map_err(|error| format!("{key}: `{pattern}`: {error}"))?;
        builder.add(glob);
    }
    builder.build().map_err(|error| format!("{key}: {error}"))
}

/// Whether a declared pattern or path is absolute or climbs out of the root.
pub fn leaves_root(pattern: &str) -> bool {
    pattern.starts_with('/') || pattern.split('/').any(|segment| segment == "..")
}

/// The file a directory documents itself in.
///
/// Named here rather than in each validator that looks for it, because two
/// already do and they have to be looking for the same file.
pub const README: &str = "README.md";

/// Every directory at or under a tree, in path order.
///
/// Derived from the files rather than from a directory listing, because the
/// port's read-only tree exposes files and a directory that holds nothing is
/// not a directory a README can be missing from.
pub fn directories(tree: &dyn Tree, config: &Config, root: &str) -> Vec<String> {
    let prefix = format!("{}/", root.trim_end_matches('/'));
    let mut found: BTreeSet<String> = BTreeSet::new();

    for path in tree.files() {
        if !path.starts_with(&prefix) {
            continue;
        }
        if is_excluded(&path, config.excluded()) {
            continue;
        }
        let mut segments: Vec<&str> = path.split('/').collect();
        segments.pop();
        while segments.len() >= prefix.matches('/').count() {
            found.insert(segments.join("/"));
            segments.pop();
        }
    }

    found.into_iter().collect()
}

/// An ordered list of declared surface globs.
///
/// Two sections already state the same rule -- ordered globs where the last
/// matching entry wins -- so the rule lives here once rather than in each of
/// them. A third section stating it the other way round would be a repository
/// policy that changed meaning depending on which command read it.
pub struct Surfaces {
    globs: GlobSet,
    patterns: Vec<String>,
}

/// One file a declared surface reaches.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Reached {
    /// The path the surface names it by, through any link it followed.
    pub path: String,
    /// Where its bytes are read: the same path unless a link was followed.
    pub source: String,
}

/// Why a declared surface could not be walked, as the refusal it becomes.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Unreachable {
    code: ErrorCode,
    message: String,
}

impl Unreachable {
    fn unreadable(message: String) -> Self {
        Self {
            code: ErrorCode::FileUnreadable,
            message,
        }
    }

    /// The refusal a leaf reports, with exit `2`.
    pub fn refusal(self, category: &'static str) -> Report {
        Report::refused_as(self.code, category, self.message)
    }
}

impl Surfaces {
    pub fn compile<'a>(
        key: &str,
        globs: impl IntoIterator<Item = &'a str>,
    ) -> Result<Self, String> {
        let patterns: Vec<String> = globs.into_iter().map(str::to_string).collect();
        glob_set(key, patterns.iter().map(String::as_str)).map(|globs| Self { globs, patterns })
    }

    /// Every file these surfaces may govern, in path order: each file the
    /// walk lists, and each governed file behind a link a surface reaches.
    ///
    /// A link a surface reaches is followed when its target stays inside the
    /// root. One that leads outside refuses with `rhino.path.escapes-root`,
    /// and one that loops or leads nowhere refuses with
    /// `rhino.file.unreadable`: a surface that silently read nothing there
    /// would report a clean policy it never checked. A link no surface
    /// reaches is neither followed nor refused, so an unrelated link cannot
    /// stop a run.
    pub fn files(&self, tree: &dyn Tree, config: &Config) -> Result<Vec<Reached>, Unreachable> {
        let walked = files(tree, config);
        let links: Vec<(String, LinkTarget)> = tree
            .links()
            .into_iter()
            .filter(|(link, _)| !is_excluded(&format!("{link}/"), config.excluded()))
            .collect();
        let mut reached: Vec<Reached> = walked
            .iter()
            .map(|path| Reached {
                path: path.clone(),
                source: path.clone(),
            })
            .collect();
        for (link, target) in &links {
            if self.reaches(link) {
                let mut chain = vec![link.as_str()];
                self.follow(
                    &walked,
                    &links,
                    config,
                    link,
                    target,
                    &mut chain,
                    &mut reached,
                )?;
            }
        }
        reached.sort_by(|left, right| left.path.cmp(&right.path));
        Ok(reached)
    }

    /// Follow one link from the path a surface reached it at.
    ///
    /// `chain` holds every link already followed on the way here. Meeting one
    /// of them again is a cycle, which would otherwise never end.
    #[allow(clippy::too_many_arguments)]
    fn follow<'a>(
        &self,
        walked: &[String],
        links: &'a [(String, LinkTarget)],
        config: &Config,
        at: &str,
        target: &LinkTarget,
        chain: &mut Vec<&'a str>,
        reached: &mut Vec<Reached>,
    ) -> Result<(), Unreachable> {
        let target = match target {
            LinkTarget::Within(target) => target,
            LinkTarget::Outside => {
                return Err(Unreachable {
                    code: ErrorCode::PathEscapesRoot,
                    message: format!(
                        "{at}: a declared surface reaches a symbolic link that leads outside the repository root"
                    ),
                });
            }
            LinkTarget::Unresolved => {
                return Err(Unreachable::unreadable(format!(
                    "{at}: a declared surface reaches a symbolic link whose target cannot be resolved"
                )));
            }
        };
        let prefix = normalise(target);
        let admit = |path: String, source: &str, reached: &mut Vec<Reached>| {
            if self.governing(&path).is_some() && !is_excluded(&path, config.excluded()) {
                reached.push(Reached {
                    path,
                    source: source.to_string(),
                });
            }
        };
        for source in walked {
            if source == target {
                admit(at.to_string(), source, reached);
            } else if let Some(rest) = source.strip_prefix(&prefix) {
                admit(format!("{at}/{rest}"), source, reached);
            }
        }
        for (link, next) in links {
            let Some(rest) = link.strip_prefix(&prefix) else {
                continue;
            };
            let path = format!("{at}/{rest}");
            if !self.reaches(&path) || is_excluded(&format!("{path}/"), config.excluded()) {
                continue;
            }
            if chain.contains(&link.as_str()) {
                return Err(Unreachable::unreadable(format!(
                    "{path}: a declared surface reaches a symbolic-link cycle"
                )));
            }
            chain.push(link);
            self.follow(walked, links, config, &path, next, chain, reached)?;
            chain.pop();
        }
        Ok(())
    }

    /// Whether any surface could match a path at or under `path`.
    ///
    /// Judged by each glob's literal prefix -- the segments before its first
    /// wildcard -- because a link's contents are unknown until it is
    /// followed. A glob with a wildcard reaches every path that agrees with
    /// its literal prefix as far as both go; a glob with none reaches only
    /// the path it names and the directories on the way to it.
    fn reaches(&self, path: &str) -> bool {
        let at: Vec<&str> = path.split('/').collect();
        self.patterns.iter().any(|pattern| {
            let segments: Vec<&str> = pattern.split('/').collect();
            let literal: Vec<&str> = segments
                .iter()
                .take_while(|segment| !segment.contains(['*', '?', '[', '{', '\\']))
                .copied()
                .collect();
            let wild = literal.len() < segments.len();
            let common = literal.len().min(at.len());
            let agree = literal[..common]
                .iter()
                .zip(&at[..common])
                .all(|(declared, walked)| declared.eq_ignore_ascii_case(walked));
            agree && (wild || at.len() <= literal.len())
        })
    }

    /// The index of the surface that governs a path, or `None` when no declared
    /// surface covers it.
    ///
    /// The *last* match rather than the first: where two surfaces cover one
    /// file, the later declaration is the more specific intent, which is how a
    /// repository says "this tree, except that one file".
    pub fn governing(&self, path: &str) -> Option<usize> {
        self.globs.matches(path).into_iter().max()
    }
}

/// A directory as a prefix: `""` for the root, otherwise with one trailing
/// separator, so a sibling whose name merely starts the same way never
/// matches.
fn normalise(directory: &str) -> String {
    if directory.is_empty() {
        String::new()
    } else {
        format!("{directory}/")
    }
}

/// Whether any directory on the way to a file is one the repository excludes.
///
/// The file's own name is never tested, so a file called `.git` is inspected
/// while everything inside a directory called `.git` is not.
pub fn is_excluded(path: &str, excluded: &[String]) -> bool {
    let mut segments: Vec<&str> = path.split('/').collect();
    segments.pop();
    segments
        .iter()
        .any(|segment| excluded.iter().any(|name| name == segment))
}

/// What an invocation narrowed itself to.
///
/// Passed to the leaves that accept a narrowing flag rather than read from the
/// command line by each of them, so "the declared surface" and "the surface I
/// was pointed at" are one decision made once.
#[derive(Debug, Clone, Default)]
pub struct Scope {
    /// Paths given with `--file`. `-` stands for standard input.
    pub files: Vec<String>,
    pub directory: Option<String>,
    /// What `--file -` should read, when a caller supplied it.
    pub stdin: Option<String>,
}

/// The path a document read from standard input is reported under.
pub const STDIN: &str = "-";

/// Why a selected path yielded no document.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Unselectable {
    /// The path passes through a filesystem link, which RHINO never follows,
    /// so it names something outside the repository root.
    Escapes,
    /// The path names no file.
    Missing,
    /// The path names a file that could not be read.
    Unreadable,
}

impl Unselectable {
    /// The refusal a leaf reports for a selection it could not read.
    ///
    /// One place, so every leaf that accepts `--file` answers a bad selection
    /// with the same code and the same words.
    pub fn refusal(self, category: &'static str, path: &str) -> crate::report::Report {
        let (code, message) = match self {
            Self::Escapes => (
                crate::errors::ErrorCode::PathEscapesRoot,
                format!("{path}: passes through a symbolic link, which RHINO never follows"),
            ),
            Self::Missing => (
                crate::errors::ErrorCode::FileMissing,
                format!("{path}: does not exist"),
            ),
            Self::Unreadable => (
                crate::errors::ErrorCode::FileUnreadable,
                format!("{path}: cannot be read"),
            ),
        };
        crate::report::Report::refused_as(code, category, message)
    }
}

impl Scope {
    pub fn is_narrowed(&self) -> bool {
        !self.files.is_empty()
    }

    /// The documents this scope selects, in the order they were given.
    ///
    /// A path that cannot be read is *not* skipped: it is returned with the
    /// reason so the caller reports it, because a selection naming a file that
    /// is not there is a mistake in the invocation and not an empty repository.
    pub fn documents(&self, tree: &dyn Tree) -> Vec<(String, Result<String, Unselectable>)> {
        self.files
            .iter()
            .map(|path| {
                if path == STDIN {
                    let text = self.stdin.clone().ok_or(Unselectable::Unreadable);
                    (STDIN.to_string(), text)
                } else if tree.passes_through_link(path) {
                    (path.clone(), Err(Unselectable::Escapes))
                } else {
                    let text = tree.read(path).map_err(|error| match error {
                        TreeError::NotFound => Unselectable::Missing,
                        TreeError::Unreadable(_) | TreeError::NotText => Unselectable::Unreadable,
                    });
                    (path.clone(), text)
                }
            })
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::runtime::MemoryTree;

    #[test]
    fn corpus_scope_and_directory_walks_distinguish_read_state_and_declared_exclusions() {
        let mut tree = MemoryTree::default();
        tree.write("docs/README.MD", "# docs");
        tree.write("docs/nested/guide.md", "# guide");
        tree.write("generated/ignored.md", "# ignored");
        tree.write("docs/gone.md", "# gone");
        tree.mark_vanished("docs/gone.md");
        let config = Config {
            scan: Some(crate::config::Scan {
                exclude_directories: vec!["generated".to_string()],
            }),
            ..Config::default()
        };
        assert!(Corpus::read(&tree, &config).is_ok());
        assert!(directories(&tree, &config, "docs").contains(&"docs/nested".to_string()));
        assert!(is_markdown("docs/README.MD"));
        assert!(!is_markdown("docs/readme"));
        assert!(is_excluded("generated/ignored.md", config.excluded()));
        assert_eq!(
            Surfaces::compile("test", ["docs/*.md"])
                .unwrap()
                .governing("docs/a.md"),
            Some(0)
        );

        let scope = Scope {
            files: vec![STDIN.to_string(), "missing.md".to_string()],
            directory: None,
            stdin: Some("standard input".to_string()),
        };
        assert_eq!(
            scope.documents(&tree),
            vec![
                (STDIN.to_string(), Ok("standard input".to_string())),
                ("missing.md".to_string(), Err(Unselectable::Missing))
            ]
        );
        tree.mark_binary("docs/nested/guide.md");
        assert!(Corpus::read(&tree, &config).is_err());
    }

    #[test]
    fn a_selection_behind_a_link_escapes_and_an_unreadable_one_is_unreadable() {
        let mut tree = MemoryTree::default();
        tree.write("docs/outside/held.md", "# held");
        tree.mark_link("docs/outside");
        let scope = Scope {
            files: vec!["docs/outside/held.md".to_string(), STDIN.to_string()],
            directory: None,
            stdin: None,
        };
        assert_eq!(
            scope.documents(&tree),
            vec![
                (
                    "docs/outside/held.md".to_string(),
                    Err(Unselectable::Escapes)
                ),
                (STDIN.to_string(), Err(Unselectable::Unreadable)),
            ]
        );
        tree.write("docs/sealed.md", "# sealed");
        tree.mark_unreadable("docs/sealed.md");
        let sealed = Scope {
            files: vec!["docs/sealed.md".to_string()],
            ..Scope::default()
        };
        assert_eq!(
            sealed.documents(&tree),
            vec![("docs/sealed.md".to_string(), Err(Unselectable::Unreadable))]
        );
        for (reason, code) in [
            (Unselectable::Escapes, "rhino.path.escapes-root"),
            (Unselectable::Missing, "rhino.file.missing"),
            (Unselectable::Unreadable, "rhino.file.unreadable"),
        ] {
            let refused = reason
                .refusal("mermaid", "docs/a.md")
                .render(crate::cli::Format::Json);
            assert_eq!(refused.exit_code, 2);
            assert!(refused.stderr.contains(code), "{}", refused.stderr);
        }
    }

    fn reached_paths(
        surfaces: &Surfaces,
        tree: &MemoryTree,
        config: &Config,
    ) -> Vec<(String, String)> {
        surfaces
            .files(tree, config)
            .unwrap()
            .into_iter()
            .map(|reached| (reached.path, reached.source))
            .collect()
    }

    #[test]
    fn a_surface_follows_an_in_root_link_to_a_directory_a_file_and_a_nested_link() {
        let mut tree = MemoryTree::default();
        tree.write("governance/a.md", "# a");
        tree.write("governance/deeper/b.md", "# b");
        tree.write("notes/c.md", "# c");
        tree.write("single.md", "# single");
        tree.mark_link_to("gov", "governance");
        tree.mark_link_to("governance/notes", "notes");
        tree.mark_link_to("alias.md", "single.md");
        let config = Config::default();
        let surfaces = Surfaces::compile("test", ["gov/**/*.md", "alias.md"]).unwrap();
        let reached = reached_paths(&surfaces, &tree, &config);
        for (path, source) in [
            ("alias.md", "single.md"),
            ("gov/a.md", "governance/a.md"),
            ("gov/deeper/b.md", "governance/deeper/b.md"),
            ("gov/notes/c.md", "notes/c.md"),
            ("governance/a.md", "governance/a.md"),
        ] {
            assert!(
                reached.contains(&(path.to_string(), source.to_string())),
                "{path} from {source} in {reached:?}"
            );
        }
        // A followed file no surface governs is not admitted.
        assert!(
            !reached
                .iter()
                .any(|(path, _)| path == "governance/notes/c.md")
        );

        let corpus = Corpus::reached(&tree, &config, &surfaces).unwrap();
        assert!(
            corpus
                .documents()
                .iter()
                .any(|document| document.path == "gov/notes/c.md" && document.text == "# c")
        );
    }

    #[test]
    fn a_surface_refuses_a_link_it_reaches_that_escapes_loops_or_leads_nowhere() {
        let config = Config::default();
        let surfaces = Surfaces::compile("test", ["docs/**/*.md"]).unwrap();
        let code = |tree: &MemoryTree| {
            let refused = surfaces
                .files(tree, &config)
                .unwrap_err()
                .refusal("word-budget")
                .render(crate::cli::Format::Json);
            assert_eq!(refused.exit_code, 2);
            refused.stderr
        };

        let mut escaping = MemoryTree::default();
        escaping.write("docs/outside/held.md", "# held");
        escaping.mark_link("docs/outside");
        assert!(code(&escaping).contains("rhino.path.escapes-root"));

        let mut looping = MemoryTree::default();
        looping.write("docs/a.md", "# a");
        looping.mark_link_to("docs/loop", "docs");
        assert!(code(&looping).contains("symbolic-link cycle"));

        let mut dangling = MemoryTree::default();
        dangling.write("docs/a.md", "# a");
        dangling.mark_link_to("docs/gone", "missing");
        assert!(code(&dangling).contains("rhino.file.unreadable"));

        let mut unreadable = MemoryTree::default();
        unreadable.write("held/a.md", "# a");
        unreadable.write("held/b.md", "# b");
        unreadable.mark_link_to("docs", "held");
        unreadable.mark_unreadable("held/a.md");
        let Err(refused) = Corpus::reached(&unreadable, &config, &surfaces) else {
            panic!("an unreadable followed file refuses");
        };
        assert_eq!(refused.code, ErrorCode::FileUnreadable);
        let mut binary = MemoryTree::default();
        binary.write("held/a.md", "# a");
        binary.write("held/b.md", "# b");
        binary.mark_link_to("docs", "held");
        binary.mark_vanished("held/a.md");
        binary.mark_binary("held/b.md");
        let Err(binary) = Corpus::reached(&binary, &config, &surfaces) else {
            panic!("a followed file holding no text refuses");
        };
        assert!(
            binary.message.contains("holds no text"),
            "{}",
            binary.message
        );
    }

    #[test]
    fn a_link_no_surface_reaches_or_the_scan_excludes_is_neither_followed_nor_refused() {
        let mut tree = MemoryTree::default();
        tree.write("docs/a.md", "# a");
        tree.write("vendor/outside/held.md", "# held");
        tree.mark_link("vendor/outside");
        tree.write("generated/loop/x.md", "# x");
        tree.mark_link_to("docs/generated", "docs");
        tree.mark_link_to("docs/linked", "docs/a.md");
        let config = Config {
            scan: Some(crate::config::Scan {
                exclude_directories: vec!["generated".to_string()],
            }),
            ..Config::default()
        };
        let surfaces = Surfaces::compile("test", ["docs/**/*.md", "exact/path.md"]).unwrap();
        let reached = reached_paths(&surfaces, &tree, &config);
        assert!(reached.contains(&("docs/a.md".to_string(), "docs/a.md".to_string())));
        // The link to a file is reached, followed, and governed by no surface.
        assert!(!reached.iter().any(|(path, _)| path == "docs/linked"));

        assert!(surfaces.reaches("docs"));
        assert!(surfaces.reaches("DOCS/deeper"));
        assert!(surfaces.reaches("exact"));
        assert!(!surfaces.reaches("exact/path.md/below"));
        assert!(!surfaces.reaches("vendor/outside"));
    }
}
