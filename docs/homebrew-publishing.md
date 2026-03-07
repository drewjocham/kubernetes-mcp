# Publishing kube-watcher to Homebrew

This guide covers everything needed to make `brew install drewjocham/tap/kube-watcher` work.

## Prerequisites

1. **GoReleaser** installed locally (for testing): `brew install goreleaser`
2. A **separate GitHub repo** for your Homebrew tap: `drewjocham/homebrew-tap`
3. A **GitHub Personal Access Token** with `repo` scope (for GoReleaser to push the formula)

## One-Time Setup

### 1. Create the tap repository

```bash
# Create the repo on GitHub
gh repo create drewjocham/homebrew-tap --public --description "Homebrew tap for kube-watcher"

# Clone and initialize
git clone git@github.com:drewjocham/homebrew-tap.git
cd homebrew-tap
mkdir Formula
touch README.md
git add . && git commit -m "init tap" && git push
```

### 2. Add the GitHub secret

In the **kubernetes-mcp** repo, add a secret:

- Go to **Settings → Secrets and variables → Actions**
- Create `HOMEBREW_TAP_TOKEN` with a PAT that has `repo` scope on `drewjocham/homebrew-tap`

### 3. Verify GoReleaser config

The `.goreleaser.yaml` in this repo is already configured. The key section:

```yaml
brews:
  - name: kube-watcher
    repository:
      owner: drewjocham
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
```

This tells GoReleaser to auto-commit the formula to `drewjocham/homebrew-tap/Formula/kube-watcher.rb` on each release.

## Releasing

### Automated (recommended)

Tag and push:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The GitHub Actions workflow (`.github/workflows/release.yaml`) will:

1. Build binaries for linux/darwin × amd64/arm64 + windows/amd64
2. Create a GitHub Release with archives and checksums
3. Auto-update the Homebrew formula in `drewjocham/homebrew-tap`

### Manual (testing)

```bash
# Dry run (no publish)
goreleaser release --snapshot --clean

# Check the generated formula
cat dist/homebrew/Formula/kube-watcher.rb
```

## Installing via Homebrew

Once published:

```bash
# Add the tap (one-time)
brew tap drewjocham/tap

# Install
brew install kube-watcher

# Verify
kube-watcher --version
```

Or as a one-liner:

```bash
brew install drewjocham/tap/kube-watcher
```

## Updating

When you push a new tag, GoReleaser updates the formula automatically. Users update with:

```bash
brew upgrade kube-watcher
```

## Files Summary

| File | Purpose |
|------|---------|
| `.goreleaser.yaml` | GoReleaser config: builds, archives, Homebrew formula generation |
| `.github/workflows/release.yaml` | CI: runs GoReleaser on `v*` tag push |
| `Formula/kube-watcher.rb` | Template formula (auto-updated by GoReleaser) |
| `LICENSE` | MIT license (required by Homebrew) |

## Troubleshooting

**"tap not found"**: make sure the repo is named exactly `homebrew-tap` (Homebrew convention: `homebrew-{name}` → `brew tap owner/name`).

**"sha256 mismatch"**: GoReleaser computes SHA256 from the actual archives. If you manually edit the formula, recompute with `shasum -a 256 kube-watcher_*.tar.gz`.

**"token permission denied"**: the PAT needs `repo` scope on the tap repo, not just the source repo.
