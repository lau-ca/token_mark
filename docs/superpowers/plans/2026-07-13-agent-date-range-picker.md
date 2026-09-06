# Agent Date Range Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/agent-users` use exactly the same combined date-time range picker and default time range as `/usage-logs/common`.

**Architecture:** Import the existing `CompactDateTimeRangePicker` and `getDefaultTimeRange` instead of duplicating UI or date logic. Keep the agent statistics API parameters and local filter state unchanged.

**Tech Stack:** React 19, TypeScript, TanStack Query, existing shadcn/Base UI components, Bun.

---

### Task 1: Reuse the usage-log time range

**Files:**
- Modify: `web/default/src/features/agents/index.tsx`

- [ ] Remove the two standalone `DateTimePicker` filter controls.
- [ ] Initialize `startDate` and `endDate` from `getDefaultTimeRange()`.
- [ ] Render one `CompactDateTimeRangePicker` with the same start/end/onChange contract used by common logs.
- [ ] Preserve page reset behavior when the time range changes.

### Task 2: Verify and commit

- [ ] Run frontend type checking and targeted oxlint.
- [ ] Run the production frontend build.
- [ ] Check the scoped diff and commit only the agent page change.
