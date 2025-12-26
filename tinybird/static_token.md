---
title: Static tokens
meta:
    description: Static tokens are permanent and long-term.
headingMaxLevels: 2
---
# Static tokens

Static tokens are permanent and long-term. They're stored inside Tinybird and don't have an expiration date or time. They will be valid until deleted or refreshed. They're useful for backend-to-backend integrations, where you call Tinybird as another service.

## Default tokens (created by Tinybird)

All workspaces come with a set of default tokens:

{% table %}
  * Token name {% width="30%" %}
  * Description
  ---
  * `Workspace admin token`
  * The Workspace token. This token is workspace-bound and enables any operation over it. Note: only workspace admins have access to this token.
  ---
  * `Admin <your-email> token`
  * The CLI token. This token is managed by Tinybird for you and the CLI uses it to authenticate via 'tb login' (stores it locally in the `.tinyb` file).
  ---
  * `User token`
  * Required only for certain operations through the API (like creating workspaces) - the system will ask you for it if required. 
{% /table %}

See below how to [list exiting tokens](#list-existing-tokens)

## User created tokens

Users can create additional tokens with different authorization scopes. This allow you to grant granular access to resources or to create tokens for CI/CD or for other purposes.

There are two types of static tokens:

- **[Resource-scoped tokens](#resource-scoped-tokens):** grant specific permissions on specific resources, such as reading from a given endpoint or appending to a given data source. Created in _.pipe_ and _.datasource_ files and managed via deployments.
- **[Workspace and Org. level tokens](#other-tokens):** tokens with workspace or organization-wide scopes: `WORKSPACE:READ_ALL`, `ADMIN`, `TOKENS` or `ORG_DATASOURCES:READ`. Created and managed via the CLI or API.

### Resource-scoped tokens

When you create a resource-scoped token, you can define which resources can be accessed by that token, and which methods can be used to access them.

They are managed using the `TOKEN` directive in data files, with the following structure `TOKEN <token_name> <scope>`. Scopes are `READ` or `APPEND`.

For example in a .datasource file:

```tb {% title="example.datasource" %}
TOKEN app_read READ
TOKEN landing_read READ
TOKEN landing_append APPEND
SCHEMA >
    ...
```

For .pipe files, the behavior is the same:

```tb {% title="example.pipe" %}
TOKEN app_read READ

NODE node_1
SQL >
    %
    SELECT
```

{% callout type="info" %}
Resource-scoped tokens are created and updated through deployments. Tinybird will keep track of which ones to create or destroy based on all the tokens defined within the data files in your project. You can find the deployment-generated tokens in the Workspace UI or by running tb (--cloud) token ls.
{% /callout %}

The following scopes are available for resource-scoped tokens:

{% table %}
  * Token Scope (API) {% width="30%" %}
  * Token Scope (CLI) {% width="30%" %}
  * Description
  ---
  * `DATASOURCES:READ:datasource_name`
  * `TOKEN <token_name> READ` in `.datasource` files
  * Grants the token read permissions on the specified data source(s)
  ---
  * `DATASOURCES:APPEND:datasource_name`
  * `TOKEN <token_name> APPEND` in `.datasource` files
  * Grants the token permission to append data to the specified data source.
  ---
  * `PIPES:READ:pipe_name`
  * `TOKEN <token_name> APPEND` in `.pipe` files
  * Grants the token read permissions for the specified pipe.
{% /table %}

{% callout type="info" %}
When adding the `DATASOURCES:READ` scope to a token, it automatically grants read permissions to the [quarantine data source](/forward/get-data-in/quarantine) associated with it.
{% /callout %}
{% callout type="caution" %}
SQL filters (`:sql_filter` suffix) are not supported in Tinybird Forward. Use fixed parameters in JWTs for row-level security instead.
{% /callout %}

### Other tokens

These are operational tokens that are not tied to specific resources. Run the following command in the CLI:

```bash
tb token create static new_admin_token --scope <scope> 
```

The following scopes are available for general tokens:

{% table %}
  * Value
  * Description
  ---
  * `TOKENS`
  * Grants the token permission to create, delete or refresh tokens.
  ---
  * `ADMIN`
  * Grants full access to the workspace. Use sparingly.
  ---
  * `WORKSPACE:READ_ALL`
  * Grants read access to all workspace resources: datasources, pipes, `tinybird.*` service data sources, and `system.*` tables. Particularly useful for BI Tools.
  ---
  * `ORG_DATASOURCES:READ`
  * Grants the token read access to organization service datasources.
{% /table %}

## List existing tokens

You can review your existing tokens using:

- **CLI**: Run `tb token ls` to list all tokens in your workspace. See [tb token](/forward/dev-reference/commands/tb-token) for reference.  
- **UI**: Navigate to the "Tokens" section in the sidebar of your Tinybird workspace.

## Refresh a static token

To refresh a token, run the `tb token refresh` command. For example:

```bash
tb token refresh my_static_token
```

See [tb token](/forward/dev-reference/commands/tb-token) for more information.

## Delete a static token

### Resource-scoped tokens

Resource-scoped tokens are updated through deployments. Tinybird will keep track of which ones destroy based on all the tokens defined within the data files in your project. 

So, to remove a resource-scoped token, just **delete it from the data files and make a deployment.** The changes will be applied automatically.


### Other tokens

To delete [other tokens](/forward/administration/tokens/static-tokens#other-tokens) that are not tied to specific resources, run the following command:

```bash
tb token rm <token_name>
```

See [tb token](/forward/dev-reference/commands/tb-token) for more information.
