# Managing Permissions

## Understanding the System

- **Roles** - Groups of permissions (e.g., `global_admin`, `member`)
- **Permissions** - Specific actions (e.g., `view_items`, `manage_users`)
- **Scopes** - Where permissions apply (`global` or `group`)

## Adding New Permissions

**Create migration:**
```bash
make migrate-create name=add_new_permission
```

**Migration structure:**
```sql
-- +goose Up
INSERT INTO permissions (name, description) VALUES
    ('permission_name', 'Description');

INSERT INTO role_permissions (role_name, permission_name)
VALUES ('role_name', 'permission_name');

-- +goose Down
DELETE FROM role_permissions WHERE permission_name = 'permission_name';
DELETE FROM permissions WHERE name = 'permission_name';
```

**Apply migration:**
```bash
make migrate-up
```

## Using Permissions

**In OpenAPI spec (`api/swagger.yaml`):**
```yaml
security:
  - BearerAuth: []
  - OAuth2: [permission_name]
```

**In handlers:**
```go
hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "permission_name", nil)
if !hasPermission {
    // Return 403 Forbidden
}
```

**For group-specific permissions:**
```go
hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "permission_name", &groupID)
```

## User Management

**Create user with role:**
```bash
./bin/hashgen email password connection_string role_name scope
```

**Check user permissions:**
```sql
SELECT DISTINCT p.name, p.description, ur.scope, ur.scope_id
FROM permissions p
JOIN role_permissions rp ON p.name = rp.permission_name
JOIN user_roles ur ON rp.role_name = ur.role_name
WHERE ur.user_id = $1;
```

## Current Structure

**View permissions:**
```sql
SELECT name, description FROM permissions ORDER BY name;
```

**View role mappings:**
```sql
SELECT r.name as role, p.name as permission
FROM roles r
JOIN role_permissions rp ON r.name = rp.role_name
JOIN permissions p ON rp.permission_name = p.name
ORDER BY r.name, p.name;
```