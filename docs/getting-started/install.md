# Install Agent2Host

Homebrew is the recommended install. A release archive is the fallback. Both install only the Agent2Host binary. They do not install Claude Code, Kiro, or Codex.

This is a public alpha, not a stable 1.0. `a2h version` prints the exact build you have.

## Before you install

You need:

- **macOS or Linux** on `arm64` or `amd64`;
- **a terminal** and permission to place `a2h` on your `PATH`;
- **one native host installed and signed in**: Claude Code (`claude`), Kiro (`kiro-cli`), or Codex (`codex`);
- **Git** only if you want to clone the official examples or build from source;
- **Go 1.22 or newer** only if you choose a source build.

Check the host you plan to use before installing Agent2Host:

```bash
claude --version
# or
kiro-cli --version
# or
codex --version
```

Agent2Host launches the host you already use. It does not install that host, choose its model, or complete its sign-in flow.

## Install with Homebrew

```bash
brew install agent2host/tap/a2h
which a2h
a2h version
```

Later upgrades:

```bash
brew update
brew upgrade a2h
```

`which a2h` should point at the Homebrew Cellar binary.

## Install a release archive

Use this path when you do not have Homebrew. Open the [latest GitHub Release](https://github.com/agent2host/agent2host/releases/latest) and copy its tag. Download the archive for your machine.

| Machine | Archive suffix |
| --- | --- |
| Apple Silicon Mac | `darwin-arm64.tar.gz` |
| Intel Mac | `darwin-amd64.tar.gz` |
| Linux x86_64 | `linux-amd64.tar.gz` |
| Linux arm64 | `linux-arm64.tar.gz` |

Replace `TAG` with the tag you copied.

### macOS

Apple Silicon. For an Intel Mac, replace `darwin-arm64` with `darwin-amd64`.

```bash
# Paste the latest tag from the Releases page, then:
TAG=
mkdir -p "$HOME/bin"
curl -fL -o /tmp/a2h.tgz \
  "https://github.com/agent2host/agent2host/releases/download/${TAG}/a2h-${TAG}-darwin-arm64.tar.gz"
tar -xzf /tmp/a2h.tgz -C "$HOME/bin"
```

Add `$HOME/bin` to your `PATH` in zsh:

```bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

Confirm that the shell found the release binary:

```bash
which a2h
a2h version
```

The release binary reports its version, source commit, and build time.

### Linux

Use the same steps with the `linux-amd64` or `linux-arm64` archive. Add `$HOME/bin` to your Bash profile:

```bash
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

## Verify the download

Every release publishes `SHA256SUMS`. Download it from the same release, calculate the archive hash, and compare the two values.

```bash
# Use the same TAG as the archive you downloaded.
curl -fL -o /tmp/SHA256SUMS \
  "https://github.com/agent2host/agent2host/releases/download/${TAG}/SHA256SUMS"
shasum -a 256 /tmp/a2h.tgz
grep "a2h-${TAG}-darwin-arm64.tar.gz" /tmp/SHA256SUMS
```

The value printed by `shasum` must match the first value on the `SHA256SUMS` line. On Linux, use `sha256sum /tmp/a2h.tgz`.

## Build from source

Use this path when you want to inspect or build the source rather than install a release:

```bash
git clone https://github.com/agent2host/agent2host.git
cd agent2host
go build -o a2h ./cmd/a2h
./a2h version
```

A local source build reports `0.0.0-dev`. The binary stays in the clone directory. To use it from anywhere:

```bash
mkdir -p "$HOME/bin"
cp a2h "$HOME/bin/"
```

## Next step

The release archive and the Homebrew formula contain the executable and license, not the example Agent Systems. Clone the repository when following [Run your first Agent](first-run.md), or register a System folder you already own.
