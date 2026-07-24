# Phase 5 compact-sidecar decision

Decision: **no-go**.

The Phase 4 CPU profile attributes 9.03% cumulative CPU to
`cst.nodeTokenBounds`. A compact preorder span sidecar was then forced for
every SFTPGo file using the same fixed two-core, cold-Strider-cache,
warm-Go-cache environment:

| File-local check | Selective/no full sidecar | Full span sidecar | Change |
| --- | ---: | ---: | ---: |
| Syntax worker sum | 644 ms | 746 ms | +15.8% |
| Allocated bytes | 831 MB | 1,026 MB | +23.5% |

Construction and coexistence cost more than the indexed lookups saved. The
production-order span representation remains available for the suppression
candidate scan, where one construction replaces ranges for every candidate.
The normal syntax walk keeps direct node ranges and the non-allocating token
iterator.

The larger Phase 5 design—dense node IDs, child tables, and replacement of
`Pass.functions` maps—is rejected for this plan because the measured query
share is below the cost already demonstrated by the minimal span-only
prototype. Reconsider only with a new profile showing CST query work on the
critical path.
