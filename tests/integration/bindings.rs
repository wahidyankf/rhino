//! Integration bindings.
//!
//! The same scenarios, driven against a real temporary directory rather than an
//! in-memory tree, so the filesystem port's concrete implementation is exercised
//! by the same sentences.

use crate::registry::{Binding, Exemption};

pub const BINDINGS: &[Binding] = &[
    (
        "harness-parity",
        "A constraint is still enforced when the canon renames its fields",
    ),
    (
        "harness-parity",
        "A capability may be written as one command vector",
    ),
    (
        "internal-link",
        "A link quoted inside a code span is prose about a link",
    ),
    (
        "internal-link",
        "A real link on the same line as a quoted one is still checked",
    ),
    (
        "harness-parity",
        "A route template written with uneven spacing is the same route",
    ),
    (
        "harness-parity",
        "A closed adapter still declares the fields its translations name",
    ),
    (
        "harness-parity",
        "A route wrapped across lines is the same route",
    ),
    (
        "word-budget",
        "A repository counting whitespace-separated fields counts the marker too",
    ),
    (
        "word-budget",
        "The budget is measured by the rule the repository declared",
    ),
    (
        "harness-parity",
        "The canon declares its permissions under names the repository chose",
    ),
    (
        "harness-parity",
        "Drift is still caught when the canon renames its permission fields",
    ),
    (
        "harness-parity",
        "A canonical agent must carry the field its declaration fixes",
    ),
    (
        "repo-config",
        "Canonical agents whose permission fields are unnamed are refused",
    ),
    (
        "repo-config",
        "A declaration shape with no canonical agents to describe is refused",
    ),
    (
        "cli-contract",
        "Every canonical nested command path succeeds",
    ),
    ("cli-contract", "Word-budget validation is isolated"),
    ("cli-contract", "Directory-map validation is isolated"),
    (
        "cli-contract",
        "A complete selected tree passes directory-map validation",
    ),
    (
        "cli-contract",
        "A selected directory without a README fails",
    ),
    ("cli-contract", "An omitted selected sibling fails"),
    (
        "cli-contract",
        "A selected tree requires recursive README directory maps",
    ),
    (
        "cli-contract",
        "Mermaid accessibility validation is isolated",
    ),
    ("cli-contract", "Root defaults to the current directory"),
    (
        "cli-contract",
        "Root is accepted before and after nested commands",
    ),
    (
        "cli-contract",
        "A selected root is the repository that gets inspected",
    ),
    (
        "cli-contract",
        "A selected root brings its own scan exclusions",
    ),
    (
        "cli-contract",
        "Leaf output has an atomic command-category prefix",
    ),
    (
        "cli-contract",
        "Inspection errors use command-specific diagnostics",
    ),
    ("cli-contract", "Help requests succeed"),
    (
        "cli-contract",
        "Version reports the embedded release identity",
    ),
    ("cli-contract", "Invalid invocations return usage failure"),
    (
        "cli-contract",
        "A governed file above its declared limit returns validation failure",
    ),
    (
        "cli-contract",
        "An incomplete directory map returns validation failure",
    ),
    (
        "cli-contract",
        "Output format is recursive for every validator",
    ),
    ("cli-contract", "A file word count is observable as JSON"),
    (
        "cli-contract",
        "Word-count inspection never reports findings",
    ),
    (
        "cli-contract",
        "JSON validation failures retain the validation exit code",
    ),
    (
        "cli-contract",
        "Unsupported output formats return usage failure",
    ),
    (
        "cli-contract",
        "Global presentation flags are accepted by every reporting leaf",
    ),
    (
        "cli-contract",
        "Help is scoped to the command path that asked for it",
    ),
    (
        "cli-contract",
        "Repeated file selection is accepted by the Mermaid leaf",
    ),
    (
        "cli-contract",
        "The Mermaid leaf reads a diagram from standard input",
    ),
    (
        "directory-map",
        "Directory-map inspection ignores other validators' concerns",
    ),
    (
        "directory-map",
        "Complete maps cover direct files and directories",
    ),
    (
        "directory-map",
        "The selected directory must stay relative and inside the repository",
    ),
    (
        "directory-map",
        "Query and fragment suffixes do not change a sibling target",
    ),
    (
        "directory-map",
        "Absolute URL and malformed map links are invalid",
    ),
    (
        "directory-map",
        "Every directory in a mapped tree needs a README",
    ),
    (
        "directory-map",
        "Every README needs a Directory Map section",
    ),
    (
        "directory-map",
        "Every direct sibling must appear in the map",
    ),
    (
        "directory-map",
        "Independent inspections report independent violations",
    ),
    ("directory-map", "A map entry must exist"),
    ("directory-map", "A map entry must be a direct sibling"),
    (
        "harness-parity",
        "Canonical instructions, skills, agents, and capability declarations pass",
    ),
    (
        "harness-parity",
        "The canonical instruction body must exist",
    ),
    (
        "harness-parity",
        "A declared instruction adapter must exist",
    ),
    (
        "harness-parity",
        "The instruction adapter may contain only the canonical import",
    ),
    (
        "harness-parity",
        "The canonical instruction body and its adapter need not be Markdown",
    ),
    (
        "harness-parity",
        "A declared instruction adapter is optional",
    ),
    (
        "harness-parity",
        "With no adapter declared, no file may import the canonical instructions",
    ),
    (
        "harness-parity",
        "A README in the canonical agents root is an index, not an agent",
    ),
    (
        "harness-parity",
        "A README in a harness's agent directory is an index, not an adapter",
    ),
    (
        "harness-parity",
        "A source file containing the import is not an instruction source",
    ),
    (
        "harness-parity",
        "A prohibited name competes with the canon whatever the file kind",
    ),
    (
        "harness-parity",
        "A documentation page showing the import in a fenced example is not a source",
    ),
    (
        "harness-parity",
        "A documentation page showing the import in an inline code span is not a source",
    ),
    (
        "harness-parity",
        "An unclosed fence does not hide a competing instruction source",
    ),
    (
        "harness-parity",
        "A code span containing a backtick is read to its matching close",
    ),
    (
        "harness-parity",
        "A stray backtick does not hide a competing instruction source",
    ),
    (
        "harness-parity",
        "A harness configuration field may not carry its own instructions",
    ),
    (
        "harness-parity",
        "A prohibited configuration field that says nothing is not a source",
    ),
    (
        "harness-parity",
        "A prohibited configuration field in a settings file nobody wrote is not a source",
    ),
    (
        "harness-parity",
        "A settings file unreadable in its declared format is a source that cannot be ruled out",
    ),
    (
        "harness-parity",
        "A prohibited configuration field is read in the format its harness writes",
    ),
    (
        "harness-parity",
        "A settings file written in TOML is read as TOML rather than refused",
    ),
    (
        "harness-parity",
        "A prohibited configuration field nobody wrote is not a source",
    ),
    (
        "harness-parity",
        "A harness instruction overlay is a competing source",
    ),
    (
        "harness-parity",
        "Additional or nested instruction sources fail",
    ),
    (
        "harness-parity",
        "A harness declaring a command directory needs one thin wrapper per skill",
    ),
    (
        "harness-parity",
        "A harness declaring no command directory needs no wrappers",
    ),
    (
        "harness-parity",
        "A skill wrapper's description cannot drift from the skill",
    ),
    (
        "harness-parity",
        "A skill wrapper may not grow a body of its own",
    ),
    (
        "harness-parity",
        "A skill wrapper may declare nothing beyond its route",
    ),
    (
        "harness-parity",
        "A wrapper no canonical skill asked for is reported",
    ),
    (
        "harness-parity",
        "Skill bodies and supporting resources affect the contract digest",
    ),
    (
        "harness-parity",
        "Files outside the canon do not affect the contract digest",
    ),
    (
        "harness-parity",
        "A canonical skill with no declaration fails",
    ),
    (
        "harness-parity",
        "A canonical skill with no description fails",
    ),
    (
        "harness-parity",
        "A canonical skill must be declared under the directory it lives in",
    ),
    (
        "harness-parity",
        "Every canonical agent has one adapter per declared harness",
    ),
    (
        "harness-parity",
        "Missing and extra agent adapters fail, each named for what it is",
    ),
    (
        "harness-parity",
        "Extra prompt content and semantic drift are reported apart",
    ),
    (
        "harness-parity",
        "An adapter must grant what the canon requires",
    ),
    (
        "harness-parity",
        "An adapter may grant more than the canon requires",
    ),
    (
        "harness-parity",
        "A capability the canon never took on obliges an adapter nothing",
    ),
    (
        "harness-parity",
        "A repository with harnesses and no canonical agents reconciles none",
    ),
    (
        "harness-parity",
        "A harness may write its grants as a list or as one line",
    ),
    (
        "harness-parity",
        "An adapter must name the agent it stands for",
    ),
    (
        "harness-parity",
        "An adapter must answer a capability in its own vocabulary",
    ),
    (
        "harness-parity",
        "An adapter must grant every member a capability translates to",
    ),
    (
        "harness-parity",
        "An adapter may not declare a field its harness forbids",
    ),
    (
        "harness-parity",
        "An adapter may not change a field its harness fixes",
    ),
    (
        "harness-parity",
        "An agent may declare only vocabulary the repository declared",
    ),
    (
        "harness-parity",
        "An agent may declare only constraints the repository declared",
    ),
    (
        "harness-parity",
        "An adapter may not ignore a declared constraint",
    ),
    (
        "harness-parity",
        "Equivalent required-capability declarations pass in every harness format",
    ),
    (
        "harness-parity",
        "A divergent required-capability declaration fails",
    ),
    (
        "harness-parity",
        "A required-capability declaration with a divergent argument vector fails",
    ),
    (
        "harness-parity",
        "A harness that declares no capability file fails",
    ),
    (
        "harness-parity",
        "An unreadable capability declaration fails closed",
    ),
    (
        "harness-parity",
        "A roster narrowed by flag still reports every finding it inspects",
    ),
    (
        "harness-parity",
        "Declared excluded directories and filesystem links are not inspected",
    ),
    (
        "harness-parity",
        "Findings are stable, sorted, and inspection is read-only",
    ),
    (
        "internal-link",
        "Existing local links and non-local links pass",
    ),
    (
        "internal-link",
        "Missing or out-of-repository local targets fail",
    ),
    (
        "internal-link",
        "A reference-style definition with a missing target fails",
    ),
    (
        "internal-link",
        "Fragment links and fenced Markdown examples are ignored",
    ),
    (
        "internal-link",
        "A malformed local target fails without stopping inspection",
    ),
    ("internal-link", "Image syntax is not an internal link"),
    (
        "internal-link",
        "A declared excluded source is not a link source",
    ),
    (
        "internal-link",
        "An excluded source remains a valid link target",
    ),
    ("mermaid-cli", "File scope excludes unrelated diagrams"),
    (
        "mermaid-cli",
        "A repository with no diagrams reports a clean explicit zero",
    ),
    (
        "mermaid-cli",
        "Parseable diagram types enforce class colors",
    ),
    ("mermaid-cli", "Unparseable diagram types are skipped"),
    ("mermaid-cli", "Tilde-fenced Mermaid diagrams are extracted"),
    (
        "mermaid-cli",
        "Diagram type is found after YAML front matter",
    ),
    (
        "mermaid-cli",
        "A Mermaid block without a diagram type is skipped",
    ),
    (
        "mermaid-cli",
        "Declared excluded directories are not scanned",
    ),
    ("mermaid-cli", "An accessible colored class passes"),
    ("mermaid-cli", "Colors outside classDef are rejected"),
    ("mermaid-cli", "Unsupported color formats are rejected"),
    (
        "mermaid-cli",
        "Palette comments are neither required nor inspected",
    ),
    (
        "mermaid-cli",
        "Node color roles and normal-text contrast are enforced",
    ),
    ("mermaid-cli", "A fill color the repository declared passes"),
    ("mermaid-cli", "An accessible stroke-only class passes"),
    (
        "mermaid-cli",
        "Text-only and inaccessible stroke-only classes fail",
    ),
    (
        "mermaid-legibility",
        "Mermaid inspection ignores other validators' concerns",
    ),
    (
        "mermaid-legibility",
        "Mermaid diagnostics identify the Markdown source line",
    ),
    (
        "mermaid-legibility",
        "Every parseable diagram type rejects an overlong visible node label",
    ),
    (
        "mermaid-legibility",
        "Label segments use deterministic grapheme boundaries",
    ),
    (
        "mermaid-legibility",
        "A different declared limit moves the boundary",
    ),
    (
        "mermaid-legibility",
        "State transition semicolons are rejected",
    ),
    (
        "mermaid-legibility",
        "Legibility JSON reports deterministic measurement fields",
    ),
    (
        "mermaid-legibility",
        "Non-label Mermaid declarations are excluded",
    ),
    ("repo-config", "A well-formed configuration validates"),
    (
        "repo-config",
        "An absent configuration file is a configuration fault",
    ),
    (
        "repo-config",
        "An unrecognized schema identifier is refused",
    ),
    (
        "repo-config",
        "The predecessor schema spelling is accepted as an alias",
    ),
    (
        "repo-config",
        "An unknown key inside an owned section is refused",
    ),
    ("repo-config", "An unknown top-level section is ignored"),
    ("repo-config", "A missing required key is refused"),
    ("repo-config", "A value of the wrong type is refused"),
    (
        "repo-config",
        "A declared path may not escape the repository root",
    ),
    (
        "repo-config",
        "An empty harness roster is a legal declaration",
    ),
    (
        "repo-config",
        "An empty harness roster alongside a declared canonical root is refused",
    ),
    (
        "repo-config",
        "An empty harness roster alongside a required MCP server is refused",
    ),
    (
        "repo-config",
        "A validator refuses to run against a configuration it cannot read",
    ),
    (
        "repo-config",
        "A repository whose harnesses reach no capability server is legal",
    ),
    (
        "repo-config",
        "A repository whose harnesses express no canonical agents is legal",
    ),
    (
        "repo-config",
        "A canonical agents root no harness expresses is refused",
    ),
    (
        "repo-config",
        "An agent adapter with no canonical agents root is refused",
    ),
    (
        "repo-config",
        "A skill adapter with no canonical skills root is refused",
    ),
    (
        "repo-config",
        "A canonical root and the route its adapters carry are one declaration",
    ),
    (
        "repo-config",
        "A translation naming no capability is refused",
    ),
    (
        "repo-config",
        "A translation naming an undeclared capability is refused",
    ),
    (
        "repo-config",
        "A required server no harness declares a capability file for is refused",
    ),
    (
        "repo-config",
        "A capability file no required server reads is refused",
    ),
    (
        "word-budget",
        "Markdown punctuation does not create extra words",
    ),
    ("word-budget", "Only declared surfaces are scanned"),
    ("word-budget", "The declared limit is inclusive"),
    ("word-budget", "An empty repository scans nothing"),
    (
        "word-budget",
        "Files outside every declared surface have no limit",
    ),
    (
        "word-budget",
        "A later surface overrides an earlier one for the same file",
    ),
    (
        "word-budget",
        "Word-budget inspection ignores other validators' concerns",
    ),
    (
        "repo-config",
        "A blank line does not end the search for the schema comment",
    ),
    (
        "repo-config",
        "A configuration that declares no schema is refused",
    ),
    (
        "repo-config",
        "A configuration whose first content is not a comment declares no schema",
    ),
    (
        "repo-config",
        "A configuration file that cannot be read is refused",
    ),
    (
        "repo-config",
        "A declared harness roster requires the keys that reconcile it",
    ),
    (
        "cli-contract",
        "Version reports the release identity as text",
    ),
    (
        "mermaid-cli",
        "A selected file that cannot be read is refused",
    ),
    (
        "mermaid-cli",
        "A fenced Mermaid block with no diagram in it is not counted",
    ),
    (
        "mermaid-cli",
        "A class that sets no color role declares no color",
    ),
    (
        "mermaid-cli",
        "A class with a stroke and a text color but no fill is still checked",
    ),
    (
        "mermaid-cli",
        "A color set outside a class declaration is refused whatever its notation",
    ),
    (
        "mermaid-cli",
        "An unpaired edge-label delimiter ends the edge scan",
    ),
    (
        "mermaid-legibility",
        "A label is measured as it is seen, not as it is written",
    ),
    ("cli-contract", "An empty command line asks for a command"),
    (
        "word-budget",
        "An unusable surface glob is a configuration fault",
    ),
    ("word-budget", "Word-count inspection needs a file to count"),
    (
        "word-budget",
        "Word-count inspection refuses a file it cannot read",
    ),
    (
        "internal-link",
        "A link with an empty target names nothing to resolve",
    ),
    (
        "internal-link",
        "A Markdown source that holds no text is refused",
    ),
    ("internal-link", "A source that cannot be read is refused"),
    (
        "directory-map",
        "A selected location that is not a directory is refused",
    ),
    (
        "directory-map",
        "An excluded directory inside a mapped tree is listed but not inspected",
    ),
    ("directory-map", "A map section ends at the next heading"),
    (
        "directory-map",
        "A map entry may not reach past a sibling README",
    ),
    (
        "harness-parity",
        "A narrowed inspection refuses a harness the repository does not declare",
    ),
    (
        "harness-parity",
        "A repository that requires no capability server reconciles none",
    ),
    (
        "harness-parity",
        "A file that is not text is not an instruction source",
    ),
    (
        "harness-parity",
        "A file that cannot be opened still stops the run",
    ),
    (
        "harness-parity",
        "A file no rule here reads does not stop the run when it cannot be opened",
    ),
    (
        "harness-parity",
        "A file a prohibited glob names holding no text is not a source",
    ),
    (
        "harness-parity",
        "A file a prohibited glob names stops the run when it cannot be opened",
    ),
    (
        "harness-parity",
        "An unusable prohibited-source glob stops the instruction check",
    ),
    (
        "harness-parity",
        "A file directly under the canonical skills root is not a skill",
    ),
    (
        "harness-parity",
        "Only Markdown files directly under the agents root are agents",
    ),
    (
        "harness-parity",
        "A canonical agent without a declaration is invalid",
    ),
    (
        "harness-parity",
        "An agent adapter without a declaration is invalid",
    ),
    (
        "harness-parity",
        "A skill wrapper without a declaration diverges from the skill",
    ),
    (
        "harness-parity",
        "Only an adapter-shaped file under a harness agent directory is an adapter",
    ),
    (
        "harness-parity",
        "A canonical skill whose front matter is never closed is invalid",
    ),
    (
        "harness-parity",
        "A capability declaration RHINO cannot read is a divergence",
    ),
    (
        "harness-parity",
        "A capability declaration may nest the required server inside a list",
    ),
    (
        "harness-parity",
        "An empty harness roster reconciles a canon that is not there",
    ),
    (
        "internal-link",
        "An unusable excluded-source glob is a configuration fault",
    ),
    (
        "mermaid-cli",
        "A fenced block in another language is not a diagram",
    ),
    (
        "directory-map",
        "A map entry containing JSON metacharacters survives the JSON rendering",
    ),
    (
        "md-naming",
        "A kebab-case surface accepts a kebab-case name",
    ),
    (
        "md-naming",
        "A kebab-case surface refuses a name written any other way",
    ),
    (
        "md-naming",
        "A path-prefixed name encodes the directory it sits in",
    ),
    (
        "md-naming",
        "A path-prefixed name whose prefix does not encode its directory is refused",
    ),
    (
        "md-naming",
        "Each directory segment encodes by the rule its own words follow",
    ),
    (
        "md-naming",
        "A file at the surface root has no directory to encode",
    ),
    ("md-naming", "An exempt file is named by no style at all"),
    (
        "md-naming",
        "A later surface overrides an earlier one for the same file",
    ),
    (
        "md-naming",
        "Omitting the section refuses rather than assuming a convention",
    ),
    (
        "md-naming",
        "An unusable surface glob is a configuration fault",
    ),
    (
        "md-naming",
        "A path-prefixed surface needs the separator that joins its two halves",
    ),
    (
        "md-naming",
        "A kebab-case surface has no two halves to separate",
    ),
    ("md-frontmatter", "A surface's requirements are met"),
    (
        "md-frontmatter",
        "Front matter that opens and never closes is unusable",
    ),
    (
        "md-frontmatter",
        "A required key that is absent is a finding",
    ),
    (
        "md-frontmatter",
        "A file carrying no front matter is missing every required key",
    ),
    (
        "md-frontmatter",
        "A declared value set refuses anything outside it",
    ),
    (
        "md-frontmatter",
        "A declared value set accepts a value inside it",
    ),
    (
        "md-frontmatter",
        "A declared date key holds an ISO calendar date and nothing else",
    ),
    (
        "md-frontmatter",
        "A forbidden key is a finding wherever it appears",
    ),
    (
        "md-frontmatter",
        "A surface requiring nothing accepts a file with no front matter",
    ),
    (
        "md-frontmatter",
        "A later surface overrides an earlier one for the same file",
    ),
    (
        "md-frontmatter",
        "Files outside every declared surface carry no schema",
    ),
    (
        "md-frontmatter",
        "Omitting the section refuses rather than assuming a schema",
    ),
    (
        "md-frontmatter",
        "An unusable surface glob is a configuration fault",
    ),
    ("md-frontmatter", "A source that cannot be read is refused"),
    (
        "md-heading-hierarchy",
        "A hierarchy that descends one level at a time passes",
    ),
    (
        "md-heading-hierarchy",
        "A governed file with no level-1 heading is a finding",
    ),
    (
        "md-heading-hierarchy",
        "A second level-1 heading is a finding at the line it appears on",
    ),
    (
        "md-heading-hierarchy",
        "A heading dropping further than the declared jump is a finding",
    ),
    (
        "md-heading-hierarchy",
        "A wider declared jump permits what a narrower one refuses",
    ),
    (
        "md-heading-hierarchy",
        "A hash inside a fenced block is not a heading",
    ),
    (
        "md-heading-hierarchy",
        "A hash with no space after it is not a heading",
    ),
    (
        "md-heading-hierarchy",
        "A repository permitting any number of level-1 headings says so",
    ),
    (
        "md-heading-hierarchy",
        "The first heading establishes the level the rest are measured from",
    ),
    (
        "md-heading-hierarchy",
        "Files outside every declared surface carry no heading rules",
    ),
    (
        "md-heading-hierarchy",
        "Omitting the section refuses rather than assuming a structure",
    ),
    (
        "md-heading-hierarchy",
        "An unusable surface glob is a configuration fault",
    ),
    (
        "md-readme-index",
        "Every directory under a declared tree carries a README",
    ),
    ("md-readme-index", "A directory with no README is a finding"),
    (
        "md-readme-index",
        "A README with no directory map is still a README",
    ),
    (
        "md-readme-index",
        "An excluded directory inside a declared tree is not inspected",
    ),
    (
        "md-readme-index",
        "Directories outside every declared tree are not inspected",
    ),
    ("md-readme-index", "Each declared tree is walked"),
    (
        "md-readme-index",
        "A declared tree that holds nothing inspects nothing",
    ),
    (
        "md-readme-index",
        "Omitting the section refuses rather than assuming a tree",
    ),
    (
        "convention-emoji",
        "An emoji in a prohibited file is a finding at the line it sits on",
    ),
    (
        "convention-emoji",
        "The same code point in a file matching no prohibited glob is not a finding",
    ),
    ("convention-emoji", "Ordinary punctuation is not an emoji"),
    (
        "convention-emoji",
        "Each line carrying an emoji is reported once",
    ),
    (
        "convention-emoji",
        "A prohibited file that is clean passes and is still counted",
    ),
    (
        "convention-emoji",
        "Every declared prohibited surface is walked",
    ),
    (
        "convention-emoji",
        "An excluded directory holds nothing to inspect",
    ),
    (
        "convention-emoji",
        "A prohibited file that cannot be read is refused",
    ),
    (
        "convention-emoji",
        "Omitting the section refuses rather than assuming a prohibition",
    ),
    (
        "convention-emoji",
        "An unusable prohibited glob is a configuration fault",
    ),
    (
        "repo-config",
        "A configuration declaring none of the optional sections is complete",
    ),
    (
        "md-naming",
        "Files outside every declared surface carry no style",
    ),
    (
        "md-naming",
        "An unusable exemption glob is a configuration fault",
    ),
    (
        "md-heading-hierarchy",
        "A source that cannot be read is refused",
    ),
    (
        "convention-emoji",
        "A prohibited file that holds no text is refused",
    ),
    (
        "metadata",
        "A governance document carrying both required keys passes",
    ),
    (
        "metadata",
        "A workflow carries the name its own schema requires",
    ),
    (
        "metadata",
        "The path selects the schema, so the same key is right in one tree and wrong in another",
    ),
    (
        "metadata",
        "A skill declares the one optional key its schema allows",
    ),
    ("metadata", "An agent declares both of its optional lists"),
    (
        "metadata",
        "A required key that is absent is reported against the artifact",
    ),
    (
        "metadata",
        "An artifact carrying no front matter is missing every required key",
    ),
    (
        "metadata",
        "Front matter that opens and never closes is unusable",
    ),
    (
        "metadata",
        "A duplicate key fails before a decoder could discard one of them",
    ),
    ("metadata", "A key the path already supplies is refused"),
    ("metadata", "An explicit null is not a value"),
    ("metadata", "An empty value is not a value either"),
    (
        "metadata",
        "Canonical key order is enforced rather than preferred",
    ),
    (
        "metadata",
        "A description shorter than the floor is a description that says nothing",
    ),
    (
        "metadata",
        "A routing trigger written as a plain scalar is refused",
    ),
    (
        "metadata",
        "A trigger that repeats the description routes nothing",
    ),
    (
        "metadata",
        "A declared name that is not the path identity fails",
    ),
    (
        "metadata",
        "A workflow entrypoint README takes its name from its directory",
    ),
    ("metadata", "A tier outside the closed set is not a tier"),
    (
        "metadata",
        "A capability outside the portable vocabulary is refused",
    ),
    (
        "metadata",
        "Capabilities are listed in the canonical order rather than the author's",
    ),
    ("metadata", "An empty required array is an absent answer"),
    ("metadata", "A repeated array member is refused"),
    (
        "metadata",
        "A file outside every declared surface is not inspected",
    ),
    (
        "metadata",
        "Diagnostics sort by path, then rule, then field",
    ),
    (
        "metadata",
        "The metadata section is optional and its absence is refused rather than assumed",
    ),
    (
        "repo-config",
        "A complete tier mapping carries both of its fields",
    ),
    (
        "repo-config",
        "An unmapped tier is an answer rather than an omission",
    ),
    (
        "repo-config",
        "A broken tier mapping is refused before anything is generated",
    ),
    (
        "repo-config",
        "A tier mapping keyed by an unsupported harness is refused",
    ),
    (
        "repo-config",
        "A tier mapping keyed by a tier outside the closed set is refused",
    ),
    (
        "repo-config",
        "A mapping keyed by an agent name is not a tier mapping",
    ),
    (
        "metadata",
        "A trigger of more than three sentences stops being a trigger",
    ),
    (
        "metadata",
        "A name that is not a portable identifier is refused before it is compared",
    ),
    ("metadata", "A list key written as one value is refused"),
    (
        "metadata",
        "A single-value key written as a list is refused",
    ),
    (
        "metadata",
        "A description written as a literal block is refused",
    ),
    (
        "metadata",
        "A line inside the block that declares nothing is reported",
    ),
    (
        "metadata",
        "A horizontal rule further down the document is not front matter",
    ),
    (
        "metadata",
        "A blank line between declarations is not a declaration",
    ),
    ("metadata", "A quoted value is the value it quotes"),
    (
        "metadata",
        "A surface glob that cannot compile is a configuration fault",
    ),
    (
        "metadata",
        "A file that cannot be read refuses the whole run",
    ),
    (
        "metadata",
        "The JSON form carries the column and the field a finding is about",
    ),
];

/// Bindings this layer legitimately does not have. Each must name the concrete
/// boundary it cannot reach and the alternative proof that covers it.
pub const EXEMPTIONS: &[Exemption] = &[
    Exemption {
        feature: "convention-emoji",
        scenario: "A file that vanishes between the walk and the read is not a finding",
        boundary: "a real directory cannot be made to drop a file between the walk and the \
               read without a second process racing the one under inspection, so the \
               outcome would be decided by the scheduler rather than by the validator",
        alternative_proof: "the unit adapter drives the same `TreeError::NotFound` through the \
                        same port the disk tree returns it on, so the validator's answer to \
                        a vanished file is asserted at the seam where the two trees agree",
    },
    Exemption {
        feature: "harness-parity",
        scenario: "A file that vanishes between the walk and the read is not a finding",
        boundary: "a real directory cannot be made to drop a file between the walk and the \
               read without a second process racing the one under inspection, so the \
               outcome would be decided by the scheduler rather than by the validator",
        alternative_proof: "the unit adapter drives the same `TreeError::NotFound` through the \
                        same port the disk tree returns it on, so the validator's answer to \
                        a vanished file is asserted at the seam where the two trees agree",
    },
];
