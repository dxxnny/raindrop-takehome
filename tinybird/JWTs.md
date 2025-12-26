---
title: JSON Web tokens (JWTs)
meta:
    description: JWTs are signed tokens that allow you to securely authorize and share data between your application and Tinybird.
headingMaxLevels: 2
---
# JSON Web tokens (JWTs)

JWTs are signed tokens that allow you to securely and independently authorize and consume data from Tinybird.

Unlike static tokens, **JWTs are not stored in Tinybird**. They're created by you, inside your application, and signed with a shared secret between your application and Tinybird. Tinybird validates the signature of the JWT, using the shared secret, to ensure it's authentic.

A great use case for JWTs is when you want to allow your app to call Tinybird API endpoints directly from the browser without proxying through your backend.

The typical pattern looks like this:

1. A user starts a session in your application.
2. The frontend requests a JWT from your backend.
3. Your backend generates a new JWT, signed with the Tinybird shared secret, and returns to the frontend.
4. The frontend uses the JWT to call the Tinybird API endpoints directly.

## JWT payload

The payload of a JWT is a JSON object that contains the following fields:

{% table %}
  * Key {% width="10%" %}
  * Example Value {% width="10%" %}
  * Required {% width="10%" %}
  * Description
  ---
  * workspace_id
  * {% user("workspaceId") %}
  * Yes
  * The UUID of your Tinybird workspace, found in the workspace list.
  ---
  * name
  * frontend_jwt
  * Yes
  * Used to identify the token in the `tinybird.pipe_stats_rt` table, useful for analytics. Doesn't need to be unique.
  ---
  * exp
  * 123123123123
  * Yes
  * The Unix timestamp (UTC) showing the expiry date & time. After a token has expired, Tinybird returns a 403 HTTP status code.
  ---
  * scopes
  * [{"type": "PIPES:READ", "resource": "requests_per_day", "fixed_params": {"org_id": "testing"}}]
  * Yes
  * Used to pass data to Tinybird, including the Tinybird scope, resources and fixed parameters.
  ---
  * scopes.type
  * PIPES:READ or DATASOURCES:READ
  * Yes
  * The type of scope, for example `PIPES:READ`. See [JWT scopes](#jwt-scopes) for supported scopes.
  ---
  * scopes.resource
  * t_b9427fe2bcd543d1a8923d18c094e8c1 or top_airlines
  * Yes
  * The ID or name of the pipe that the scope applies to, like which API endpoint the token can access.
  ---
  * scopes.fixed_params
  * {"org_id": "testing"}
  * No
  * Valid for scope `PIPES:READ`. Pass arbitrary fixed values to the API endpoint. These values can be accessed by pipe templates to supply dynamic values at query time. 
  ---
  * scopes.filter
  * "org_id = 'testing'"
  * No
  * Valid for scope `DATASOURCES:READ`. Passes a WHERE filter that will be appended to the specified scope resource.
  --- 
  * limits
  * {"rps": 10}
  * No
  * You can limit the number of requests per second the JWT can perform. See [JWT rate limit](#rate-limits-for-jwts).
{% /table %}

Check out the [JWT example](#jwt-example) to see what a complete payload looks like.

## JWT algorithm

Tinybird always uses HS256 as the algorithm for JWTs and doesn't read the `alg` field in the JWT header. You can skip the `alg` field in the header.

## JWT scopes

{% table %}
  * Value
  * Description
  ---
  * `PIPES:READ:pipe_name`
  * Gives your token read permissions for the specified pipe. Use `fixed_params` to filter by the pipe parameters.
  ---
  * `DATASOURCES:READ:datasource_name`
  * Gives your token read permissions for the specified datasource. Use `filter` to filter by the data source columns.
  ---
{% /table %}

## JWT expiration

JWTs can have an expiration time that gives each token a finite lifespan.

Setting the `exp` field in the JWT payload is mandatory, and not setting it results in a 403 HTTP status code from Tinybird when requesting the API endpoint.

Tinybird validates that a JWT hasn't expired before allowing access to the API endpoint.

If a token has expired, Tinybird returns a 403 HTTP status code.

## JWT fixed parameters

Fixed parameters allow you to pass arbitrary values to the API endpoint. These values can be accessed by pipe templates to supply dynamic values at query time.

For example, consider the following API Endpoint that accepts a parameter called `org` that filters by the `org_id` column:

```tb {% title="example.pipe" %}
SELECT fieldA, fieldB FROM my_ds WHERE org_id = '{{ String(org) }}'
TYPE ENDPOINT
```

The following JWT payload passes a parameter called `org` with the value `test_org` to the API endpoint:

```json {% title="Example fixed parameters" %}
{
  "type": "PIPES:READ",
  "resource": "requests_per_day",
  "fixed_params": {
      "org": "test_org"
  }
}
```

This is particularly useful when you want to pass dynamic values to an API endpoint that are set by your backend and must be safe from user tampering. A good example is multi-tenant applications that require row-level security, where you need to filter data based on a user or tenant ID.

{% callout %}
The value for the `org` parameter is always the one specified in the `fixed_params`. Even if you specify a new value in the URL when requesting the endpoint, Tinybird always uses the one specified in the JWT.
{% /callout %}

## JWT filters

Filters allow you to pass WHERE clauses to the data source queries. The filter has to be a valid `WHERE` clause that will be automatically appended to the data source at query time.

For example, consider the following data source with an `org_id` column:

```tb {% title="example.datasource" %}
SCHEMA >
    `timestamp` DateTime `json:$.timestamp`,
    `org_id` String `json:$.org_id`,
    `action` LowCardinality(String) `json:$.action`,
    `version` LowCardinality(String) `json:$.version`,
    `payload` String `json:$.payload`

ENGINE MergeTree
ENGINE_SORTING_KEY org_id, timestamp
```

The following JWT payload passes a filter for the data source:

```json {% title="Example filter in a scope" %}
{
  "type": "DATASOURCES:READ",
  "resource": "events",
  "filter": "org_id = 'testing'"
}
```

This is particularly useful when you want to pass dynamic filters to queries. A good example is multi-tenant applications that require row-level security, where you need to filter data based on a user or tenant ID.

## JWT example

Consider the following payload with all [required and optional fields](#jwt-payload):

```json {% title="Example payload" %}
{
    "workspace_id": "{% user("workspaceId") %}",
    "name": "frontend_jwt",
    "exp": 123123123123,
    "scopes": [
        {
            "type": "PIPES:READ",
            "resource": "requests_per_day",
            "fixed_params": {
                "org_id": "testing"
            }
        },
        {
            "type": "DATASOURCES:READ",
            "resource": "events",
            "filter": "org_id = 'testing'"
        }
    ],
    "limits": {
      "rps": 10
    }
}
```

Use the workspace admin token as your signing key (`TINYBIRD_SIGNING_KEY`), for example:

```json {% title="Example workspace admin token" %}
p.eyJ1IjogIjA1ZDhiYmI0LTdlYjctND...
```

With the payload and admin token, the signed JWT payload would look like this:

```json {% title="Example JWT" %}
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ3b3Jrc3BhY2V...
```

## JWT limitations

The following limitations apply to JWTs:

- You can't refresh JWTs individually from inside Tinybird as they aren't stored in Tinybird. You must do this from your application, or you can globally invalidate all JWTs by refreshing your admin token.
- If you refresh your admin token, all the tokens are invalidated.
- If your token expires or is invalidated, you get a 403 HTTP status code from Tinybird when requesting the API endpoint.

## Create a JWT in production

There is wide support for creating JWTs in many programming languages and frameworks. Any library that supports JWTs should work with Tinybird.

{% tabs initial="Python" %}

{% tab label="JavaScript (Next.js)"  %}

```JavaScript {% title="Create a JWT in JavaScript using jsonwebtoken" %}
"use server";

import jwt from "jsonwebtoken";

const TINYBIRD_SIGNING_KEY = process.env.TINYBIRD_SIGNING_KEY ?? "";

export async function generateJWT() {
  const next10minutes = new Date();
  next10minutes.setTime(next10minutes.getTime() + 1000 * 60 * 10);

  const payload = {
    workspace_id: "{% user("workspaceId") %}",
    name: "frontend_jwt",
    exp: Math.floor(next10minutes.getTime() / 1000),
    scopes: [
      {
        type: "PIPES:READ",
        resource: "requests_per_day",
        fixed_params: {
          org_id: "testing"
        }
      },
      {
        type: "DATASOURCES:READ",
        resource: "events",
        filter: "org_id = 'testing'"
      },
    ],
  };

  return jwt.sign(payload, TINYBIRD_SIGNING_KEY, {noTimestamp: true});
}
```

{% /tab %}

{% tab label="Python" %}

```python {% title="Create a JWT in Python using pyjwt" %}
import jwt
import datetime
import os

TINYBIRD_SIGNING_KEY = os.getenv('TINYBIRD_SIGNING_KEY')

def generate_jwt():
  expiration_time = datetime.datetime.utcnow() + datetime.timedelta(hours=3)
  payload = {
      "workspace_id": "{% user("workspaceId") %}",
      "name": "frontend_jwt",
      "exp": expiration_time,
      "scopes": [
          {
              "type": "PIPES:READ",
              "resource": "requests_per_day",
              "fixed_params": {
                  "org_id": "testing"
              }
          },
          {
              "type": "DATASOURCES:READ",
              "resource": "events",
              "filter": "org_id = 'testing'"
          },
      ]
  }

  return jwt.encode(payload, TINYBIRD_SIGNING_KEY, algorithm='HS256')
```

{% /tab %}

{% /tabs %}

## Create a JWT token via CLI

If for any reason you don't want to generate a JWT on your own, Tinybird provides a command and an endpoint to create a JWT token.

{% tabs initial="CLI" %}

{% tab label="API"  %}

```shell {% title="Create a JWT with the Tinybird API" %}
curl \
  -X POST "{% user("apiHost") %}/v0/tokens/?name=my_jwt&expiration_time=123123123123" \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -d '{"scopes": [{"type": "PIPES:READ", "resource": "my_pipe", "fixed_params": {"test": "testing"}}]}'
```

{% /tab %}

{% tab label="CLI" %}

```shell {% title="Create a JWT with the Tinybird CLI" %}
tb token create jwt my_jwt --ttl 1h --scope PIPES:READ --resource my_pipe --fixed-params "column_name=value" --scope DATASOURCES:READ --resource my_datasource --filter "column_name='value'"
```

{% /tab %}

{% /tabs %}

## Error handling

There are many reasons why a request might return a `403` status code. When a `403` is received, check the following:

1. Confirm the JWT is valid and hasn't expired. The expiration time is in the `exp` field in the JWT's payload.
2. The generated JWTs can only read Tinybird API endpoints or query data sources. Confirm you're not trying to use the JWT to access other APIs.
3. Confirm the JWT has a scope to read the endpoint or data source you are trying to read.
4. If you generated the JWT outside of Tinybird, without using the API or the CLI, make sure you are using the **workspace** `admin token`, not your personal one.

## Rate limits for JWTs

{% snippet title="forward-limits-reminder" /%}

When you specify a `limits.rps` field in the payload of the JWT, Tinybird uses the name specified in the payload of the JWT to track the number of requests being done. If the number of requests goes beyond the limit, Tinybird starts rejecting new requests and returns an "HTTP 429 Too Many Requests" error.

The following example shows the tracking of all requests done by `frontend_jwt`. Once you reach 10 requests per second, Tinybird would start rejecting requests:

```json {% title="Example payload with global rate limit" %}
{
    "workspace_id": "{% user("workspaceId") %}",
    "name": "frontend_jwt",
    "exp": 123123123123,
    "scopes": [
        {
            "type": "PIPES:READ",
            "resource": "requests_per_day",
            "fixed_params": {
                "org_id": "testing"
            }
        },
        {
            "type": "DATASOURCES:READ",
            "resource": "events",
            "filter": "org_id = 'testing'"
        }
    ],
    "limits": {
      "rps": 10
    }
}
```

{% callout type="info" %}

If `rps <= 0`, Tinybird ignores the limit and assumes there is no limit.

{% /callout %}

As the `name` field doesn't have to be unique, all the tokens generated using the `name=frontend_jwt` would be under the same umbrella. This can be useful if you want to have a global limit in one of your apps or components.

If you want to limit for each specific user, you can generate a JWT using the following payload. In this case, you would specify a unique name so the limits only apply to each user:

```json {% title="Example of a payload with isolated rate limit" %}
{
    "workspace_id": "{% user("workspaceId") %}",
    "name": "frontend_jwt_user_<unique identifier>",
    "exp": 123123123123,
    "scopes": [
        {
            "type": "PIPES:READ",
            "resource": "requests_per_day",
            "fixed_params": {
                "org_id": "testing"
            }
        },
        {
            "type": "DATASOURCES:READ",
            "resource": "events",
            "filter": "org_id = 'testing'"
        }
    ],
    "limits": {
      "rps": 10
    }
}
```

## Next steps

- Learn about [workspaces](../workspaces).
- Learn about [endpoints](../../work-with-data/publish-data/endpoints).
- Using [Clerk.com](https://clerk.com) in your app? [Let Clerk.com create your Tinybird JWTs automatically](https://clerk.com/blog/tinybird-and-clerk).
- Using [Auth0](https://auth0.com/) in your app? [Let Auth0 create your Tinybird JWTs automatically](https://www.tinybird.co/templates/auth0-jwt).
