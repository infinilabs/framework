---
title: "Processor Reference"
weight: 16
---
# Processor Reference

One reference page per pipeline processor, generated from the framework source and kept in a uniform format: what it does, its configuration parameters, and an example. The same catalog is discoverable at runtime via `GET /pipeline/processors` (`?grouped=1` groups by category).

Record-scope processors operate on the current record inside [`for_each`](for_each/) sub-chains; pipeline-scope processors run against the pipeline context; the `consumer`/`for_each`/`queue_output` trio moves batches through queues.

## Framework

| Processor | Description |
|-----------|-------------|
| [`echo`](echo/) | Logs a configured message. |
| [`dag`](dag/) | Executes a DAG of processors with parallel and dependency-based execution. |
| [`for_each`](for_each/) | Splits a batch into records, runs a sub-chain per record, re-encodes the batch. |

## Queue

| Processor | Description |
|-----------|-------------|
| [`consumer`](consumer/) | Consumes a queue (by name or label selector) and exposes the batch to the pipeline. |
| [`queue_output`](queue_output/) | Appends the processed batch onto a target queue — the two-stage pipeline bridge. |

## Elasticsearch

| Processor | Description |
|-----------|-------------|
| [`bulk_indexing`](bulk_indexing/) | Bulk-indexes documents into Elasticsearch for high-throughput ingestion. |
| [`json_indexing`](json_indexing/) | Indexes JSON documents into Elasticsearch. |
| [`indexing_merge`](indexing_merge/) | Merges small documents into combined indexing requests before writing. |
| [`merge_to_bulk`](merge_to_bulk/) | Merges events into Elasticsearch bulk requests. |

## Output transport

| Processor | Description |
|-----------|-------------|
| [`otlp_export`](otlp_export/) | Ships the processed batch to any OTLP/gRPC collector. |

## General

| Processor | Description |
|-----------|-------------|
| [`http`](http/) | Sends HTTP requests to external services. |
| [`smtp`](smtp/) | Sends email notifications via SMTP. |
| [`replay`](replay/) | Replays recorded events for testing or reprocessing. |

## Parsing

| Processor | Description |
|-----------|-------------|
| [`dissect`](dissect/) | Fast, regex-free delimiter-based field extraction. |
| [`grok`](grok/) | Regex-based extraction with the `%{PATTERN:name}` syntax and a 350+ definition pattern library. |
| [`json`](json/) | Decodes a JSON string field into structured attributes. |
| [`xml`](xml/) | XML → map conversion. |
| [`csv`](csv/) | Parses a CSV line into named fields. |
| [`kv`](kv/) | Parses `key=value` pairs out of a string field. |
| [`url`](url/) | Splits a URL or `path?query` field into structured parts. |
| [`urldecode`](urldecode/) | URL-decodes string fields. |
| [`date`](date/) | Parses a timestamp from a field into the record's canonical `Timestamp`. |
| [`date_format`](date_format/) | Renders a timestamp field in any layout/timezone. |
| [`useragent`](useragent/) | Dependency-free User-Agent classifier. |
| [`syslog_decode`](syslog_decode/) | Decodes syslog numeric codes into names. |
| [`web_log`](web_log/) | Preset parsers for classic web server log formats. |
| [`string`](string/) | String-function toolbox: substring, split/join/concat, tests, regex replace. |
| [`decode_base64_field`](decode_base64_field/) | Base64-decodes a single string field. |
| [`decompress_gzip_field`](decompress_gzip_field/) | Gunzips a string or bytes field in place. |

## Transform

| Processor | Description |
|-----------|-------------|
| [`mutate`](mutate/) | Field-level mutations covering the Logstash mutate surface. |
| [`field_standardize`](field_standardize/) | Normalizes record keys to the canonical INFINI log data model naming. |
| [`otel_normalize`](otel_normalize/) | Maps source fields onto the canonical OTel log structure. |
| [`context_enrich`](context_enrich/) | Promotes collection context into the record's Fields. |
| [`extract_array`](extract_array/) | Pulls elements out of an array field. |
| [`decode_duration`](decode_duration/) | Converts Go duration strings into numeric durations. |

## Enrichment

| Processor | Description |
|-----------|-------------|
| [`geoip`](geoip/) | Geographical enrichment from a MaxMind mmdb database. |
| [`cidr`](cidr/) | Tests an IP field against a list of networks. |
| [`registered_domain`](registered_domain/) | Splits a domain into its registered domain via the Public Suffix List. |
| [`community_id`](community_id/) | Computes the Community ID flow hash for network events. |
| [`fingerprint`](fingerprint/) | Hashes fields into a stable deduplication identity. |
| [`enrich_es`](enrich_es/) | Joins the record against an Elasticsearch/Easysearch index. |
| [`pattern_tagger`](pattern_tagger/) | Tags records with the log pattern they belong to. |
| [`add_locale`](add_locale/) | Stamps events with the host's timezone abbreviation or UTC offset. |

## Routing & Governance

| Processor | Description |
|-----------|-------------|
| [`throttle`](throttle/) | Per-key rate limiting (token bucket); exceeding records are tagged or dropped. |
| [`sample`](sample/) | Probabilistic sampling by `ratio`. |
| [`clone`](clone/) | Duplicates the current record N times with optional mutations. |
| [`drop_event`](drop_event/) | Marks the current record to be dropped from the batch. |
| [`drop_fields`](drop_fields/) | Removes fields; with `keep: true` inverts into prune semantics. |
| [`redact`](redact/) | Masks sensitive substrings in the configured fields. |

## Sink & Advanced

| Processor | Description |
|-----------|-------------|
| [`pizza_bulk`](pizza_bulk/) | Ships the batch to a Pizza engine via its `/_bulk` endpoint. |
| [`script`](script/) | Arbitrary record transforms in ECMAScript (goja). |
