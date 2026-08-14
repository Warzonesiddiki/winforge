package config

import (
	"fmt"
	"unicode/utf8"
)

// Per-field and per-list bounds for configuration.
//
// The 8 MiB whole-file cap in the loader stops a single enormous document, but
// it does not stop a document that is individually plausible and collectively
// abusive: one 8 MiB registry path, a tweak with a million operations, or a
// description long enough to wedge the dashboard. Every value below is a hard
// upper bound applied after JSON decoding and before any value reaches the
// engine, the executor, or the UI.
//
// Bounds are chosen from two sources, in this order:
//
//  1. The Windows platform limit, where one exists and is authoritative
//     (registry value names are capped at 16,383 characters; SCM service names
//     at 256). A configuration value larger than the platform accepts can only
//     ever fail at execution time, so rejecting it at load time is strictly
//     better.
//  2. Otherwise, generous headroom over the largest value in the embedded
//     catalog — no bound here is under 4x the observed maximum, and most are
//     far above it. The headroom matters because users may override any file;
//     a bound tight enough to reject a reasonable override would be a
//     regression, not a hardening.
//
// Lengths are measured in runes, not bytes, matching appmanager's package-ID
// validation, so a bound never depends on how many bytes a UTF-8 character
// happens to occupy.
const (
	// Identity and display fields shared across catalogs.
	maxIDLen          = 256  // catalog max 42
	maxNameLen        = 256  // catalog max 36
	maxCategoryLen    = 128  // catalog max 21
	maxDescriptionLen = 4096 // catalog max 121 (240 in playbooks.json)

	// Operation targets.
	maxRegistryPathLen      = 512   // catalog max 132; deep keys stay well inside
	maxRegistryValueNameLen = 16383 // Windows limit for a registry value name
	maxStringValueLen       = 16384 // catalog max 661
	maxServiceNameLen       = 256   // Windows SCM limit for a service name
	maxTaskPathLen          = 512   // catalog max 47
	maxAppxNameLen          = 256   // catalog max 47
	maxCommandLen           = 512   // catalog max 12; absolute paths still fit
	maxArgLen               = 1024  // catalog max 75

	// Raw JSON for a single operation value, bounding the decode itself rather
	// than the decoded result.
	maxRawValueBytes = 64 << 10 // catalog max 711 bytes

	// DNS fields.
	maxDnsProfileLen = 128 // catalog max 10
	maxDnsServerLen  = 64  // catalog max 15; ample for any IPv6 literal

	// Application fields.
	maxTagLen = 64 // catalog max 11

	// Per-list bounds. A list bound stops a catalog that is technically valid
	// field-by-field from becoming unbounded in aggregate.
	maxTweaks             = 4096 // catalog 163
	maxOperationsPerTweak = 512  // catalog max 19 (per operations/revert list)
	maxArgsPerOperation   = 64   // catalog max 7
	maxApplications       = 4096 // catalog 52
	maxTagsPerApp         = 32   // catalog max 2
	maxDnsPresets         = 256  // catalog 4
	maxProtectedServices  = 1024 // catalog 14
)

// checkLen enforces a rune-count bound on one field, reporting the field, the
// bound, and the actual size so an over-long override is diagnosable without
// echoing the entire offending value back into a log.
func checkLen(field, value string, max int) error {
	if n := utf8.RuneCountInString(value); n > max {
		return fmt.Errorf("%s exceeds the %d-character limit (%d characters)", field, max, n)
	}
	return nil
}

// checkCount enforces a bound on the length of a list.
func checkCount(field string, n, max int) error {
	if n > max {
		return fmt.Errorf("%s exceeds the limit of %d entries (%d entries)", field, max, n)
	}
	return nil
}
