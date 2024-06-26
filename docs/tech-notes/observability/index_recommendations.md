# Index recommendations on SQL Statistics
Last Update: February 2024

Original author: maryliag

This document provides an overview of how Index Recommendations are 
used for statement statistics. How frequent they're generated and how they're
stored. This document doesn't focus on the generation itself. For more information
on the index recommendation generation, check its [RFC](https://github.com/cockroachdb/cockroach/blob/master/docs/RFCS/20211112_index_recommendation.md).

Table of contents:

- [Overview](#overview)
- [Cache](#cache)
- [Cluster Settings](#cluster-settings)
- [Console](#console)

## Overview
When a statement is executed, we want to calculate index recommendations for it, in
case there are better indexes that can improve its execution. Generating an index
recommendation is a costly operation, so we don't want to generate it on each execution. 
At the same time, generating frequent index recommendations for the same plan would
likely generate the same results (unless there is a big increase/decrease in rows affected
by the query).
With this in mind, we use a cache to store the latest index recommendation for the most
recent statement fingerprints, avoiding a new generation on each execution.

The value of the recommendation can be found on the column `index_recommendation`
as a `STRING[] NOT NULL` on:
- `system.statement_statistics`
- `crdb_internal.statement_statistics`
- `crdb_internal_statement_statistics_persisted`

When recording a statement, it calls the function `ShouldGenerateIndexRecommendation`.
This function returns true if there was no index recommendation generated for the
statement fingerprint in the past hour and there was at least 5 executions 
(`minExecCount`) of it.
We don't generate index recommendations for statement fingerprints with 5 or fewer 
executions because we don't want to perform a heavy operation[^1] for a statement that
is barely executed.

Index recommendations for the SQL Statistics system are only generated for DML 
statements that are not internal.

It then calls `UpdateIndexRecommendations`. This function can make two types of updates:
1. A new index recommendation was generated: it updates the value on the cache and 
reset the last update time and the execution count.
2. No index recommendation was generated: increase the counter of execution count. The
counter is only increased if less than 5, since that is the count we care about. If the 
value is already greater than 5, no need to keep updating.

If a recommendation is generated for a new fingerprint, and we reached the limit
on the cache of how much we can store, it will try to remove any entries that have
the last update value older than 24hrs (`timeThresholdForDeletion`). It also needs 
to be at least 5 minutes (`timeBetweenCleanups`) between cleanups (to avoid cases where 
a lot of new fingerprints are created and could cause contention on the cache). If the
limit has reached and has been less the `timeBetweenCleanups` or no data older than
24hrs can be deleted, new entries won't be added to the cache.

## Cache
The Index Recommendation cache is a mutex map with corresponding info:
- Key (indexRecKey): statement fingerprint, database, plan hash
- Value (indexRecInfo): last generated timestamp, recommendations, execution count

## Cluster Settings
There are 2 cluster settings controlling this system:
- `sql.metrics.statement_details.index_recommendation_collection.enabled`:
enable/disable if the system will generate index recommendations. This is a safety
measure in case there is a performance degradation on this feature, and it can be
disabled. Default value is `true`.
- `sql.metrics.statement_details.max_mem_reported_idx_recommendations`: 
defines the maximum number of reported index recommendations info we store in the
cache defined previously. Default value is `5000`.

## Console
This information can be seen in a few different places:
- Statement Details page (Explain Plan tab): on the table on the page, there is a 
column for `Insights`. If a row shows that there are insights for that particular
plan, when clicking on it, it will display the recommendation.
- Insights (Workload): Any Insights with type `Suboptimal Plan` can be selected and
the index recommendation will be displayed on its details page.
- Insights (Schema): It will list the latest index recommendation per fingerprint.

All options above will display a button to create/alter/drop the index directly 
from the Console UI.

[^1]: The cost of generating index recommendations is highly variable. Index recommendations are generated by:
      1. Analyzing the query to find "hypothetical indexes" that may improve the performance of the query.
      2. Running the query optimizer as if the hypothetical indexes actually exist.
      3. Any hypothetical index in the final query plan becomes an index recommendation.

      Step 1 is not picky. Its goal is to cover all possible indexes that might help. 
      In general, the number of hypothetical indexes grows with the number of filtered columns 
      in the query.
      Step 2 can be fast for simple queries (<1ms), but the cost of optimization grows with 
      the number of joins, filters, and columns (>1s). 
      If Step 1 adds many hypothetical indexes, Step 2 will take longer because there are more query plans to explore. 
