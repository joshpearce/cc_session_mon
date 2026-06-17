# Testing the agent-count metric

A repeatable way to drive the session list's `agents` column (`peak / active-last-min`)
to a high number using a **real** Claude Code subagent fan-out — real transcripts on
disk, not synthesized files.

## How the metric reacts (so you know what to watch)

The column reads `MAX_CONCURRENT / ACTIVE_LAST_MINUTE`:

- **Peak (left):** the most subagents whose activity spans overlapped at one instant,
  over the session's whole life. Every `Agent`/`Task` call writes a real
  `…/<session>/<id>/subagents/agent-<id>.jsonl`; the peak is the max overlap of those
  files' first→last record spans. To raise it, maximize agents running *at the same
  time* — one big parallel batch beats many sequential ones.
- **Active (right):** subagents whose last record is within the trailing 1-minute
  window, evaluated against the wall clock at render. It climbs while the fan-out runs
  and decays to `0` within ~30s of the session going quiet (the refresh tick interval).

Token usage is irrelevant to this metric, so the prompt below keeps every subagent at
near-zero tokens — fast and cheap, just lots of them.

## Setup

1. Run the monitor: `make run` (or `nix run`).
2. In a **separate** terminal, start a Claude Code session inside a directory the
   monitor watches — this repo works, since its projects dir is the local one.
3. Watch that session's row in the monitor while you paste a prompt below.

## The prompt — start here

Paste this into the watched Claude Code session:

```
Launch 12 subagents in a SINGLE message so they run concurrently. Use the
`general-purpose` agent type (or `ed3d-basic-agents:haiku-general-purpose` if it is
available — it is the cheapest). Give every subagent this exact task, varying only the
number so their labels differ:

  "You are load-test agent N of 12. Return only the word DONE. Do not use any tools and
   do not read any files."

The goal is maximum simultaneous agents at near-zero token cost, so do not give them
real work. Report back the peak concurrency you achieved once they all return.
```

## Dialing it up

Re-run with a larger batch to find the ceiling. Keep them in **one message** — that is
what creates simultaneity:

- `12` → `30` → `60` → `120` agents per message.

The harness caps how many subagents truly run at once, so beyond that cap the extras
queue; their spans still partially overlap, so the peak keeps climbing but with
diminishing returns. The point where the left number stops growing as you raise the
batch size is your effective concurrency ceiling.

To sustain a high **active** count (right number) rather than a one-shot spike, ask for
several back-to-back batches:

```
Run 5 back-to-back waves. Each wave: launch 30 subagents in a single message, each with
the task "Return only the word DONE; use no tools." Start the next wave as soon as the
current one returns. Keep waves contiguous so the active-in-last-minute count stays
high the whole time.
```

## Expected behavior

- During the fan-out: left number rises toward your batch size / the harness cap; right
  number tracks how many are live.
- After it finishes: within ~30s the right number falls to `0` while the left number
  stays at the lifetime peak (e.g. `30/0`) — confirming peak is a lifetime figure and
  active is wall-clock relative.
- A session that spawned no subagents shows `—`.

## Notes

- If the count seems to lag, remember updates land on fsnotify writes plus a 30s tick;
  the active number only re-evaluates against the clock on a render, so worst-case decay
  lag is ~30s.
- This is a throwaway load test — the spawned subagents do nothing and leave only their
  transcripts behind.
