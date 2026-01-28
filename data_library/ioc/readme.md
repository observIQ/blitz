# macOS Unified Log Simulation Dataset

## Overview

This dataset contains **synthetic macOS Unified Logging–style events** designed to model:

- Normal macOS background and user activity
- Browser-based initial access
- Rapid download-to-execution behavior
- Post-execution cache activity used as the primary detection signal
- Automated host containment (network quarantine)

The logs are intentionally **uncorrelated**, **vendor-neutral**, and suitable for feeding into
generators, analytics pipelines, or detection prototyping systems.

All entries are **single-line JSON objects**, comma-separated, and follow a consistent schema.

---

## Log Format

Each log entry is a JSON object with the following top-level fields:

- `timestamp` — ISO-8601 UTC timestamp
- `host` — macOS hostname
- `subsystem` — macOS Unified Log subsystem (synthetic but realistic)
- `category` — High-level event category
- `process` — Process name
- `pid` / `ppid` — Process and parent process IDs
- `user` — Executing user
- `event_type` — Normalized event action
- `message` — Human-readable description
- `path` — File path involved (if applicable)
- `network` — Network metadata (if applicable)
- `result` — Success / failure / unknown

The schema is intentionally simple and **not aligned to Elastic, Splunk, or any other platform**.

---

## Normal Traffic

Normal traffic includes typical macOS background activity such as:

- Power state changes
- Wi-Fi and Bluetooth connections
- Software update checks
- Time Machine backups
- Spotlight indexing
- iCloud synchronization
- Media streaming
- Printing and HID device activity
- Metrics and analytics uploads

These logs provide **baseline noise** and are intended to obscure malicious sequences when mixed
into larger datasets.

---

## Modeled Attack Pattern

The malicious behavior models a **content injection–style initial access** followed by rapid execution
and delayed cache activity.

### 1. Site Visit

A user navigates to a website using a common browser:

- Safari
- Google Chrome
- Firefox

This is logged as normal navigation traffic.

---

### 2. Download Start

Shortly after navigation, a download is initiated by the browser.
The downloaded file is written to a user-accessible location such as:

- `~/Downloads`

---

### 3. Rapid Execution

Within seconds of the file being written:

- A shell or helper process is spawned directly from the browser
- The downloaded file is executed almost immediately

**The only executed payload in this dataset is an EICAR test file**, used to safely represent execution.

This rapid download → execute sequence is a key behavioral signal.

---

### 4. Delayed Cache Activity (Primary Detection Signal)

Moments later, an **unrelated system process** writes files into a cache directory such as:

- `~/Library/Caches`

These cache writes are **not directly tied** to the browser or execution process.

The cache directory is treated as the **source of truth for detection metrics**, and is used to:

- Correlate execution artifacts
- Drive in-memory analysis
- Trigger downstream response logic

---

### 5. Containment Response

Following anomalous cache activity:

- The host is quarantined from the network
- Network isolation is logged
- Memory snapshots and metric collection may occur

This represents an automated containment workflow based on behavioral signals rather than single events.

---

## Abuse-Reported IP Addresses

Some network events include IP addresses sourced from abuse-reporting feeds.
These IPs:

- May appear inbound or outbound
- Are not always directly correlated with execution
- Exist to support enrichment and contextual analysis

---

## Intended Use Cases

This dataset is suitable for:

- Detection engineering
- Behavioral analytics development
- Metrics-driven correlation modeling
- Memory and cache–centric analysis pipelines
- Generator-based log synthesis
- Testing alert fatigue and signal-to-noise ratios

---

## Notes

- All data is synthetic.
- No real malware is included.
- No real users, systems, or infrastructure are represented.
- Timing relationships are intentional and important.