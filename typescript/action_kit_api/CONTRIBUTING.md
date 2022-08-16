# Contributing Guidelines

## Executing the Generator

```sh
npm run build
```

## Releasing

 1. Update `CHANGELOG.md`
 2. Push the changelog changes `git add CHANGELOG.md && git commit -m 'chore: prepare release'`
 3. Raise the version number in the `package.json`: `npm version (major|minor|patch)`
 4. Push the tag: `git push origin main --tags`
