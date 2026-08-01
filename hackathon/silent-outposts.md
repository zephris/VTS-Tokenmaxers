# Silent Outposts brief

## Context

The CS Building contains the damaged transmitters, Wi-Fi equipment, and relay racks that form the robots' communication network. Survivors gather in the Sunken Garden to decide how to respond to its broadcasts.

The network carries warnings, safe routes, supply requests, situation reports, and emergencies. Peacock interference can imitate legitimate senders, so neither a confident message nor a strong signal is automatically trustworthy.

## Chosen thread

Some outposts have stopped broadcasting because they fear their identities will be copied. The resulting silence has become dangerous: the network distrusts almost everything, genuine emergencies are missed, and nobody promptly notices when a previously reliable outpost disappears.

## Primary user and decision

The user is a robot coordinator in the Sunken Garden. They need to decide:

1. Which outposts require an immediate welfare check.
2. Which incoming emergency reports should be escalated.
3. What evidence supports that decision despite spoofed, stale, or incomplete data.

## Evidence in the supplied data

Outpost-Delta is the clearest silent-outpost example:

- Its last direct broadcast was a verified routine check from Guild Village at **2026-07-13 06:45**.
- On **2026-07-16 10:05**, Mini-Marv-01 reported no response from Delta for 48 hours.
- On **2026-07-22 11:05**, the compromised New Meridian identity broadcast a false all-clear for Guild Village.
- At **12:00** that day, Mini-Marv-01 rejected the all-clear because Delta was still silent and the claim could not be confirmed.

The wider log repeatedly shows genuine emergency calls followed by contradictory all-clear messages and delayed scout verification. This makes silence, conflicting reports, sender history, and response delay useful signals when viewed together.

## Focused MVP

Build a **silent-outpost watchlist and incident triage dashboard**:

- Rank outposts by time since last trusted broadcast and normal check-in behaviour.
- Show a timeline of recent messages, contradictions, missing check-ins, and scout verification.
- Surface genuine-looking emergencies that were not checked or were contradicted by suspicious all-clears.
- Let an LLM produce a short evidence-based incident brief and recommend a next action such as monitor, cross-check, dispatch scout, or escalate.

The LLM output should cite the broadcast IDs it used, clearly state uncertainty, and avoid inventing facts. Deterministic code should calculate timestamps, gaps, counts, and risk indicators; the LLM should explain and synthesise them.

## Success criteria

- A coordinator can identify a newly silent outpost quickly.
- Every alert explains the underlying evidence.
- Conflicting messages are visible together rather than judged in isolation.
- Unknown or missing information remains visibly uncertain.
- The UI satisfies the required web, data-visualisation, and meaningful-LLM components.

## Scope boundary

Other regions, optional side quests, and general-purpose features unrelated to silent-outpost detection are excluded.
