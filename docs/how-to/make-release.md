# How to make a release ?

## Requirements

- Be an administrator
- The master branch must compile (`make build` and `make docker-build`)
- The master branch must have linter ok (`make code/lint`)
- The master branch must have tests ok (`make setup/services test`)

## Prepare the release

- Make a branch from master
- Look at the git log and find a good version following semver logic and git commit message types (feat, fix, BREAKING CHANGE, ...).
- Run CRDs generation `make manifests generate`
- Update Helm chart values for new version
- Update `CHANGELOG.md`
- Push all of this with commit message `chore: Release NEW_SEMVER_VERSION`

## Release

- Create a pull request for all these changes
- Once it is merged, put a tag on the commit previously created. In Github release, put the changelog section.
