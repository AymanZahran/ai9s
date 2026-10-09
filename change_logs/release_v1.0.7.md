# Release v1.0.7

- Free-text search matches a word as typed in the name, title, session id, directory, branch, model, or harness. Letters with gaps no longer match, and the transcript is not searched.
- `2` and `:harnesses` list harnesses. `:agents` and `:providers` still open that view. The HARNESS column shows the program name.
- Enter on a harness, directory, branch, or model keeps that choice while `/` searches inside it. Esc clears the search first. A second Esc returns to that list.
- `$HARNESS` is the program name on a plugin. `$AGENT` stays the id.
