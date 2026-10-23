// Package bytediff is a test helper for verifying that log generator
// refactors preserve byte-exact CLI output.
//
// A log generator migrated to yield embed.LogRecord, instead of building
// the wire-format bytes directly, relies on the CLI path to convert each
// LogRecord back to bytes through an adapter. This helper asserts that the
// resulting byte sequence matches what the generator produced before the
// migration, for the same RNG seed and input.
package bytediff
