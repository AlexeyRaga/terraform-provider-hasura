with import <nixpkgs> {};
let
  nixpkgs_tf = fetchFromGitHub {
    owner = "NixOS";
    repo = "nixpkgs";
    rev = "5a2fd66948917c155a565a6d58bafbf055b1e990";
    sha256 = "14v3mp48rg7zrhqlivar4pgj029ncmigc95sfz43a791q67gdm8b";
  };
  pinnedPkgs = import nixpkgs_tf {};
in mkShell {
  name = "terraform";
  buildInputs = [
    pinnedPkgs.terraform_1_0 # pin terraform version
    eksctl awscli aws-iam-authenticator jq kubernetes-helm kubernetes
    openssl # for bin/fetch_fingerprint
    tree
    go
  ];
}
