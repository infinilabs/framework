/* Copyright © INFINI LTD. All rights reserved. */

package easysearch

// The search-response decode path is covered by core/elastic's
// TestDecodeSearchResult / TestDecodeHits — the module now uses
// elastic.DecodeSearchResult instead of a local helper. ProbeCluster and
// truncate likewise moved to core/elastic (client_provider.go) and are
// tested there.
