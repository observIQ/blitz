// Package bytediff is a test helper for verifying that log generator
// refactors in PIPE-975 PRs #2-11 preserve byte-exact CLI output.
//
// Each log-generator PR migrates one module to yield embed.LogRecord
// instead of building the wire-format bytes directly. The CLI path then
// converts LogRecord back to bytes via an adapter. This helper makes it
// easy for those PRs to assert that the post-refactor byte sequence
// matches what the pre-refactor module would have produced for the
// same RNG seed and input.
package bytediff
