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
pub const VOCABULARY: [&str; 17] = [
    "I invoke the CLI with {string}",
    "I run the {string} validator",
    "stderr contains {string}",
    "stdout JSON property {string} is {int}",
    "stdout contains {string}",
    "the configuration file is this text:",
    "the exit code is {int}",
    "the file {string} still contains {string}",
    "the first stdout JSON violation kind is {string}",
    "the generated adapter at {string} contains {string}",
    "the generated adapter at {string} does not contain {string}",
    "the last adapter generation changes the repository",
    "the last adapter generation makes no repository change",
    "the repository contains:",
    "the repository contains a symbolic link at {string}",
    "the repository is unchanged by the inspection",
    "the two runs are byte-identical",
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
