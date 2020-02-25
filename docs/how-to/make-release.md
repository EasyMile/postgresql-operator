# How to make a release ?

## Requirements

- Be an administrator
- The master branch must compile (`make code/compile` and `make code/docker version=latest`)
- The master branch must have linter ok (`make code/lint`)

## Prepare the release

- Make a branch from master
- Look at the git log and find a good version following semver logic and git commit message types (feat, fix, BREAKING CHANGE, ...).
- Run `./set_version.sh NEW_SEMVER_VERSION`
- Run CRDs generation `make code/gen`
- Run `make release/olm-catalog version=NEW_SEMVER_VERSION`
- Update `CHANGELOG.md`
- Push all of this with commit message `chore: Release NEW_SEMVER_VERSION`

## Release

- Create a pull request for all these changes
- Once it is merged, put a tag on the commit previously created. In Github release, put the changelog section.

