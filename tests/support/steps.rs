//! The step vocabulary.
//!
//! Every sentence the corpus uses, normalised so that a quoted value reads as
//! `{string}` and a bare number as `{int}`. The list is the shared vocabulary
//! all three adapters bind against: one sentence means one thing regardless of
//! which boundary is running it.
//!
//! Two properties are worth more than the list itself. A step the corpus uses
//! that no entry matches is `Undefined` -- a scenario written in a sentence no
//! adapter knows, which would otherwise pass silently by never being run. A
//! step that matches but has no implementation yet is `Unimplemented`, which is
//! what every step is before its validator is ported.

/// Every sentence the corpus is allowed to use.
pub const VOCABULARY: [&str; 116] = [
    "I add a canonical skill supporting resource",
    "I count the words in {string}",
    "I find word-limit violations",
    "I inspect Mermaid accessibility",
    "I inspect and remember the harness-parity digest",
    "I inspect directory maps",
    "I inspect directory maps under the invalid {string} location",
    "I inspect harness parity",
    "I inspect harness parity again",
    "I inspect harness parity narrowed to {string}",
    "I inspect harness parity twice",
    "I inspect the word budget",
    "I invoke the CLI from the repository directory with {string}",
    "I invoke the CLI with {string}",
    "I remember the repository snapshot",
    "I run the directory-map validator for {string}",
    "I run the {string} validator",
    "I scan the declared surfaces",
    "Markdown text containing a heading marker, Hello, can't-stop, naïve, and {int}",
    "a duplicate canonical skill name exists",
    "a file imports the canonical instruction body",
    "a nested repository instruction file exists",
    "a repository declaring every section with nothing to find",
    "a valid one-skill one-agent harness contract",
    "all violations are {string}",
    "an argument error is raised",
    "an empty repository",
    "an unexpected agent adapter exists",
    "an unsafe {string} Mermaid diagram exists at {string} using backtick fences",
    "an unsafe {string} Mermaid diagram exists at {string} using tilde fences",
    "each declared excluded directory contains an unsafe Mermaid diagram",
    "each harness declares the required capability in its own capability format",
    "excluded instruction sources and a linked skill exist",
    "file {string} contains this Markdown:",
    "file {string} contains {int} words",
    "file {string} exceeds its declared word budget",
    "file {string} has an empty {string} directory map followed by {int} words",
    "file {string} has title {string} and an empty directory map",
    "harness {string} declares a command directory",
    "harness {string} declares an instruction overlay",
    "harness {string} declares no command directory",
    "harness-parity outputs are identical",
    "harness-parity validation succeeds",
    "harness-parity validation succeeds with {int} harnesses, {int} skill, {int} agent, and {int} capability",
    "harness-parity violations are ordinally sorted",
    "no Markdown files are scanned",
    "no directories were inspected",
    "stderr lines start with {string}",
    "stderr names the canon that has nowhere to be reconciled",
    "stderr names the missing configuration file",
    "stderr names the missing key",
    "stderr names the offending key and its position",
    "stderr names the unknown key and its position",
    "stderr names the unrecognized schema",
    "stdout JSON has a {string} and a 40-character {string}",
    "stdout JSON property {string} is {int}",
    "stdout is empty",
    "stdout lines start with {string}",
    "stdout reports that zero diagrams were checked",
    "the agent adapter for {string} contains extra prompt instructions",
    "the agent adapter for {string} is missing",
    "the agent adapter for {string} weakens a denied capability",
    "the canonical agent requires a capability outside the declared vocabulary",
    "the capability declaration for harness {string} is unreadable",
    "the configuration adds the top-level section {string}",
    "the configuration adds the unknown key {string} to {string}",
    "the configuration omits {string}",
    "the configuration sets {string} to {string}",
    "the declared instruction adapter contains extra instructions",
    "the exit code is {int}",
    "the first stdout JSON legibility fields are {string}, {int}, and {int}",
    "the first stdout JSON violation kind is {string}",
    "the formatted violation starts with {string}",
    "the governed files are exclusively locked",
    "the harness-parity digest changed",
    "the harness-parity violations include {string}",
    "the only violation is a 41-word limit for {string}",
    "the only violation is a 60-word limit for {string}",
    "the only violation is a Mermaid accessibility issue at {string}",
    "the only violation is a missing README at {string}",
    "the only violation is a missing directory map at {string}",
    "the only violation is a missing map entry from {string} to {string}",
    "the only violation is an invalid map entry from {string} to {string}",
    "the repository contains Mermaid sample {string} at {string}",
    "the repository contains a Mermaid class filled {string} with a declared stroke and text color",
    "the repository contains a Mermaid node label of {int} graphemes",
    "the repository contains:",
    "the repository declares a complete configuration",
    "the repository declares a configuration with an empty harness roster and a canonical skills root",
    "the repository declares a configuration with an empty harness roster and no canonical skill or agent root",
    "the repository declares a harness roster of {string}, {string}, and {string}",
    "the repository declares a palette whose only fill color is {string}",
    "the repository declares a word-budget surface {string} failing above {int}",
    "the repository declares excluded scan directories:",
    "the repository declares no instruction adapter",
    "the repository declares node labels at {int} graphemes and edge labels at {int}",
    "the repository declares the accessible palette",
    "the repository declares the internal-link excluded source {string}",
    "the repository declares the mapped tree {string}",
    "the repository declares the mapped trees {string} and {string}",
    "the repository declares the schema {string}",
    "the repository declares word-budget surfaces:",
    "the repository has no configuration file",
    "the repository snapshot is unchanged",
    "the required-capability command for harness {string} diverges",
    "the scanned Markdown paths are:",
    "the skill wrapper for {string} has a stale description and extra body",
    "the skill wrapper for {string} is missing",
    "the violations include an overlong {string} and its missing map entry for {string}",
    "the word count is {int}",
    "there are no violations",
    "there are {int} violations",
    "two sorted harness-parity violations exist",
    "{int} Mermaid diagrams were inspected",
    "{int} directories were inspected",
    "{int} harnesses were inspected",
];

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StepOutcome {
    /// The sentence matches no vocabulary entry.
    Undefined,
    /// The sentence is known; the production call behind it does not exist yet.
    Unimplemented(&'static str),
}

/// Match a step sentence against the vocabulary, returning the entry it matched.
///
/// `{string}` matches a double-quoted run and `{int}` a bare integer, so the
/// match is structural rather than a substring test: a sentence that differs
/// outside its placeholders does not match, which is what makes `Undefined`
/// meaningful.
pub fn lookup(sentence: &str) -> Option<&'static str> {
    VOCABULARY
        .iter()
        .copied()
        .find(|pattern| matches(pattern, sentence))
}

pub fn dispatch(sentence: &str) -> StepOutcome {
    match lookup(sentence) {
        Some(pattern) => StepOutcome::Unimplemented(pattern),
        None => StepOutcome::Undefined,
    }
}

fn matches(pattern: &str, sentence: &str) -> bool {
    // Walk the pattern literal-by-literal, consuming a placeholder's value from
    // the sentence whenever the pattern reaches one.
    let mut rest = sentence;
    let mut pattern_rest = pattern;
    loop {
        let Some(open) = pattern_rest.find('{') else {
            return rest == pattern_rest;
        };
        let literal = &pattern_rest[..open];
        let Some(after_literal) = rest.strip_prefix(literal) else {
            return false;
        };
        rest = after_literal;

        let Some(close) = pattern_rest[open..].find('}') else {
            return false;
        };
        let placeholder = &pattern_rest[open + 1..open + close];
        pattern_rest = &pattern_rest[open + close + 1..];

        let consumed = match placeholder {
            "string" => consume_quoted(rest),
            "int" => consume_int(rest),
            _ => None,
        };
        let Some(width) = consumed else {
            return false;
        };
        rest = &rest[width..];
    }
}

/// A quoted run, including both quotes. Quotes do not nest in the corpus.
fn consume_quoted(rest: &str) -> Option<usize> {
    let body = rest.strip_prefix('"')?;
    let end = body.find('"')?;
    Some(end + 2)
}

fn consume_int(rest: &str) -> Option<usize> {
    let width = rest.chars().take_while(char::is_ascii_digit).count();
    (width > 0).then_some(width)
}
