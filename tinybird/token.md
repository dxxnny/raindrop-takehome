---
title: Authentication
meta:
    description: Learn how to authenticate your requests to Tinybird.
headingMaxLevels: 2
---

# {% icon name="token" /%} Tokens

Tinybird uses tokens to authenticate CLI and API requests. Tokens protect access to your resources. Any operation to manage your resources using the CLI or REST API requires a valid token with the necessary permissions.

There are two types of tokens:

- [Static tokens](/forward/administration/tokens/static-tokens): Use them to perform operations on your account, like importing data, creating data sources, or publishing APIs using the CLI or REST API. Use them to read data as well, just be mindful of their permanent nature.
- [JSON Web tokens](/forward/administration/tokens/jwt): Use them to read from published endpoints that expose your data to an application, when you want to implement filtering per user via fixed parameters (RBAC) or to apply rate limiting for different end users of Tinybird endpoints.


## Authenticate from local

When working with [Tinybird Local](/forward/install-tinybird/local), you can authenticate by running `tb login`. For example:

```bash
tb login
```

The command opens a browser window where you can sign in. See [tb login](/forward/dev-reference/commands/tb-login). 

Credentials are stored in the `.tinyb` file. See [.tinyb file](/forward/dev-reference/datafiles/tinyb-file).

## Using default local tokens

Tinybird Local supports generating default user and workspace tokens for local development and testing. This is especially useful in CI/CD pipelines, testing, or automated setups where dynamically [fetching tokens](api-reference/__api-reference/token-api.md) adds unnecessary complexity.

### Generating tokens

You can generate valid local tokens using the following [command](/forward/dev-reference/commands/tb-local):

```bash
tb local generate-tokens
```
This command outputs valid tokens for both a user token and a workspace token. You can then export them as environment variables:

```bash
TB_LOCAL_WORKSPACE_TOKEN=$(tb --output=json local generate-tokens | jq -r '.workspace_token')
TB_LOCAL_USER_TOKEN=$(tb --output=json local generate-tokens | jq -r '.user_token')
tb local start
```

Alternatively, you can pass the generated tokens values in the arguments to `tb local start`. The generate-tokens command prints the tokens to your console; you must copy these values to use in the start command:

```bash
tb local generate-tokens
tb local start --user-token=<USER_TOKEN> --workspace-token=<WORKSPACE_TOKEN>
```
Once `tb local` has started, you can reference `$TB_LOCAL_USER_TOKEN` and `$TB_LOCAL_WORKSPACE_TOKEN` as environment variables in your API calls or scripts from that shell session.