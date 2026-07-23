# Phase 4 semantic-engine measurements

The semantic result queue previously reserved one slice-header slot per
package/check task. A Go slice header is 24 bytes on the measured arm64
platform, so 500 loaded packages × 106 selected checks reserved about 1.27 MB
before storing a finding. The queue is now bounded to the worker count (10 on
the measurement host, or 240 bytes) while dispatch and collection proceed
concurrently.

The static-call fact builder previously walked every SSA instruction twice:
once to count per-package calls and once to populate the index. It now builds
the same filtered index in one instruction pass. First-call-argument facts
also no longer retain a second position map; lookups use the first element of
the existing argument-slice index.
