---
slug: safe-mutation
title: Safe Mutation
description: Change host state without losing work.
when_to_use: Before any command that writes.
commands: [host apply]
---

## Decide

Confirm the target is the one you resolved, not the one you assumed.

## Run

```sh
host apply --plan latest
```

## Save

Record the returned change ID; recovery needs it.

## Preconditions

A fresh plan exists and auth is valid.

## Recover

`host rollback <change-id>` restores the previous state.

## Next

Verify with `host status`, then see the audit guide.
