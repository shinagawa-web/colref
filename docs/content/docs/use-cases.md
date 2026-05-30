---
title: Use cases
weight: 25
---

# Use cases

colref started as a deletion check — "is it safe to remove this column?" — but the underlying operation (find every reference to a specific field) is useful in more situations. This page documents the ones that come up regularly.

## Schema cleanup

### Dead column detection

Instead of starting from a specific deletion candidate, work backwards from the schema. Take a column from your migration history you don't recognize, run colref, and see what comes back.

```sh
colref check --orm django --model Article --field legacy_content
```

Zero results: it goes on the deletion candidate list. You're not verifying a decision — you're generating the list to decide about. This is faster than schema archaeology, and it turns "I think this might be unused" into "not referenced in Python code."

### Rename impact mapping

Before renaming a column, enumerate every location that needs updating:

```sh
colref check --orm rails --model User --field email_address
```

The output becomes the codemod target list. You're not asking "can I rename this?" — you're answering "what changes when I do?"

### Expand-contract verification

Zero-downtime column removal follows a two-phase pattern: first add the new column and migrate data (expand), then drop the old column once all code is updated (contract). Before the contract phase, confirm the old column is no longer referenced:

```sh
colref check --orm django --model Order --field legacy_status
```

This is a precondition check, not a guess. If colref returns references, the contract migration is not safe to run yet.

## CI integration

If a pull request migration drops or renames a column, run colref automatically and fail the build (or post a comment) if references remain.

```sh
# detect dropped columns from migration diff (Rails example)
DROPPED=$(git diff origin/main -- "db/migrate/*.rb" \
  | grep "remove_column" \
  | grep -oE ":[a-z_]+" \
  | tail -1 \
  | tr -d ':')

for field in $DROPPED; do
  output=$(colref check --orm rails --model YourModel --field "$field")
  echo "$output"
  if echo "$output" | grep -q "^References found"; then
    echo "colref: references to $field still exist — remove them before dropping the column" >&2
    exit 1
  fi
done
```

"Can we delete this?" stops being a judgment call and becomes a gate. This works well as one rule inside a PR review bot: detect the migration, run the check, block if references are found.

The output direction matters here (see [Output direction](#output-direction) below): blocking on "references found" is reliable. That evidence is real.

## Audit and governance

### Sensitive column access map

Where is `email` referenced? Where does `password` appear in code?

```sh
colref check --orm django --model User --field email
colref check --orm django --model User --field password
```

The output shows every place in the codebase that touches these fields. This doesn't replace a proper data flow audit, but it surfaces access points quickly — useful when preparing for a privacy review or mapping data flows for compliance documentation.

### API exposure check (Rails)

Combine colref with a search for serialization methods to check whether a sensitive column reaches the JSON response:

```sh
# find where the column is accessed
colref check --orm rails --model User --field ssn

# check for serializer declarations
grep -rn -E "as_json|to_json|attributes.*ssn" app/
```

If colref finds accesses and a serializer declaration includes the field name, investigate whether that output is exposed externally.

## Code understanding

### Reference navigation

New to a codebase and want to understand how a column is used?

```sh
colref check --orm django --model Article --field status
```

Returns every place `status` is referenced, with file and line number. Faster than searching, and without the noise.

### Blast radius estimation

Before touching a column, estimate the scope of changes:

```sh
colref check --orm rails --model Post --field published_at
```

A column with 40 references across 15 files carries more change risk than one with 3 references in a single file. The reference count is a signal, not a guarantee — but it tells you whether to expect a small patch or a broad refactor.

## Output direction

Before deciding how much to automate any of these, understand how colref fails. It can miss references (dynamic access, templates, admin class attributes — see [Limitations]({{< relref "limitations" >}})), but it does not fabricate them.

This creates an asymmetry:

| Result | Interpretation | Reliable for automation? |
|--------|---------------|--------------------------|
| References found | The field is definitely used | Yes — block deletion, CI fail |
| No references found | Not found by the scanner | No — candidate only, requires human verification |

**"References found" is a hard signal.** You can use it to block a CI pipeline with confidence.

**"No references found" is a lower bound.** Dynamic access, templates, and other unscanned patterns may still reference the column. Treat zero results as a starting point for human review, not as proof the column is unused.

This asymmetry shapes what you should automate. Blocking on evidence is safe. Triggering deletion on absence of evidence is not.
