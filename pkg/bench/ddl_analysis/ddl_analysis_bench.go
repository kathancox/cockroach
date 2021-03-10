// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package bench

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/tracing"
	"github.com/cockroachdb/cockroach/pkg/util/tracing/tracingpb"
)

// RoundTripBenchTestCase is a struct that holds the name of a benchmark test
// case for ddl analysis and the statements to run for the test.
// Reset must drop any remaining objects after the current database is dropped
// so setup and stmt can be run again.
type RoundTripBenchTestCase struct {
	name  string
	setup string
	stmt  string
	reset string
}

// RunRoundTripBenchmark sets up a db run the RoundTripBenchTestCase test cases
// and counts how many round trips the stmt specified by the test case performs.
func RunRoundTripBenchmark(b *testing.B, tests []RoundTripBenchTestCase) {
	defer log.Scope(b).Close(b)
	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			var stmtToKvBatchRequests sync.Map

			beforePlan := func(trace tracing.Recording, stmt string) {
				if _, ok := stmtToKvBatchRequests.Load(stmt); ok {
					stmtToKvBatchRequests.Store(stmt, trace)
				}
			}

			params := base.TestServerArgs{
				UseDatabase: "bench",
				Knobs: base.TestingKnobs{
					SQLExecutor: &sql.ExecutorTestingKnobs{
						WithStatementTrace: beforePlan,
					},
				},
			}

			s, db, _ := serverutils.StartServer(
				b, params,
			)
			sql := sqlutils.MakeSQLRunner(db)

			defer s.Stopper().Stop(context.Background())

			roundTrips := 0
			b.ResetTimer()
			b.StopTimer()
			for i := 0; i < b.N; i++ {
				sql.Exec(b, "CREATE DATABASE bench;")
				sql.Exec(b, tc.setup)
				stmtToKvBatchRequests.Store(tc.stmt, nil)

				b.StartTimer()
				sql.Exec(b, tc.stmt)
				b.StopTimer()

				out, _ := stmtToKvBatchRequests.Load(tc.stmt)
				r, ok := out.(tracing.Recording)
				if !ok {
					b.Fatalf(
						"could not find number of round trips for statement: %s",
						tc.stmt,
					)
				}

				// If there's a retry error then we're just going to throw away this
				// run.
				rt, hasRetry := countKvBatchRequestsInRecording(r)
				if hasRetry {
					i--
				} else {
					roundTrips += rt
				}

				sql.Exec(b, "DROP DATABASE bench;")
				sql.Exec(b, tc.reset)
			}

			b.ReportMetric(float64(roundTrips)/float64(b.N), "roundtrips")
		})
	}
}

// count the number of KvBatchRequests inside a recording, this is done by
// counting each "txn coordinator send" operation.
func countKvBatchRequestsInRecording(r tracing.Recording) (sends int, hasRetry bool) {
	root := r[0]
	return countKvBatchRequestsInSpan(r, root)
}

func countKvBatchRequestsInSpan(r tracing.Recording, sp tracingpb.RecordedSpan) (int, bool) {
	count := 0
	// Count the number of OpTxnCoordSender operations while traversing the
	// tree of spans.
	if sp.Operation == kvcoord.OpTxnCoordSender {
		count++
	}
	if logsContainRetry(sp.Logs) {
		return 0, true
	}

	for _, osp := range r {
		if osp.ParentSpanID != sp.SpanID {
			continue
		}

		subCount, hasRetry := countKvBatchRequestsInSpan(r, osp)
		if hasRetry {
			return 0, true
		}
		count += subCount
	}

	return count, false
}

func logsContainRetry(logs []tracingpb.LogRecord) bool {
	for _, l := range logs {
		if strings.Contains(l.String(), "TransactionRetryWithProtoRefreshError") {
			return true
		}
	}
	return false
}
