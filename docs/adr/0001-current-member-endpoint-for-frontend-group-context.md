# Current member endpoint for frontend group context

Campus Vault backend should provide a current-member endpoint that returns the authenticated member's identity, roles, and group memberships before the frontend member flows are built. The frontend needs this contract to choose an Active Group for group-scoped catalog, cart, checkout, and administration workflows; relying on all-groups or admin-only user endpoints would make member workflows ambiguous and permission-sensitive.
