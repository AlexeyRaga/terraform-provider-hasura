terraform {
  required_providers {
    hasura = {
      version = "0.0.5"
      source  = "AlexeyRaga/hasura"
    }
  }
}

provider "hasura" {
  query_uri = "http://127.0.0.1:8080/v1/query"
  admin_secret = "Password1"
}

resource "hasura_remote_schema" "spacex" {
  name = "SpaceX"
  url = "https://api.spacex.land/graphql/"
  forward_headers = true
  additional_headers = {
    "X-FOO" = "FOO"
    "X-BAR" = "BAR"
  }
}

resource "hasura_remote_schema" "rm" {
  name = "TrevorBlade"
  url = "https://countries.trevorblades.com/"
  forward_headers = false
  additional_headers = {
    "X-FOO" = "NEW-FOO"
  }
}
