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
pub const VOCABULARY: [&str; 225] = [
    "I add a canonical skill supporting resource",
    "I add a file outside the canon",
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
    "I inspect internal links",
    "I inspect the word budget",
    "I invoke the CLI from the repository directory with {string}",
    "I invoke the CLI with no arguments",
    "I invoke the CLI with {string}",
    "I run the directory-map validator for {string}",
    "I run the {string} validator",
    "I scan the declared surfaces",
    "Markdown text containing a heading marker, Hello, can't-stop, naïve, and {int}",
    "a canonical skill declares a name that is not its directory",
    "a documentation page carries a stray backtick before the canonical import",
    "a documentation page hides the canonical import behind an unclosed fence",
    "a documentation page shows the canonical import in a code span containing a backtick",
    "a documentation page shows the canonical import in an inline code span",
    "a documentation page shows the canonical import inside a fenced example",
    "a file imports the canonical instruction body",
    "a file with a prohibited name is not Markdown",
    "a harness translates a capability it does not name",
    "a harness translates a capability the repository never declared",
    "a loose file sits directly under the canonical skills root",
    "a nested repository instruction file exists",
    "a nested repository under {string} scans a directory the outer repository excludes",
    "a repository declaring every section with nothing to find",
    "a required MCP server is declared",
    "a source file contains the canonical import",
    "the canonical instruction body and its adapter are not Markdown",
    "a harness configuration field is declared as a prohibited instruction source",
    "a harness configuration field in TOML is declared as a prohibited instruction source",
    "that field is written as {string}",
    "that settings file is not in the repository",
    "that settings file is not written in its declared format",
    "a valid one-skill one-agent one-capability harness contract",
    "all violations are {string}",
    "an argument error is raised",
    "an empty repository",
    "an index README sits in every harness agent directory",
    "an index README sits in the canonical agents root",
    "an out-of-order pair of harness-parity violations exists",
    "an unexpected agent adapter exists",
    "an unexpected skill wrapper exists for {string}",
    "an unsafe {string} Mermaid diagram exists at {string} using backtick fences",
    "an unsafe {string} Mermaid diagram exists at {string} using tilde fences",
    "an unsafe {string} Mermaid diagram is supplied on standard input",
    "each declared excluded directory contains an unsafe Mermaid diagram",
    "each harness declares the required capability in its own capability format",
    "every harness closes its agent adapter declaration",
    "excluded instruction sources and a linked skill exist",
    "file {string} cannot be opened",
    "file {string} contains this Markdown:",
    "file {string} contains {int} words",
    "file {string} exceeds its declared word budget",
    "file {string} has an empty {string} directory map followed by {int} words",
    "file {string} has title {string} and an empty directory map",
    "file {string} holds a bolded word and a URL",
    "file {string} holds bytes that are not text",
    "file {string} vanishes between the walk and the read",
    "harness {string} declares a command directory",
    "harness {string} declares an instruction overlay",
    "harness {string} declares no command directory",
    "harness {string} writes its capability as one command vector",
    "harness-parity outputs are identical",
    "harness-parity validation succeeds",
    "harness-parity validation succeeds with {int} harnesses, {int} skill, {int} agent, and {int} reconciled capability declarations",
    "harness-parity violations are ordinally sorted",
    "no Markdown files are scanned",
    "no canonical agents are declared",
    "no canonical agents are declared and no harness expresses them",
    "no canonical skills are declared",
    "no capability server is required",
    "no capability server is required and no harness declares a capability file",
    "no directories were inspected",
    "no output names an optional section",
    "no harness declares a capability file",
    "no harness declares an agent adapter",
    "stderr contains {string}",
    "stderr lines start with {string}",
    "stderr names a file it could not read",
    "stderr names the canon that has nowhere to be reconciled",
    "stderr names the missing configuration file",
    "stderr names the missing key",
    "stderr names the missing schema declaration",
    "stderr names the offending key and its position",
    "stderr names the unknown key and its position",
    "stderr names the unreadable capability file",
    "stderr names the unreadable configuration file",
    "stderr names the unrecognized schema",
    "stdout JSON has a {string} and a {int}-character hexadecimal {string}",
    "stdout JSON property {string} is {int}",
    "stdout does not name the command {string}",
    "stdout escapes the quote, backslash, tab, and control character",
    "stdout is empty",
    "stdout is exactly the release identity envelope",
    "stdout is one non-empty line",
    "stdout is the version the JSON form reported",
    "stdout lines start with {string}",
    "stdout names every exit code",
    "stdout names the command {string}",
    "stdout reports that zero diagrams were checked",
    "the agent adapter for {string} carries no declaration",
    "the agent adapter for {string} changes a field its harness fixes",
    "the agent adapter for {string} contains extra prompt instructions",
    "the agent adapter for {string} declares a field its harness forbids",
    "the agent adapter for {string} grants nothing answering a capability",
    "the agent adapter for {string} ignores a declared constraint",
    "the agent adapter for {string} is missing",
    "the agent adapter for {string} lists its grants",
    "the agent adapter for {string} names another agent",
    "the agent adapter for {string} weakens a denied capability",
    "the agent adapter for {string} withholds a required capability",
    "the agent adapter for {string} wraps its route across lines",
    "the agent directory for {string} holds the extra file {string}",
    "the canonical agent carries no declaration",
    "the canonical agent declares a constraint outside the declared vocabulary",
    "the canonical agent omits the field every declaration must carry",
    "the canonical agent requires a capability outside the declared vocabulary",
    "the canonical agent requires less and no adapter answers for it",
    "the canonical agent requires less than its adapters grant",
    "the canonical agents are gone and only their declaration shape remains",
    "the canonical agents declare their permissions under {string} names",
    "the canonical instruction body is absent",
    "the canonical skill carries no declaration",
    "the canonical skill declares no description",
    "the canonical skill front matter is never closed",
    "the capability declaration for harness {string} is a list of server groups",
    "the capability declaration for harness {string} is absent",
    "the capability declaration for harness {string} is missing the required capability",
    "the capability declaration for harness {string} is not valid in its declared format",
    "the capability declaration for harness {string} is unreadable",
    "the configuration adds the top-level section {string}",
    "the configuration adds the unknown key {string} to {string}",
    "the configuration file cannot be read",
    "the configuration file is this text:",
    "the configuration omits {string}",
    "the configuration sets {string} to {string}",
    "the declared instruction adapter contains extra instructions",
    "the declared instruction adapter is absent",
    "the exit code is {int}",
    "the first stdout JSON legibility fields are {string}, {int}, and {int}",
    "the first stdout JSON violation kind is {string}",
    "the formatted violation starts with {string}",
    "the governed files are exclusively locked",
    "the harness-parity digest changed",
    "the harness-parity digest is unchanged",
    "the harness-parity violation for {string} is {string}",
    "the harness-parity violations include {string}",
    "the only violation is a Mermaid accessibility issue at {string}",
    "the only violation is a missing README at {string}",
    "the only violation is a missing directory map at {string}",
    "the only violation is a missing map entry from {string} to {string}",
    "the only violation is a {int}-word limit for {string}",
    "the only violation is an invalid map entry from {string} to {string}",
    "the only violation starts with {string}",
    "the reported version is spelled as its release tag",
    "the reported version is the one the product manifest declares",
    "the repository contains Mermaid sample {string} at {string}",
    "the repository contains a Mermaid class filled {string} with a declared stroke and text color",
    "the repository contains a Mermaid node label of {int} graphemes",
    "the repository contains a Mermaid node label written as {string}",
    "the repository contains:",
    "the repository counts words as {string}",
    "the repository declares a complete configuration",
    "the repository declares a configuration with an empty harness roster and a canonical skills root",
    "the repository declares a configuration with an empty harness roster and no canonical skill or agent root",
    "the repository declares a harness roster of {string}, {string}, and {string}",
    "the repository declares a palette whose only fill color is {string}",
    "the repository declares a word-budget surface {string} failing above {int}",
    "the repository declares an unusable prohibited instruction source",
    "the repository declares excluded scan directories:",
    "the repository declares no instruction adapter",
    "the repository declares node labels at {int} graphemes and edge labels at {int}",
    "the repository declares the emoji-prohibited surface {string}",
    "the repository declares the accessible palette",
    "the repository declares the front-matter surface {string} forbidding {string}",
    "the repository declares the heading surface {string} with any number of H1s and a maximum jump of {int}",
    "the repository declares the heading surface {string} with one H1 and a maximum jump of {int}",
    "the repository declares the front-matter surface {string} requiring nothing",
    "the repository declares the front-matter surface {string} requiring {string}",
    "the repository declares the front-matter surface {string} with the ISO date key {string}",
    "the repository declares the front-matter surface {string} with the {string} values {string}",
    "the repository declares the internal-link excluded source {string}",
    "the repository declares the kebab-case naming surface {string}",
    "the repository declares the metadata surface {string} using the {string} schema",
    "the repository declares the model-tier mapping {string}",
    "the repository maps harness {string} tier {string} to model {string} at effort {string}",
    "the violations are ordinally sorted",
    "the repository declares the mapped tree {string}",
    "the repository declares the mapped trees {string} and {string}",
    "the repository declares the naming exemption {string}",
    "the repository declares the path-prefixed naming surface {string} separated by {string}",
    "the repository declares the README-index tree {string}",
    "the repository declares the schema {string}",
    "the repository declares word-budget surfaces:",
    "the repository has no configuration file",
    "the repository holds a map entry whose target needs escaping",
    "the repository is unchanged by the inspection",
    "the repository writes its route template with uneven spacing",
    "the required-capability arguments for harness {string} diverge",
    "the required-capability command for harness {string} diverges",
    "the scanned Markdown paths are:",
    "the skill wrapper for {string} carries no declaration",
    "the skill wrapper for {string} declares more than its route",
    "the skill wrapper for {string} has a stale description",
    "the skill wrapper for {string} has an extra body",
    "the skill wrapper for {string} is missing",
    "the word count is {int}",
    "there are no violations",
    "there are {int} violations",
    "{int} Mermaid diagrams were inspected",
    "{int} capability declarations were reconciled",
    "{int} directories were inspected",
    "{int} files were inspected",
    "{int} harnesses were inspected",
    "{int} links were inspected",
];

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StepOutcome {
    /// The sentence matches no vocabulary entry.
    Undefined,
    /// The sentence is known; the production call behind it does not exist yet.
    Unimplemented(&'static str),
}

/// A matched sentence, with the values its placeholders captured in order.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Match {
    pub pattern: &'static str,
    pub strings: Vec<String>,
    pub integers: Vec<usize>,
}

impl Match {
    /// The nth quoted value, or a clear failure naming which one was missing.
    pub fn string(&self, index: usize) -> &str {
        self.strings.get(index).unwrap_or_else(|| {
            panic!(
                "step `{}` has no quoted value at position {index}",
                self.pattern
            )
        })
    }

    pub fn integer(&self, index: usize) -> usize {
        *self
            .integers
            .get(index)
            .unwrap_or_else(|| panic!("step `{}` has no integer at position {index}", self.pattern))
    }
}

/// Match a step sentence against the vocabulary, capturing its placeholders.
///
/// `{string}` matches a double-quoted run and `{int}` a bare integer, so the
/// match is structural rather than a substring test: a sentence that differs
/// outside its placeholders does not match, which is what makes `Undefined`
/// meaningful.
/// Whether one pattern matches a sentence, without capturing.
///
/// Exposed so the static coverage check can ask how *many* entries match a
/// sentence: `lookup` answers with the first, which is the right answer for a
/// running adapter and the wrong one for a check about ambiguity.
pub fn matches(pattern: &str, sentence: &str) -> bool {
    capture(pattern, sentence).is_some()
}

pub fn lookup(sentence: &str) -> Option<Match> {
    VOCABULARY.iter().copied().find_map(|pattern| {
        capture(pattern, sentence).map(|(strings, integers)| Match {
            pattern,
            strings,
            integers,
        })
    })
}

type Captures = (Vec<String>, Vec<usize>);

fn capture(pattern: &str, sentence: &str) -> Option<Captures> {
    // Walk the pattern literal-by-literal, consuming a placeholder's value from
    // the sentence whenever the pattern reaches one.
    let mut rest = sentence;
    let mut pattern_rest = pattern;
    let mut strings: Vec<String> = Vec::new();
    let mut integers: Vec<usize> = Vec::new();

    loop {
        let Some(open) = pattern_rest.find('{') else {
            return (rest == pattern_rest).then_some((strings, integers));
        };
        let literal = &pattern_rest[..open];
        rest = rest.strip_prefix(literal)?;

        let close = pattern_rest[open..].find('}')?;
        let placeholder = &pattern_rest[open + 1..open + close];
        pattern_rest = &pattern_rest[open + close + 1..];

        match placeholder {
            "string" => {
                let body = rest.strip_prefix('"')?;
                let end = body.find('"')?;
                strings.push(body[..end].to_string());
                rest = &body[end + 1..];
            }
            "int" => {
                let width = rest.chars().take_while(char::is_ascii_digit).count();
                if width == 0 {
                    return None;
                }
                integers.push(rest[..width].parse().ok()?);
                rest = &rest[width..];
            }
            _ => return None,
        }
    }
}
