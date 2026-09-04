# Install Agent2Host

Direct download is the supported installation path for the current public alpha. A Homebrew command will become the recommended path after the tap passes a clean installation test.

## Before you install

You need:

- **macOS or Linux** on `arm64` or `amd64`;
- **a terminal** and permission to place `a2h` in a directory on your `PATH`;
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

!!! info "Homebrew is next, not available yet"
    The repository already publishes release archives and checksums. The documentation will switch to `brew install agent2host/tap/a2h` only after the separate tap exists and that exact command has passed a clean-machine test. Until then, use a release archive below.

## Install a release archive

Open the [GitHub Releases page](https://github.com/agent2host/agent2host/releases) and choose the archive for your machine.

| Machine | Archive suffix |
| --- | --- |
| Apple Silicon Mac | `darwin-arm64.tar.gz` |
| Intel Mac | `darwin-amd64.tar.gz` |
| Linux x86_64 | `linux-amd64.tar.gz` |
| Linux arm64 | `linux-arm64.tar.gz` |

### macOS

The commands below install `v0.1.0-alpha.1` on an Apple Silicon Mac. For an Intel Mac, replace `darwin-arm64` with `darwin-amd64`.

```bash
mkdir -p "$HOME/bin"
curl -fL -o /tmp/a2h.tgz \
  https://github.com/agent2host/agent2host/releases/download/v0.1.0-alpha.1/a2h-v0.1.0-alpha.1-darwin-arm64.tar.gz
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
curl -fL -o /tmp/SHA256SUMS \
  https://github.com/agent2host/agent2host/releases/download/v0.1.0-alpha.1/SHA256SUMS
shasum -a 256 /tmp/a2h.tgz
grep 'a2h-v0.1.0-alpha.1-darwin-arm64.tar.gz' /tmp/SHA256SUMS
```

The value printed by `shasum` must match the first value on the `SHA256SUMS` line. On Linux, use `sha256sum /tmp/a2h.tgz`.

## Build from source

Use this path when you want to inspect or build the source rather than install a release archive:

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

The release archive contains the executable and license, not the example Agent Systems. Clone the repository when following [Run your first Agent](first-run.md), or register a System folder you already own.
