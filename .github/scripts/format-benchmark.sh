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

  # Print all pkg: lines so results are attributed to their package.
  /^pkg:/ { print; next }

  # Track current metric from benchstat table headers.
  /│[[:space:]]*sec\/op[[:space:]]*│/ { current_metric = "time"; next }
  /│[[:space:]]*B\/op[[:space:]]*│/   { current_metric = "memory"; next }
  /│[[:space:]]*allocs\/op[[:space:]]*│/ { current_metric = "allocs"; next }

  # Skip table separator/header rows (leading whitespace or │).
  /^[[:space:]]/ { next }
  /│/            { next }

  # Skip blank lines and footnote lines (footnotes start with Unicode
  # superscripts which are not plain ASCII letters).
  /^$/  { next }
  /^[^A-Za-z]/ { next }

  # Process benchmark result lines and geomean lines.
  # Both start with an ASCII letter (e.g. "ParseModels-4", "Scan-4", "geomean").
  {
    gsub(/±[[:space:]]*∞[[:space:]]*[¹²³⁴⁵⁶⁷⁸⁹⁰]*/, "")
    gsub(/\([^)]*\)[[:space:]]*[¹²³⁴⁵⁶⁷⁸⁹⁰]*/, "")
    delta = $NF
    status = ""
    if (delta ~ /^\+[0-9.]+%$/) {
      sub(/^\+/, "", delta); sub(/%$/, "", delta); percent = delta + 0
      if (percent >= 50)     { status = " ❌" }
      else if (percent >= 10) { status = " ⚠️" }
      else                    { status = " ✅" }
    } else if (delta ~ /^-[0-9.]+%$/) { status = " ✅" }
    else if (delta == "~")             { status = " ✅" }
    metric_label = ""
    if      (current_metric == "time")   { metric_label = " [time/op]" }
    else if (current_metric == "memory") { metric_label = " [memory/op]" }
    else if (current_metric == "allocs") { metric_label = " [allocs/op]" }
    print $0 status metric_label
  }
' "$INPUT_FILE" > "$OUTPUT_FILE"
