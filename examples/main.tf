terraform {
  required_providers {
    hasura = {
      version = "0.0.1"
      source  = "AlexeyRaga/hasura"
    }
  }
}

provider "hasura" {
  host = "hasura.test.educationperfect.io"
  admin_secret = "Password1"
}
resource "hasura_remote_schema" "spacex" {
  name = "SpaceX"
  url = "https://api.spacex.land/graphql/"
}
