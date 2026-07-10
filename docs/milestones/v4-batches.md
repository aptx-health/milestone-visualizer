# v4-batches — authored issue sets with derived state (spec: #27)

Dependency graph. Any agent may rewrite the block between the sentinels below.

Cross-milestone: state derivation gates on #6 (label contract); test emission
composes with #12 (merge-order); web rendering needs #13 (serve).

<!-- deps:start -->
```mermaid
flowchart LR
  28[#28 Batches data model]
  29[#29 State derivation + rollup]
  30[#30 testing: config + batch test emit]
  31[#31 serve: batch rendering]
  28 --> 29
  29 --> 30
  29 --> 31
```
<!-- deps:end -->
