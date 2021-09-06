---
page_title: "Provider: Hasura"
subcategory: ""
description: |-
  Terraform provider for interacting with Hasura.
---

# Hasura Provider

The Hasura provider is used to interact with Hasura GraphQL server.

## Example Usage

Do not keep your authentication password in HCL for production environments, use Terraform environment variables.

```terraform
provider "hasura" {
  query_uri = "http://127.0.0.1:8080/v1/query"
  admin_secret = "Password1"
}
```

## Schema

- **query_uri** (String, Required) The URI of the Hasura GraphQL server.
- **admin_secret** (String, Required) The admin secret of the Hasura GraphQL server.

