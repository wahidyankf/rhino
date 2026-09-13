export default {
  "*.{md,json,yml,yaml}": ["prettier --write"],
  // Exactly the staged Rust files, through stdin; see the script for why.
  "*.rs": (files) =>
    `bash scripts/format-staged-rust.sh ${files.map((file) => JSON.stringify(file)).join(" ")}`,
};
