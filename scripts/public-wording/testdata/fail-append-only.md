Fixture for scripts/test-check-public-wording.sh. The statements below are deliberately wrong;
each line marked with a hit comment must be reported, and no other line may be.

- <!-- hit --> An append only log of every agent action.
- <!-- hit --> Append-only by design.
- <!-- hit --> Events land in an append
  only store.

Not a hit: rows the service can only append to and later prune.
