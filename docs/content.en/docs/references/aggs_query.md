---
title: "Aggregation query"
date: 2024-08-22
weight: 4
---

## Aggregation via URL Parameters

The INFINI Framework allows for the dynamic construction of complex aggregations directly through URL query parameters. This provides a powerful and flexible way to perform data analysis without needing to construct a full JSON request body.

### Basic Structure

The basic syntax for defining an aggregation is as follows:

```
agg[<aggregation_name>][<aggregation_type>][<parameter>]=<value>
```

-   **`<aggregation_name>`**: A user-defined name for the aggregation (e.g., `products_by_brand`). This name will be the key for the aggregation results.
-   **`<aggregation_type>`**: The type of aggregation to perform (e.g., `terms`, `avg`, `sum`, `date_histogram`).
-   **`<parameter>`**: A parameter for the given aggregation type (e.g., `field`, `size`, `interval`).
-   **`<value>`**: The value for the parameter.

All parts of the name and value should be URL-encoded, especially if they contain special characters.

---

### Aggregation Types

#### 1. Terms Aggregation

Groups documents by the unique values in a field.

-   **`type`**: `terms`
-   **Parameters**:
    -   `field`: The field to group by.
    -   `size`: (Optional) The number of buckets to return.

**Example**: Get the top 5 product brands.

```http
GET /my_index/_search?agg[by_brand][terms][field]=brand.keyword&agg[by_brand][terms][size]=5
```

#### 2. Metric Aggregations

Perform calculations on the values of a specific field.

-   **`avg`**: Calculates the average.
-   **`sum`**: Calculates the sum.
-   **`min`**: Finds the minimum value.
-   **`max`**: Finds the maximum value.
-   **`cardinality`**: Calculates the number of unique values (approximate count).

**Example**: Calculate the average price of all products.

```http
GET /my_index/_search?agg[avg_price][avg][field]=price
```

**Example**: Count the number of unique users.

```http
GET /my_index/_search?agg[unique_users][cardinality][field]=user.id
```

#### 3. Percentiles Aggregation

Calculates one or more percentiles over a numeric field.

-   **`type`**: `percentiles`
-   **Parameters**:
    -   `field`: The numeric field.
    -   `percents`: (Optional) A comma-separated list of percentile values. Defaults to a standard set if not provided.

**Example**: Calculate the 50th, 95th, and 99th percentiles for API latency.

```http
GET /my_index/_search?agg[latency_percentiles][percentiles][field]=latency_ms&agg[latency_percentiles][percentiles][percents]=50,95,99
```

#### 4. Date Histogram Aggregation

Groups documents into buckets based on a date/time field.

-   **`type`**: `date_histogram`
-   **Parameters**:
    -   `field`: The date field.
    -   `interval`: The time interval for the buckets (e.g., `1h`, `1d`, `1M`).
    -   `format`: (Optional) A custom format for the date keys in the response.
    -   `time_zone`: (Optional) A time zone to apply.

**Example**: Group sales data by month.

```http
GET /my_index/_search?agg[sales_by_month][date_histogram][field]=order_date&agg[sales_by_month][date_histogram][interval]=1M
```

#### 5. Derivative Aggregation

A pipeline aggregation that calculates the derivative of a metric in a parent histogram aggregation.

-   **`type`**: `derivative`
-   **Parameters**:
    -   `buckets_path`: The path to the metric to be differentiated.

**Example**: Calculate the rate of change of document counts per month.

```http
GET /my_index/_search?agg[sales_per_month][date_histogram][field]=timestamp&agg[sales_per_month][date_histogram][interval]=month&agg[sales_per_month][aggs][sales_deriv][derivative][buckets_path]=_count
```

---

### Nested Aggregations

You can nest aggregations to perform more complex, multi-level analysis. The syntax for nesting is to add an `aggs` key.

```
agg[<parent_name>][aggs][<child_name>][<child_type>][<parameter>]=<value>
```

**Example**: Group products by brand, and then for each brand, calculate the average price.

```http
GET /my_index/_search?agg[by_brand][terms][field]=brand.keyword&agg[by_brand][aggs][avg_price][avg][field]=price
```

This creates a `terms` aggregation named `by_brand` and, within each brand bucket, an `avg` aggregation named `avg_price`.

### Handling Special Characters

If your aggregation name contains special characters (including non-ASCII characters), ensure it is properly URL-encoded.

**Example**: An aggregation named "供应商" (Supplier).

The name "供应商" is URL-encoded to `%E4%BE%9B%E5%BA%94%E5%95%86`.

```http
GET /my_index/_search?agg[%E4%BE%9B%E5%BA%94%E5%95%86][terms][field]=supplier.keyword
```

The framework will automatically decode the name, and the key in the response will be "供应商".

## Direct Build Example
```go
	q := orm.NewQuery()
	q.Must(orm.TermQuery("product_id", "12345"))
  aggs := map[string]orm.Aggregation{
    "sales_over_time": (&orm.DateHistogramAggregation{
      Field:    "sale_date",
      Interval: "1M",
    }).AddNested("sales_by_region", (&orm.TermsAggregation{
      Field: "region.keyword",
    }).AddNested("avg_sale", &orm.MetricAggregation{
      Field: "sale_amount",
      Type:  "avg",
    })),
  }
	q.Aggs = aggs
  // build final dsl
  dsl := BuildQueryDSL(q)
```