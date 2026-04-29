#!/bin/bash
# Format benchmark comparison results with status symbols

set -e

INPUT_FILE="$1"
OUTPUT_FILE="$2"

if [[ ! -f "$INPUT_FILE" ]]; then
  echo "Error: Input file '$INPUT_FILE' does not exist" >&2
  exit 1
fi

if [[ ! -s "$INPUT_FILE" ]]; then
  echo "Warning: Input file '$INPUT_FILE' is empty. No benchmark comparison available." >&2
  echo "No benchmark comparison data available." > "$OUTPUT_FILE"
  exit 0
fi

awk '
  # Always print environment header lines.
  /^goos:|^goarch:|^cpu:/ { print; next }

  # Print all pkg: lines.
  /^pkg:/ { print; next }

  # Track current metric from benchstat table headers.
  /│[[:space:]]*sec\/op[[:space:]]*│/    { current_metric = "time";   next }
  /│[[:space:]]*B\/op[[:space:]]*│/      { current_metric = "memory"; next }
  /│[[:space:]]*allocs\/op[[:space:]]*│/ { current_metric = "allocs"; next }

  # Skip table separator/header rows (leading whitespace or │) and blank/footnote lines.
  /^[[:space:]]/ { next }
  /│/            { next }
  /^$/           { next }
  /^[^A-Za-z]/   { next }

  # Process benchmark result lines and geomean lines.
  # Both start with an ASCII letter (e.g. "ParseModels-4", "Scan-4", "geomean").
  {
    # Strip uncertainty markers (± ∞ ¹) and statistical annotations ((p=... n=...) ²).
    gsub(/±[[:space:]]*∞[[:space:]]*[¹²³⁴⁵⁶⁷⁸⁹⁰]*/, "")
    gsub(/\([^)]*\)[[:space:]]*[¹²³⁴⁵⁶⁷⁸⁹⁰]*/, "")

    # Require at least name, old value, new value, and delta.
    if (NF < 4) next

    name    = $1
    old_val = $2
    new_val = $3
    delta   = $NF

    # Strip GOMAXPROCS suffix (-N) from individual benchmark names.
    if (name != "geomean") sub(/-[0-9]+$/, "", name)

    # Determine status symbol from delta.
    status = ""
    if (delta ~ /^\+[0-9.]+%$/) {
      pct = delta
      sub(/^\+/, "", pct); sub(/%$/, "", pct); pct = pct + 0
      if      (pct >= 50) { status = " ❌" }
      else if (pct >= 10) { status = " ⚠️" }
      else                { status = " ✅" }
    } else if (delta ~ /^-[0-9.]+%$/) {
      status = " ✅"
    } else if (delta == "~") {
      status = " ✅"
    }

    metric_label = ""
    if      (current_metric == "time")   { metric_label = " [time/op]" }
    else if (current_metric == "memory") { metric_label = " [memory/op]" }
    else if (current_metric == "allocs") { metric_label = " [allocs/op]" }

    printf "%-20s  %10s  %10s  %8s%s%s\n", name, old_val, new_val, delta, status, metric_label
  }
' "$INPUT_FILE" > "$OUTPUT_FILE"
