# Silent Outposts dataset guide

## Download

The shared folder contains **Sunken Garden and CS Building.zip**:

- Folder: <https://drive.google.com/drive/folders/1PhiIBZ-ngRpJv91hGCu_9vg9zpwcYfbp?usp=sharing>
- Direct download: <https://drive.usercontent.google.com/download?id=1rbe3Ixi8lTIv_tZ7hFbLogMdKBzmrHB1&export=download&confirm=t>

The archive contains three useful files.

## `broadcast_message_log.csv`

The log contains **300 broadcasts** from **2026-07-10 06:15** through **2026-11-20 09:57**.

| Field | Meaning |
| --- | --- |
| `broadcast_id` | Unique message ID. |
| `timestamp` | Send time. |
| `sender_id` | Outpost, relay identity, or Mini-Marv sender. |
| `location` | Campus location concerned. |
| `broadcast_type` | `all_clear`, `warning`, `supply_request`, `situation_report`, `routine_check`, or `emergency`. |
| `message_text` | Free-text content; some values are blank due to damaged transmission. |
| `signal_strength` | Relative strength from 0–100; some values are missing. |
| `cross_check_status` | `verified`, `disputed`, `unconfirmed`, or `not_checked`. |
| `label` | Retrospective ground truth: `genuine`, `peacock_spoofed`, `outdated`, or `unknown`. |

Useful totals:

| Measure | Count |
| --- | ---: |
| Genuine | 207 |
| Peacock-spoofed | 50 |
| Outdated | 38 |
| Unknown | 5 |
| Emergency broadcasts | 37 |
| Missing message text | 5 |
| Missing signal strength | 6 |

Treat `label` as training/evaluation data. A real live message would not arrive with its ground-truth label already known, so the UI should not leak it into a supposedly real-time decision.

## `sender_history.csv`

This is a nine-row rollup with:

- Sender identity and type.
- First and last seen dates.
- Total broadcasts.
- Verified-accurate and verified-false counts.
- Reliability score from 0–100.
- Current status: `active`, `gone_quiet`, or `suspected_compromised`.

The row for **Outpost-Delta** marks it `gone_quiet`, with a last-seen date of **2026-07-13** and a reliability score of **90**.

## `marvs_logs.md`

The journal provides qualitative hints. The entries most relevant to this challenge say, in summary:

- Silence itself can be a warning when a regular voice disappears.
- A genuine emergency can lose to a soothing contradictory message if checked too late.
- Missing records should not automatically be treated as neutral.

These hints can inform features, but user-facing conclusions should still point back to concrete broadcast records.

## Data-quality warnings

- The archive README says all timestamps are in July 2026, but the broadcast log continues through November 2026.
- `sender_history.csv` omits five senders present in the broadcast log: `Outpost-Epsilon`, `Outpost-Theta`, `Outpost-Zeta`, `Mini-Marv-04`, and `Mini-Marv-05`.
- Delta has two direct rows in the broadcast log, while its history row reports three total broadcasts.
- The sender-history snapshot therefore should not be treated as fully current or internally complete. Derive last-seen and activity-gap metrics from the raw broadcast log where possible.

## Suggested derived fields

| Field | How to derive it |
| --- | --- |
| `hours_since_last_seen` | Current or selected analysis time minus sender's latest broadcast. |
| `expected_checkin_gap` | Typical interval between that sender's earlier routine broadcasts. |
| `silence_ratio` | Current gap divided by expected gap. |
| `unresolved_emergency` | Emergency with no timely verified follow-up or response. |
| `conflicting_all_clear` | Nearby all-clear that conflicts with an emergency, warning, silence alert, or scout report. |
| `evidence_ids` | Broadcast IDs supporting each alert and LLM summary. |

Avoid using a single arbitrary silence threshold for every sender. A sender should look suspicious relative to its own normal cadence and the surrounding evidence.
