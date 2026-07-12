# Agent Sales Settlement Design

## Goal

Add an agent-sales business identity, reporting page, configurable profit-sharing rules, and offline settlement confirmation while preserving the existing system role and invitation-reward systems.

Only the default frontend under `web/default` is in scope. The classic frontend is unchanged.

## Business Rules

- Not every user is an agent. An administrator explicitly enables or disables the agent identity from the user list.
- Agent identity is independent from the system `role` field. A user remains a common user or administrator for existing authorization behavior.
- A customer belongs to an agent when the customer's `inviter_id` points to that enabled agent.
- Changing a customer's inviter affects only consumption after the change time. Historical consumption remains assigned to the previous agent.
- Each agent has one platform retention rate that applies to all of that agent's groups.
- Each agent has independent gross-margin rates per group.
- Configuration changes affect only consumption after their effective time.
- An unconfigured group remains visible, but gross profit and agent earnings are zero and the row is marked as awaiting configuration. Later configuration does not recalculate earlier consumption.
- Agent earnings are separate from invitation rewards, wallet quota, and affiliate quota. The feature does not read or write `aff_quota`, `aff_history_quota`, or `aff_count`.
- Payment happens offline. Online settlement only records that a calculated amount was paid.

## Calculation

For an agent customer consuming an amount in a configured group:

```text
gross profit = consumption amount * group gross-margin rate
platform retained = gross profit * agent platform retention rate
agent earnings = gross profit - platform retained
pending settlement = cumulative agent earnings - settled amount
```

Example:

```text
consumption amount: 100
group gross-margin rate: 30%
platform retention rate: 9%
gross profit: 30
platform retained: 2.7
agent earnings: 27.3
```

All quota-to-currency display uses the project's existing quota formatting and currency configuration.

## Data Sources and Real-Time Statistics

The feature does not create a commission row for every consumption event. It calculates statistics from existing `quota_data` records when the page or settlement preview is requested.

The query flow is:

1. Select the agent and reporting time range.
2. Find customer assignment periods belonging to the agent.
3. Read matching `quota_data` rows by customer, group, and consumption time.
4. Match each time interval to the effective agent configuration version.
5. Apply the configured group gross-margin rate and platform retention rate.
6. Aggregate results to customer and group dimensions.
7. Subtract confirmed settlement totals to produce the pending amount.

Indexes cover the agent assignment period, configuration effective time, and the existing quota-data dimensions required by the query. No spreadsheet is used at runtime. The supplied pricing workbook only confirms the business terminology and calculation model.

## Data Model

### Agent Profile

`agent_profiles` stores the current business identity and administrative metadata:

- agent user ID, unique;
- enabled state;
- current platform retention rate for display and editing;
- administrator remark;
- creator and updater IDs;
- created and updated timestamps.

Enabling an agent requires a valid retention rate between zero and one. Disabling an agent prevents new customer assignment periods and new earnings after the disable time, while preserving historical reporting and settlement records.

### Configuration Versions

`agent_margin_versions` stores immutable effective versions:

- agent user ID;
- platform retention rate;
- effective-from timestamp;
- creating administrator and creation timestamp.

`agent_group_margins` stores the group rules belonging to a version:

- version ID;
- group name;
- gross-margin rate.

Saving agent settings creates a new complete version. It never overwrites an earlier version. Rates must be finite and between zero and one. Duplicate groups in one version are rejected.

### Customer Assignment History

`agent_customer_assignments` stores invitation-based assignment periods:

- customer user ID;
- agent user ID;
- effective-from timestamp;
- effective-to timestamp, nullable for the current period;
- creating administrator and creation timestamp.

`users.inviter_id` remains the source of current truth. When an administrator changes an inviter, the existing assignment period is closed and a new period is opened only if the new inviter is an enabled agent. The inviter update and assignment-history update occur in one database transaction.

When an existing user is first enabled as an agent, current invitees become assigned from the agent enable time. Consumption before enablement does not earn agent income because no active agent configuration existed.

### Settlement Records

`agent_settlements` stores one immutable record per confirmed offline payment:

- agent user ID;
- settlement start and cutoff timestamps;
- consumption amount;
- gross-profit amount;
- platform-retained amount;
- agent-earnings amount;
- payment note or reference;
- confirming administrator;
- confirmed timestamp.

Settlement amounts use the same integer quota unit as existing billing data so database arithmetic remains exact and compatible across SQLite, MySQL, and PostgreSQL. Percentage calculations use the project's safe quota conversion helpers and never produce negative amounts.

Confirming a settlement uses a transaction and locks the agent's settlement scope to prevent duplicate overlapping confirmations. A settlement covers all currently unsettled earnings through its cutoff. Partial arbitrary-amount settlement is not supported.

## Backend API and Authorization

A dedicated agent authorization check permits access when the caller is either:

- an administrator; or
- an enabled agent requesting their own data.

Administrators may:

- list enabled and disabled agent profiles;
- read an agent's profile and configuration history;
- enable, configure, or disable an agent;
- preview an agent settlement;
- confirm an offline settlement;
- view any agent's statistics and settlement records.

Agents may:

- read only their own summary, customer-group detail, and settlement history.

Agents may not view platform retention rates or retained amounts. The backend omits these fields for the agent-facing response rather than relying on frontend hiding.

API responses use dedicated DTOs rather than exposing database models directly. All scalar request fields that may be omitted use pointer types so explicit zero values remain distinguishable from absence.

## User Management UI

The default frontend user table adds an Agent column using the existing switch component.

- Turning the switch on opens an agent-settings drawer. The agent is enabled only after valid settings are saved.
- Editing an enabled agent opens the same drawer.
- Turning the switch off opens a confirmation dialog. Historical data remains available.
- The drawer reuses the current user drawer layout, form controls, group data source, validation messages, loading states, and toast patterns.

The drawer contains:

- enabled state;
- platform retention percentage;
- a gross-margin percentage for each selected system group;
- administrative remark;
- current version effective time.

The existing inviter ID field remains the way administrators assign customers. When the selected inviter is not an enabled agent, the relationship remains a normal invitation and produces no agent statistics.

## Agent Users Page

The new route uses the existing authenticated layout and is named Agent Users.

### Administrator View

- The page appears in the administrator navigation section.
- An agent selector at the top searches by user ID, username, or display name.
- Selecting an agent loads the profile, current rule summary, statistics, customer-group detail, and settlement history.
- Administrators can open the settings drawer and the settlement confirmation dialog.

### Agent View

- The page appears in the regular navigation section only for enabled agents.
- The agent enters their own page directly without an agent selector.
- The agent cannot edit configuration or confirm settlement.
- Platform retention percentage and retained amount are excluded.

### Shared Page Structure

The page reuses existing default-frontend components and patterns:

- `SectionPageLayout` for the page shell;
- dashboard `StatCard` for summary values;
- existing date-range controls and URL search parameters;
- existing data-table, pagination, search, loading, and empty-state patterns;
- existing `Tabs` for Earnings Detail and Settlement History;
- existing drawers, forms, switches, alerts, and confirmation dialogs.

Summary values include cumulative agent earnings, settled amount, pending settlement, customer count, and consumption amount. The administrator view may additionally show gross profit and platform retained amount.

The earnings-detail table aggregates only to customer and group:

- customer ID and username;
- group;
- consumption amount;
- group gross-margin rate;
- gross profit;
- platform retained, administrator only;
- agent earnings;
- configuration status.

The settlement-history table shows the covered time range, amount, payment reference, confirming administrator, and confirmation time.

## Settlement Flow

1. Administrator selects an agent and cutoff time.
2. Backend calculates all unsettled earnings through the cutoff using existing quota data, assignment periods, and effective configuration versions.
3. UI displays an immutable preview of consumption, gross profit, platform retained amount, and agent earnings.
4. Administrator enters an optional payment reference or note and confirms.
5. Backend recalculates inside the settlement transaction and saves one settlement record.
6. The page invalidates summary, detail, and settlement queries.

If no positive unsettled earnings exist, settlement confirmation is rejected. Disabled agents can still settle earnings generated before disablement.

## Error Handling and Boundaries

- A user cannot be enabled as an agent without a valid platform retention rate and at least one configured group.
- Root and administrator accounts may be marked as agents only if the existing user-management role hierarchy allows the administrator to manage that target user.
- A customer cannot be their own agent because inviter self-reference is already rejected.
- Missing group rules return zero earnings and an explicit unconfigured status.
- Deleted customers remain in historical statistics by stored assignment and quota-data user IDs.
- Deleted or disabled agents retain historical and settlement visibility for administrators.
- Time ranges and settlement cutoffs are validated and bounded.
- Overlapping assignment periods, duplicate effective configuration timestamps, overlapping settlements, and duplicate confirmations are rejected.
- All database queries and migrations support SQLite, MySQL, and PostgreSQL.

## Verification

Backend tests protect:

- per-agent independent rates;
- effective-time configuration changes;
- inviter changes affecting only future consumption;
- unconfigured groups producing zero earnings;
- real-time aggregation by customer and group;
- agent/admin response-field isolation;
- settlement preview and confirmation parity;
- duplicate and overlapping settlement prevention;
- disabled-agent historical settlement;
- cross-database-safe query construction and safe quota arithmetic.

Frontend tests and checks protect:

- user-list agent switch behavior;
- settings validation and version creation flow;
- administrator agent selection;
- direct self-view for agents;
- hidden platform-retention fields in the agent view;
- settlement preview and confirmation refresh;
- TypeScript type checking, targeted linting, and all supported locale keys.

No classic frontend work or end-to-end browser test is required unless separately requested.
