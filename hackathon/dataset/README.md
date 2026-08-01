# Sunken Garden & CS Building Broadcast Dataset

## Overview
--------
This dataset contains robot broadcast records from the Sunken Garden & CS Building recovery hub after the Giant Peacock attack. The robots' scavenged relay network keeps transmitting despite peacock interference, damaged hardware and a growing trust problem between outposts.

The dataset contains:
- A log of individual broadcasts sent across the relay network (warnings, all-clears, supply requests, situation reports and emergencies)
- A summary of each sender's broadcast history and reliability over time

All timestamps are recorded in July 2026.

There are three files:
1. `broadcast_message_log.csv` — the raw broadcast log
2. `sender_history.csv` — a rollup of each sender/relay identity's track record
3. `marvs_logs.md` - the journal entries of Marv, a sender, which includes cryptic hints!

---

## broadcast_message_log.csv

### Column Descriptions

**broadcast_id**
Unit: Text ID (e.g. BC-001)
Description: A unique identifier for each broadcast.

**timestamp**
Unit: Date and time (YYYY-MM-DD HH:MM)
Description: The time the broadcast was sent.

**sender_id**
Unit: Categorical value
Description: The identity that sent the broadcast — a robot outpost, a relay identity such as Marv Mail or New Meridian, or one of the junior scout groups (Mini-Marv-01/02/03).

**location**
Unit: Categorical value
Description: The campus location the broadcast concerns (e.g. Sunken Garden, CS Building Foyer, Reynolds Court).

**broadcast_type**
Unit: Categorical value
Possible values: all_clear, warning, supply_request, situation_report, routine_check, emergency
Description: The kind of message being sent.

**message_text**
Unit: Free text
Description: The content of the broadcast. Some entries are blank — damaged transmission equipment occasionally fails to record or relay a full message.

**signal_strength**
Unit: Relative signal strength (0–100)
Description: A measure of how strong and clean the received signal was. Some entries are missing due to damaged sensors along the relay chain.

**cross_check_status**
Unit: Categorical value
Possible values: verified, disputed, unconfirmed, not_checked
Description: Whether the Mini-Marvs (or another outpost) manually cross-checked this broadcast against other reports, eyewitness accounts or past patterns.

**label**
Unit: Categorical value
Possible values: genuine, peacock_spoofed, outdated, unknown
Description: The ground-truth assessment of the broadcast — whether it was a real, accurate report; a peacock-faked broadcast; a genuine identity reporting stale/outdated information; or unknown (message unrecoverable).

---

## sender_history.csv

### Column Descriptions

**sender_id**
Description: The sender/relay identity or scout group, matching `sender_id` in the broadcast log.

**sender_type**
Possible values: robot_outpost, relay_identity, junior_scout_group
Description: The category of sender.

**first_seen / last_seen**
Unit: Date (YYYY-MM-DD)
Description: The first and most recent dates this sender broadcast during the logged period.

**total_broadcasts**
Description: Total number of broadcasts recorded from this sender in the log.

**broadcasts_verified_accurate / broadcasts_verified_false**
Description: How many of this sender's broadcasts were later confirmed accurate vs. false, based on cross-checking and outcomes.

**reliability_score**
Unit: 0–100
Description: An overall trust score for the sender, reflecting its track record.

**current_status**
Possible values: active, gone_quiet, suspected_compromised
Description: The sender's current operating state — still broadcasting normally, has stopped broadcasting, or is suspected of being spoofed/compromised by the peacocks.
