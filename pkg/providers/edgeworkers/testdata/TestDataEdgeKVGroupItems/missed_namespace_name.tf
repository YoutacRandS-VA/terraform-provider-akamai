provider "akamai" {
  edgerc = "../../test/edgerc"
}

data "akamai_edgekv_group_items" "test" {
  network    = "staging"
  group_name = "TestGroup"
}