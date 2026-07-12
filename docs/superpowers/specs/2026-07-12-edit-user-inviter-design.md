# Edit User Inviter Design

## Goal

Allow an administrator to edit a user's inviter relationship from the existing user edit drawer by entering an inviter user ID.

## Scope

- Add an inviter ID field to the update-only section of the default frontend user form.
- Submit the inviter ID through the existing user update API.
- Allow `0` to clear the inviter relationship.
- Validate the relationship on the backend before updating the user.
- Preserve the existing user-management role hierarchy and authorization rules.

## Behavior

The field accepts a non-negative integer. A positive value assigns that user as the inviter. Zero removes the inviter. The backend rejects negative values, the edited user's own ID, IDs that do not exist, and deleted users.

Changing the relationship only updates `inviter_id`. It does not grant, revoke, or recompute invitation rewards, invitation quota, historical invitation quota, or invitation counts. Those values represent historical accounting and must not be rewritten by an administrative metadata correction.

## Data Flow

1. The user edit drawer loads the latest user record through the existing detail endpoint.
2. The form maps `inviter_id` into a numeric input, defaulting to `0` when absent.
3. The existing update payload includes `inviter_id` only for update operations.
4. The update controller validates the requested inviter inside the existing update transaction.
5. The model update persists the validated `inviter_id` together with the other editable fields.
6. Existing cache invalidation, audit logging, drawer closing, and user-list refresh behavior remains unchanged.

## Authorization

No new route or permission is introduced. The feature uses the existing user update endpoint and its target-role checks: administrators may edit users below their role, while Root may edit administrators and common users.

## UI and Internationalization

The inviter ID input appears in the update-only user settings section. Its label, placeholder, description, and validation feedback use the existing frontend i18n system and are provided for all supported locales.

## Error Handling

Backend validation is authoritative. Invalid inviter IDs return the project's standard localized API error response. The frontend keeps the drawer open and displays the returned message using the existing update error path.

## Verification

- Backend tests cover assigning an existing inviter, clearing with zero, rejecting self-reference, and rejecting a missing inviter.
- Frontend type checking and linting cover the form schema, mapping, payload, and component changes.
- Run focused backend tests and the default frontend's repository-native checks; no end-to-end test is required.
