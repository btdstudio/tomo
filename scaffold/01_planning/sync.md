# Fetching tickets

**Not configured yet.** Replace this with how to read a ticket from your PM tool. When
a ticket came from a planning workspace, its `external` field holds the item's id, and
the analyst reads the live item before writing requirements. `tomo update` won't
overwrite your edits.

## fetch

Read the live item by its external id. For GitHub Issues, where the id looks like
`<owner>/<repo>#<number>`:

```bash
gh issue view <number> --repo <owner>/<repo> --json title,body,state,labels,comments
```

If the fetch fails, use the snapshot in `00_issues/`, and say so in the requirements.
