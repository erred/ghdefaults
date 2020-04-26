# ghdefaults

A repo for ghdefaults

[![License](https://img.shields.io/github/license/seankhliao/ghdefaults.svg?style=flat-square)](LICENSE)
![Version](https://img.shields.io/github/v/tag/seankhliao/ghdefaults?sort=semver&style=flat-square)
[![pkg.go.dev](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://pkg.go.dev/go.seankhliao.com/ghdefaults)

Github app for setting defaults on my own repos

- General: disable Issues, Wikis, Projects
- Pull Requests: disable Merge, Rebase (only use squash)

## deployment

deploy on cloud run

manage [app settins](https://github.com/settings/apps/gh-defaults)
todo:

- update urls
- update secrets (see below)

- env:
  - `WEBHOOK_SECRET`: shared secret with github
  - `PRIVATE_KEY`: base64 encoded private key (because environment variables and newlines :shrug:)
