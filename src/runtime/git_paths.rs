//! Pure parsing for Git's raw NUL-delimited path records.

use std::path::Path;

pub(super) fn regular_file_paths(raw: &[u8]) -> Result<Vec<String>, String> {
    if raw.is_empty() {
        return Ok(Vec::new());
    }
    if !raw.ends_with(&[0]) || raw.ends_with(&[0, 0]) {
        return Err("Git returned a truncated raw path record".to_string());
    }

    let mut fields = raw[..raw.len() - 1].split(|byte| *byte == 0);
    let mut paths = Vec::new();
    while let Some(header) = fields.next() {
        if header.is_empty() {
            return Err("Git returned an empty raw path header".to_string());
        }
        let path = fields
            .next()
            .filter(|path| !path.is_empty())
            .ok_or_else(|| "Git returned a raw record without a pathname".to_string())?;
        let (destination_mode, status) = parse_header(header)?;
        let path = std::str::from_utf8(path)
            .map_err(|_| "Git returned a non-UTF-8 raw pathname".to_string())?;
        if !is_repository_path(path) {
            return Err(format!("Git returned an unsafe repository path `{path}`"));
        }
        if matches!(destination_mode, "100644" | "100755") && status != 'D' {
            paths.push(path.to_string());
        }
    }
    paths.sort();
    paths.dedup();
    Ok(paths)
}

fn parse_header(header: &[u8]) -> Result<(&str, char), String> {
    let header = std::str::from_utf8(header)
        .map_err(|_| "Git returned a non-ASCII raw path header".to_string())?;
    let Some(header) = header.strip_prefix(':') else {
        return Err("Git returned a malformed raw path header".to_string());
    };
    let fields: Vec<&str> = header.split(' ').collect();
    if fields.len() != 5 || fields.iter().any(|field| field.is_empty()) {
        return Err("Git returned a malformed raw path header".to_string());
    }
    let [
        source_mode,
        destination_mode,
        source_oid,
        destination_oid,
        status,
    ] = fields.as_slice()
    else {
        unreachable!("the header field count was checked")
    };
    if !valid_mode(source_mode) || !valid_mode(destination_mode) {
        return Err("Git returned an unsupported raw file mode".to_string());
    }
    if !valid_object_ids(source_oid, destination_oid)
        || ((*source_mode == "000000") != all_zero(source_oid))
        || ((*destination_mode == "000000") != all_zero(destination_oid))
    {
        return Err("Git returned malformed raw object IDs".to_string());
    }
    let mut status_characters = status.chars();
    let Some(status_code) = status_characters.next() else {
        return Err("Git returned an empty raw status".to_string());
    };
    if status_code.is_ascii_digit()
        || status_characters.next().is_some()
        || !matches!(status_code, 'A' | 'D' | 'M' | 'T')
    {
        return Err("Git returned an unsupported raw change status".to_string());
    }
    let source_missing = *source_mode == "000000";
    let destination_missing = *destination_mode == "000000";
    if match status_code {
        'A' => !source_missing || destination_missing,
        'D' => source_missing || !destination_missing,
        'M' | 'T' => source_missing || destination_missing,
        _ => true,
    } {
        return Err("Git returned inconsistent raw status and file modes".to_string());
    }
    Ok((destination_mode, status_code))
}

fn valid_mode(mode: &str) -> bool {
    matches!(mode, "000000" | "100644" | "100755" | "120000" | "160000")
}

fn valid_object_ids(source: &str, destination: &str) -> bool {
    let valid_id = |id: &str| {
        !id.is_empty() && id.len() <= 64 && id.bytes().all(|byte| byte.is_ascii_hexdigit())
    };
    valid_id(source) && valid_id(destination)
}

fn all_zero(value: &str) -> bool {
    value.bytes().all(|byte| byte == b'0')
}

pub(super) fn is_repository_path(path: &str) -> bool {
    !path.is_empty()
        && !Path::new(path).is_absolute()
        && path
            .split('/')
            .all(|segment| !segment.is_empty() && segment != "." && segment != "..")
}

#[cfg(test)]
mod tests {
    use super::regular_file_paths;

    fn record(source_mode: &str, destination_mode: &str, status: &str, path: &[u8]) -> Vec<u8> {
        let source = if source_mode == "000000" {
            "0".repeat(40)
        } else {
            "a".repeat(40)
        };
        let destination = if destination_mode == "000000" {
            "0".repeat(40)
        } else {
            "b".repeat(40)
        };
        let mut raw =
            format!(":{source_mode} {destination_mode} {source} {destination} {status}\0")
                .into_bytes();
        raw.extend_from_slice(path);
        raw.push(0);
        raw
    }

    #[test]
    fn accepts_eligible_destination_modes_and_excludes_other_types() {
        let mut raw = Vec::new();
        raw.extend(record("000000", "100644", "A", b"new.md"));
        raw.extend(record("100644", "100755", "M", b"exec.md"));
        raw.extend(record("100644", "120000", "T", b"link"));
        raw.extend(record("100644", "160000", "T", b"submodule"));
        raw.extend(record("100644", "000000", "D", b"deleted.md"));

        assert_eq!(regular_file_paths(&raw).unwrap(), ["exec.md", "new.md"]);
    }

    #[test]
    fn selects_type_changes_and_rename_additions_by_the_destination_mode() {
        let mut raw = Vec::new();
        raw.extend(record("100644", "100755", "M", b"mode-change"));
        raw.extend(record("120000", "100644", "T", b"link-to-file"));
        raw.extend(record("100644", "120000", "T", b"file-to-link"));
        raw.extend(record("160000", "100644", "T", b"gitlink-to-file"));
        raw.extend(record("100644", "160000", "T", b"file-to-gitlink"));
        raw.extend(record("100644", "000000", "D", b"renamed-away"));
        raw.extend(record("000000", "100644", "A", b"renamed-here"));

        assert_eq!(
            regular_file_paths(&raw).unwrap(),
            [
                "gitlink-to-file",
                "link-to-file",
                "mode-change",
                "renamed-here"
            ]
        );
    }

    #[test]
    fn sorts_deduplicates_and_preserves_unusual_valid_names() {
        let mut raw = record("100644", "100644", "M", b"dir/line\nname.md");
        raw.extend(record("100644", "100644", "M", b"-leading\tname.md"));
        raw.extend(record("100644", "100644", "M", b"dir/line\nname.md"));

        assert_eq!(
            regular_file_paths(&raw).unwrap(),
            ["-leading\tname.md", "dir/line\nname.md"]
        );
    }

    #[test]
    fn accepts_empty_output() {
        assert_eq!(regular_file_paths(b"").unwrap(), Vec::<String>::new());
    }

    #[test]
    fn rejects_truncated_or_malformed_records_even_when_the_path_would_be_excluded() {
        for raw in [
            b":100644 100644 abc def M\0path".as_slice(),
            b":100644 100644 abc def M\0\0".as_slice(),
            b":100644 100644 abc def M\0path\0extra\0".as_slice(),
            b":100644 100644 abc def U\0path\0".as_slice(),
            b":100644 100644 abc def M\0../escape\0".as_slice(),
            b":100644 120000 abc def T\0bad\xff\0".as_slice(),
        ] {
            assert!(regular_file_paths(raw).is_err(), "accepted {raw:?}");
        }
    }

    #[test]
    fn accepts_object_ids_with_different_valid_widths() {
        let mut raw = format!(
            ":100644 100644 {} {} M\0sha1.md\0",
            "a".repeat(40),
            "b".repeat(40)
        )
        .into_bytes();
        raw.extend(
            format!(
                ":100644 100644 {} {} M\0sha256.md\0",
                "c".repeat(64),
                "d".repeat(64)
            )
            .as_bytes(),
        );
        assert_eq!(regular_file_paths(&raw).unwrap(), ["sha1.md", "sha256.md"]);
    }

    #[test]
    fn accepts_valid_abbreviated_object_ids_without_width_coupling() {
        let raw = b":100644 100644 abcd bcde M\0short.md\0:100644 100644 abcdefa bcdefabc M\0unequal.md\0";
        assert_eq!(regular_file_paths(raw).unwrap(), ["short.md", "unequal.md"]);
    }

    #[test]
    fn rejects_bad_headers_modes_statuses_and_unmerged_records() {
        for raw in [
            b"100644 100644 a b M\0path\0".as_slice(),
            b":100644 100644 zzz def M\0path\0".as_slice(),
            b":100644 100644  def M\0path\0".as_slice(),
            b":100644 100644 abc def Q\0path\0".as_slice(),
            b":100644 100644 abc def U\0path\0".as_slice(),
            b":100644 100644 abc def M\0path\0extra\0".as_slice(),
        ] {
            assert!(regular_file_paths(raw).is_err(), "accepted {raw:?}");
        }
    }

    #[test]
    fn rejects_unsafe_paths_and_invalid_utf8_even_for_excluded_modes() {
        for path in [
            b"../escape".as_slice(),
            b"/absolute".as_slice(),
            b"a/../b".as_slice(),
            b"a//b".as_slice(),
            b"bad\xff".as_slice(),
        ] {
            let raw = record("100644", "120000", "T", path);
            assert!(regular_file_paths(&raw).is_err(), "accepted {path:?}");
        }
    }
}
