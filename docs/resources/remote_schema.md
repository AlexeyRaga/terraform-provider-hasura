---
page_title: "remote-schema Resource - terraform-provider-hasura"
subcategory: ""
description: |-
  The remote schema resource allows you to specify a remote schema to be added to the Hasura GraphQL Engine.
---

# Resource `hasura_remote_schema`

 The remote schema resource allows you to specify a remote schema to be added to the Hasura GraphQL Engine.

## Example Usage

```terraform
resource "hasura_remote_schema" "spacex" {
  name = "SpaceX"
  url = "https://api.spacex.land/graphql/"
  forward_headers = true
  additional_headers = {
    "X-FOO" = "FOO"
    "X-BAR" = "BAR"
  }
}
```

## Argument Reference

- `name` - (Required) The name of the remote schema.
- `url` - (Required) The URL of the remote schema.
- `forward_headers` - (Optional) Whether to forward headers from the remote schema to the GraphQL server. Defaults to `false`.
- `additional_headers` - (Optional) Additional headers to be sent to the GraphQL server.`
