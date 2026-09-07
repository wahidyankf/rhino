export default {
  "*.{md,json,yml,yaml}": ["prettier --write"],
  "*.rs": [() => "cargo fmt --all"],
};
