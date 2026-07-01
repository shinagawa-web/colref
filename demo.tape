# colref demo tape (https://github.com/charmbracelet/vhs)
#
# Regenerate the GIF with:
#   make demo               # builds the binary, then records
#   ./scripts/record-demo.sh
#
# The fixture lives in demo/ — edit it to change what the demo shows.

Output docs/static/demo.gif

Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 820
Set Theme "Dracula"
Set Padding 24
Set TypingSpeed 55ms

# Put the freshly built binary on PATH and move into the sample project,
# without showing the plumbing.
Hide
Type "export PATH=$PWD:$PATH"
Enter
Type "cd demo"
Enter
Type "clear"
Enter
Show

Sleep 800ms

# 1. A model with a field we're thinking about deleting: seo_title.
Type "cat blog/models.py"
Sleep 400ms
Enter
Sleep 2.5s

# 2. A plain text search is noisy — it also matches string literals in
#    `fields = [...]` and `search_fields = (...)`, plus the definition itself.
Type "grep -rn seo_title blog"
Sleep 400ms
Enter
Sleep 3.5s

# 3. colref parses the AST, skips comments and string literals, and reports
#    only the real references.
Type "colref check --orm django --model Article --field seo_title"
Sleep 400ms
Enter
Sleep 4s

Sleep 1.5s
