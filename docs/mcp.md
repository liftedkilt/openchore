# Managing OpenChore from an AI assistant (MCP)

OpenChore has a built-in [Model Context Protocol](https://modelcontextprotocol.io)
server. Once an assistant such as Claude is connected, a parent can manage the
household in plain language:

- "What chores does everyone have, and is anything duplicated or not assigned to anyone?"
- "Give Zoe and Natalie 'Unload the dishwasher' on school days, done by 8:30."
- "Natalie's doing piano now, not Zoe, and move it to Tuesdays and Thursdays."
- "When the dryer finishes, the first kid to fold the laundry should get 10 points."
- "Excuse Zoe's chores today, she's sick."
- "Approve everything waiting for approval that has a photo."

The assistant doesn't need to know how schedules, categories or triggers work.
The server explains OpenChore's model to it when it connects.

## Connecting

The endpoint is `https://<your-openchore>/api/mcp`. It uses MCP's streamable HTTP
transport and answers each request with plain JSON (no session or event stream
to keep open), so it works behind proxies and MCP gateways.

1. In OpenChore, open **Manage → Settings → API Tokens** and create a token,
   e.g. "Claude". Copy it; it's shown once.
2. Add the server to your MCP gateway (or any MCP client that can send headers):

   | Setting | Value |
   |---|---|
   | URL | `https://<your-openchore>/api/mcp` |
   | Transport | Streamable HTTP |
   | Header | `Authorization: Bearer <token>` |

3. Connect your assistant to the gateway as usual.

A parent's own session also works (`Authorization: Bearer ocs1.…`), but tokens
are what integrations should use. Kids' sessions are refused.

> **A token has full parent access.** Anything the assistant does through it is
> something a parent could do in the app. Revoke it under API Tokens to cut the
> assistant off. Changes made with a token are recorded against the household's
> first parent profile (the oldest parent account), because tokens don't belong
> to a person.

## What it can do

Every tool goes through OpenChore's own REST API in-process, so the same
validation, points rules, webhooks and Discord notifications apply as in the
web app.

**Start here**
- `household_overview`: people, every chore and who does it when, grouped into
  plain language ("Zoe: weekdays, due 08:30"), triggers, plus cleanup hints:
  chores nobody is assigned and chores with near-identical names.

**Chores**: `create_chore` (can assign in the same step), `update_chore` (changes
only what you mention), `assign_chore` (people × weekdays / every N days / a
date, with times and late rules), `unassign_chore`, `update_schedule`,
`get_person_chores`, `complete_chore`, `uncomplete_chore`, `excuse_chore`,
`delete_chore`, triggers (`create_trigger`, `fire_trigger`, ...).

**People**: add, edit, pause for vacation, reset PINs, remove.

**Approvals, points, rewards, streaks**: approve/reject, balances and history,
manual adjustments, decay, rewards with per-kid visibility and prices,
redemptions, savings goals, streak milestones.

**Reports and setup**: reports and AI summaries, settings, config export,
webhooks and their delivery logs, API tokens, sign-in providers and options,
AI and read-aloud configuration.

### Unassigning keeps history

Deleting a schedule also deletes its completion history, which reports and
streaks are built from. So `unassign_chore` retires schedules by default: they
end yesterday (or on `last_day`) and stay in the record. Schedules that never
came up are removed outright. Pass `mode: "delete"` to really delete. The same
goes for moving a chore to someone else or to different days: retire, then
assign. `household_overview` hides ended schedules unless asked
(`include_ended`).

### Not available through MCP

- **Signing in and first-run setup**: browser flows.
- **Photo uploads**: files can't travel through tool arguments. Approving,
  rejecting and completing without a photo all work.
- **A kid's own actions**: redeeming a reward and saving toward a goal act as
  the signed-in kid, and a token has no kid behind it. Parents can still see
  redemptions and goals and undo a redemption.
- **Personal preferences** someone sets for themselves (e.g. their wall-display
  line colour).

A test (`TestMCPCoversAPI`) walks the router and fails if a new endpoint has
neither a tool nor a reason here, so the MCP server keeps up with the API.

## Protocol notes

- Protocol revisions 2025-11-25, 2025-06-18, 2025-03-26 and 2024-11-05.
- `POST` only; `GET` returns 405 because there's no server-initiated stream.
- Capabilities: tools. Tool annotations set `readOnlyHint: true` on read-only
  tools. Every other tool is `destructiveHint: true` (the MCP default), because
  some clients and gateways auto-approve tools marked non-destructive. The one
  exception is `complete_chore`, which is additive and undone with
  `uncomplete_chore`. Keep approval prompts on for everything else, especially
  API tokens, webhooks, settings, sign-in and AI configuration.
- Tool failures (bad arguments, API errors such as "category must be required,
  core, or bonus") come back as results with `isError: true`, so the assistant
  can read the message and correct itself.
