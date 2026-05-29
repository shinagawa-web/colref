---
title: Limitations
weight: 50
---

# Limitations

colref uses static AST analysis and cannot detect every reference pattern. References where the field name is constructed at runtime (e.g. `getattr(obj, field_name)`) are out of scope by design.

**If colref reports no references, treat it as "none found by the scanner" — not as a guarantee the column is unused.**

## Django

Detected: attribute access, `getattr(obj, "field")` / `attrgetter("field")` (labeled `[getattr]`), most ORM methods (`filter`, `exclude`, `get`, `Q`, `values`, `only`, `defer`, `order_by`, `F`, aggregates, etc.), and raw SQL strings.

Not detected: `getattr` with variable field name, `update_or_create`, `save(update_fields=[...])`, `_meta.get_field`, Django admin class attributes, DRF serializer fields, and form fields.

## Rails

Detected: attribute access, most ActiveRecord query/creation/update methods (`where`, `order`, `pluck`, `create`, `update`, `find_by`, etc.), Arel subscripts, and SQL string fragments.

Not detected: `read_attribute`, `send`, symbol subscript (`record[:field]`), `validates` declarations, and strong parameters.

## Full pattern breakdown

See [Detection patterns]({{< relref "detection-patterns" >}}) for the complete per-pattern table with examples and result labels.
