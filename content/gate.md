# Approval gate

You are at an approval gate. The artifact you produced must be approved before
the next phase may begin. This is a prompted contract — stop and ask.

1. Present what you wrote (path + a short summary), not a wall of text.
2. Update the active change's record in `.tomo/state.json` (`changes[<change>]`)
   for this phase: status `await-approval` (run `tomo state set-gate <phase>
   await-approval`, or `.tomo/bin/tomo state set-gate <phase> await-approval`).
   If the binary is unavailable, edit `.tomo/state.json` directly.
3. Ask the user to approve or request changes. Do not proceed past the gate on
   your own.
4. On approval: set status `approved` (run `tomo state set-gate <phase>
   approved`), append one line to `.tomo/log.md` (artifact path, date,
   approved), then continue to the next action.
5. On requested changes: incorporate the feedback, rewrite the artifact, and
   repeat the gate from step 2.

`settings.gates`: `block` (default) — do not proceed without explicit approval;
`prompt` — ask once, proceed only on explicit yes; `auto` — treat a go-ahead or
the user's move to the next command as approval.
