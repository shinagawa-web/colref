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
  /^goos:|^goarch:|^cpu:/ { print; next }
  /^pkg:/ { print; next }
  /│[[:space:]]*sec\/op[[:space:]]*│/    { current_metric = "time";   next }
  /│[[:space:]]*B\/op[[:space:]]*│/      { current_metric = "memory"; next }
  /│[[:space:]]*allocs\/op[[:space:]]*│/ { current_metric = "allocs"; next }
  /^[[:space:]]/ { next }
  /│/            { next }
  /^$/           { next }
  /^[^A-Za-z]/   { next }
  {
    # Strip uncertainty markers: "± ∞ ¹" (n=1) and "± 5%" (n≥6).
    gsub(/±[[:space:]]*[0-9.∞]+%?[[:space:]]*[¹²³⁴⁵⁶⁷⁸⁹⁰]*/, "")
    # Strip statistical annotations "(p=... n=...) ²".
    gsub(/\([^)]*\)[[:space:]]*[¹²³⁴⁵⁶⁷⁸⁹⁰]*/, "")

    if (NF < 4) next

    name  = $1
    delta = $NF

    # When benchstat cannot determine significance it outputs "~".
    # Compute the percentage ourselves from the old and new values so
    # the comment always shows a number instead of "~".
    if (delta == "~") {
      old_n = $2; gsub(/[^0-9.]/, "", old_n); old_n += 0
      new_n = $3; gsub(/[^0-9.]/, "", new_n); new_n += 0
      if (old_n > 0) {
        pct   = (new_n - old_n) / old_n * 100
        delta = (pct >= 0) ? sprintf("+%.2f%%", pct) : sprintf("%.2f%%", pct)
        sub(/[[:space:]]*~[[:space:]]*$/, "  " delta, $0)
      }
    }

    status = ""
    if (delta ~ /^\+[0-9.]+%$/) {
      pct = delta; sub(/^\+/, "", pct); sub(/%$/, "", pct); pct += 0
      if      (pct >= 50) { status = " ❌" }
      else if (pct >= 10) { status = " ⚠️" }
      else                { status = " ✅" }
    } else if (delta ~ /^-[0-9.]+%$/) { status = " ✅" }

    # Strip GOMAXPROCS suffix (-N) from individual benchmark names.
    if (name != "geomean") sub(/-[0-9]+[[:space:]]/, " ", $0)

    metric_label = ""
    if      (current_metric == "time")   { metric_label = " [time/op]" }
    else if (current_metric == "memory") { metric_label = " [memory/op]" }
    else if (current_metric == "allocs") { metric_label = " [allocs/op]" }

    print $0 status metric_label
  }
' "$INPUT_FILE" > "$OUTPUT_FILE"
