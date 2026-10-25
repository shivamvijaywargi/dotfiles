# 🧮 Raycast-Style Advanced Calculator Workflow for Alfred

A blazing-fast, zero-dependency, native Go-compiled Alfred Script Filter that replicates Raycast's elite multi-engine calculator. It combines arithmetic, inline currency conversions, base-60 time arithmetic, CSS typography scaling, time zones, and relative dates under a single prompt.

## 🚀 Quick Start & Key Bindings

- **Trigger Keyword:** `=` (configured with argument required, **no space**)
- **Primary Action (`Enter`):** Copies the formatted result to your clipboard.
- **Modifier Action (`Cmd + Enter`):** Copies the raw unformatted number (ideal for dropping straight into code or terminal scripts).
- **Modifier Action (`Alt / Option + Enter`):** Copies the full equation string (e.g., `32/10 USD in INR + 100 = 403.83 INR`).

---

## 📋 Supported Engines & Syntax Guide

### 1. Unified Math & Inline Currency

Combines standard scalar math with real-time currency conversions using Alfred's official currency cache or keyless API failovers.

- **Compound Expressions:** `32/10 USD in INR + 100` $\rightarrow$ `403.83 INR`
- **Percentage Modifiers:** `1000 + 18%` $\rightarrow$ `1,180`
- **Percentage Off / Discount:** `20% off 80` $\rightarrow$ `64`
- **Percentage Ratio ("as % of"):** `40 as % of 200` $\rightarrow$ `20%`
- **Shorthand Multipliers:** `7.8k INR in USD` $\rightarrow$ `82.15 USD`
- **Functions & Constants:** `sqrt(625)`, `abs(-45)`, `round(12.6)`, `pi`, `e`

### 2. Base-60 Time Arithmetic & Durations

Handles compound time spans and rolling over hours/minutes naturally.

- **Addition / Subtraction:** `3h 40m + 2h 20m` $\rightarrow$ `6h`
- **Timespan Conversion:** `145 mins to timespan` $\rightarrow$ `2h 25m`
- **Division:** `(3h 40m + 2h 20m) / 2` $\rightarrow$ `3h`

### 3. CSS Typography Units

Instantly toggles and computes font sizes across relative and absolute units.

- **Bare Unit Toggle:** `24px` $\rightarrow$ `1.5 rem` (or `1.5rem` $\rightarrow$ `24 px`)
- **Compound CSS Math:** `24px + 1.5rem in rem` $\rightarrow$ `3 rem`

### 4. Time Zones & World Clocks

Supports standard IANA time zones and common city aliases (`ldn`, `ist`, `nyc`, `tokyo`, `sydney`).

- **Current Time in Zone:** `time in tokyo` or `now in ldn`
- **Converted Time Across Zones:** `5pm ldn in ist` $\rightarrow$ `9:30 PM IST`

### 5. Date & Calendar Math

Computes relative offsets, weekdays, and countdown intervals from today.

- **Relative Date Offsets:** `today + 14 days` or `now - 2 weeks`
- **Weekday Target:** `monday in 3 weeks`
- **Days Until Deadline:** `days until 31 dec` or `days until 25 Jan 2027`

---

## ⚙️ Advanced Environment Variables

You can customize the workflow's behavior by adding variables in Alfred's workflow settings (`[x]` icon in the top right of the workflow editor):

- `BASE_REM_PX`: Overrides the default root font size for CSS calculations (default is `16`). Set to `10` if your project uses `1rem = 10px`.
- `DEFAULT_CURRENCY`: Overrides the target home currency for bare foreign lookups (defaults to `INR`).
