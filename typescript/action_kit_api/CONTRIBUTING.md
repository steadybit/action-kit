# Contributing Guidelines

## Executing the Generator

```sh
npm run build
```

## Releasing

 1. Update `CHANGELOG.md`
 2. Push the changelog changes `git add CHANGELOG.md && git commit -m 'chore: prepare release'`
 3. Ensure that you have a `.git` directory within `typescript/action_kit_api`.
    Without this, npm [will not commit/tag the release](https://github.com/npm/cli/pull/4885):
    `mkdir .git`
 4. Raise the version number in the `package.json`: `npm version (major|minor|patch)`
 5. Push the tag: `git push origin main --tags`
