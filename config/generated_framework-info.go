package config

// Build-time placeholders (same convention as generated.go): the release
// pipeline regenerates this file with the real framework/vendor commit
// hashes. Committed N/A defaults keep fresh checkouts compiling — env.go
// references these constants, and without a committed fallback a clean
// clone of main fails to build.
const LastFrameworkCommitLog = "N/A"
const LastFrameworkVendorCommitLog = "N/A"
