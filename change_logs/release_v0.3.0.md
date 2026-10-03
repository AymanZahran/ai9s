# Release v0.3.0

- The keyboard UI lists, filters, describes, resumes, and deletes sessions.
- The session list has no TITLE column and no two-letter agent column. NAME is the last column. `sort:title` sorts NAME.
- Session text is escaped before it is drawn. Control characters are left out of human output and are not passed to an agent CLI.
