module.exports = {
  extends: ["@commitlint/config-conventional"],
  // Dependabot writes compare URLs that exceed body-max-line-length (100).
  // The PR title stays conventional; squash-merge uses that title.
  ignores: [(message) => /Signed-off-by: dependabot\[bot\]/.test(message)],
};
