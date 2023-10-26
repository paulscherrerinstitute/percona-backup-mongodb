# PSI git workflow

This workflow is needed whenever an upgrade of percona is due, which involves a new upstream release of the backup container.

In order to apply the patch on upstream releases, the most convinient workflow is what follows:

1. checkout to origin main
```bash
git checkout main
```
2. fetch upstream (if not set `git remote add upstream https://github.com/percona/percona-backup-mongodb.git`)
```bash
git fetch upstream
```
3. set the TAG_NAME of interest
```bash
TAG_NAME=...
```
4. rebase tag on origin/main
```bash
git rebase <$TAG_NAME>
```
5. resolve any possible conflict keeping the origin version for [psi-ci.yaml](.github/workflows/psi-ci.yml), [README.md](./README.md), [restore.go](./pbm/snapshot/restore.go)
6. force push to origin main
```bash
git push origin main --force-with-lease
```
7. push the tags
```bash
git push --tags
```

In this way the patch commits from origin are applied on top of the release, which results in preserving the upstream history.

# Create a release

At this point, since the image is created and pushed on release, create a release and the corresponding tat. The name of the tag should be `$TAG_NAME-patched`.
