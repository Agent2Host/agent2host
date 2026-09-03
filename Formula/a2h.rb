# frozen_string_literal: true

class A2h < Formula
  desc "Register an Agent System and run a Named Agent on Claude Code, Kiro, or Codex"
  homepage "https://github.com/agent2host/agent2host"
  license "Apache-2.0"
  head "https://github.com/agent2host/agent2host.git", branch: "main"

  depends_on "go" => :build
  # Does not install Claude Code, Kiro, or Codex.

  def install
    ver = if build.head?
      "0.0.0-dev"
    else
      version.to_s
    end
    commit = Utils.git_short_head || "unknown"
    built = Time.now.utc.strftime("%Y-%m-%dT%H:%M:%SZ")
    ldflags = %W[
      -s -w
      -X github.com/agent2host/agent2host/internal/cli.Version=#{ver}
      -X github.com/agent2host/agent2host/internal/cli.Commit=#{commit}
      -X github.com/agent2host/agent2host/internal/cli.BuildTime=#{built}
    ]
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/a2h"
  end

  test do
    assert_match "a2h", shell_output("#{bin}/a2h version")
  end
end
