---
title: "Query URL Parameters"
weight: 60
---

# Query URL Parameters

The query URL parameters can be used in many places (e.g., HTTP API endpoints, internal service calls, debug consoles). 
We use them to **unify how queries are received and processed** across different components of the system. 
This provides a powerful, composable, and human-readable way to construct both **full-text search** and **structured filters**, 
while also supporting advanced features like **fuzziness**, **field selection**, and **pagination**.

## 🔧 Query URL Parameters

These URL parameters are used to construct a rich and dynamic search query.

| Name                   | Type         | Description                                                                 | Example                                                                 |
|------------------------|--------------|-----------------------------------------------------------------------------|-------------------------------------------------------------------------|
| `query`                | `string`     | The main query string. Supports field boosting (`field^boost:value`).      | `query=title^2:search engine`                                           |
| `filter`               | `string[]`   | One or more filter clauses. Can be negated with `-` or `!`. Supports:      | `filter=status:active`, `filter=-exists(deleted_at)`                   |
|                        |              | - `field=value`, `field!=value`, `field>=x`, `field<y`                     | `filter=age>=18`, `filter=tag!=archived`                                |
|                        |              | - `exists(field)`                                                           | `filter=exists(status)`                                                |
| `sort`                 | `string`     | Sort rules separated by comma. Each rule is `field[:asc|desc]`.            | `sort=published_at:desc,_score`                                         |
| `from`                 | `int`        | Pagination offset.                                                          | `from=20`                                                               |
| `size`                 | `int`        | Number of results to return.                                                | `size=10`                                                               |
| `fuzziness`            | `int`        | Fuzziness level for the query (0–5).                                        | `fuzziness=3`                                                           |
| `default_operator`     | `string`     | Operator between terms if not specified (`AND` or `OR`).                   | `default_operator=AND`                                                 |
| `default_fields`       | `string`     | Comma-separated list of fields used as fallback for both query and filter. | `default_fields=title,description`                                     |
| `default_query_fields` | `string`     | Comma-separated list of fields used only for full-text search.             | `default_query_fields=title,body`                                      |
| `default_filter_fields`| `string`     | Comma-separated list of fields used only for filters.                      | `default_filter_fields=status,tag`                                     |
| `_source_includes`     | `string`     | Comma-separated fields to include in `_source`.                            | `_source_includes=title,author`                                        |
| `_source_excludes`     | `string`     | Comma-separated fields to exclude from `_source`.                          | `_source_excludes=internal_notes,raw_data`                              |

---

## 🧠 Filter Syntax Summary

| Syntax                     | Meaning                              | Example               |
|----------------------------|--------------------------------------|-----------------------|
| `field=value`              | Term query                           | `status=active`       |
| `field!=value`             | Negated term                         | `status!=deleted`     |
| `field>=value`             | Range query (greater than or equal)  | `views>=1000`         |
| `field<value`              | Range query (less than)              | `age<30`              |
| `any(value1,value2)`       | Terms filter (any term can be match) | `any(tag1,tag2,tag3)` |
| `exists(field)`            | Field existence check                | `exists(tags)`        |
| `-filterExpr` / `!filterExpr` | Negate any filter expression         | `!exists(deleted_at)` |



## 🚀 Example

### Simple Search
```
GET /search?query=go+language&default_fields=title,description
```

### Filtered and Sorted
```
GET /search?query=distributed+search&filter=status=active&sort=_score:desc,created_at:desc
```

### Advanced with Fuzziness and Range
```
GET /search?query=search&fuzziness=3&filter=age>=18&filter=!exists(deleted_at)&size=20
```

---

## 🧩 Notes

- Filters are **ANDed together** by default.
- Negated filters use either `-` or `!` prefix.
- If `query` is not provided, only filters will be applied.
- Supports combining query string logic and JSON body input (`BuildQueryDSLOnTopOfDSL` handles merging).
- Values in filters are auto-casted to `int`, `bool`, or `string`.

Example usage:
```go
	q := orm.NewQuery().Must(
		orm.ShouldQuery(
			orm.MatchQuery("lang", "en"),
			orm.MatchQuery("lang", "zh"),
		),
		orm.MustNotQuery(
			orm.TermQuery("deleted", true),
		),
	).Size(10).SortBy(
		orm.Sort{Field: "score", SortType: orm.DESC},
	)
```
