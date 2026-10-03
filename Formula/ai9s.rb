# typed: false
# frozen_string_literal: true

require "download_strategy"

# Adds a GitHub token header when HOMEBREW_GITHUB_API_TOKEN is set, so a
# private source archive can be downloaded. Public installs work without it.
class Ai9sDownloadStrategy < CurlDownloadStrategy
  def initialize(url, name, version, token: ENV.fetch("HOMEBREW_GITHUB_API_TOKEN", ""), **meta)
    unless token.empty?
      meta[:headers] ||= []
      meta[:headers] << "Authorization: Bearer #{token}"
    end
    super(url, name, version, **meta)
  end
end

class Ai9s < Formula
  desc "Keyboard-first finder for local AI coding sessions"
  homepage "https://ai9scli.io"
  url "https://github.com/AymanZahran/ai9s/archive/refs/tags/v0.1.0.tar.gz", using: Ai9sDownloadStrategy
  sha256 "038f700398e6f93b8d546b53c3f6b62e8e0d7d912fdbd16da86145cdf747ba11"
  license "MIT"
  head "https://github.com/AymanZahran/ai9s.git", branch: "main"

  livecheck do
    url :stable
    regex(/^v?(\d+(?:\.\d+)+)$/i)
  end

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X github.com/AymanZahran/ai9s/cmd.version=#{version}"), "."
  end

  test do
    assert_match(/^ai9s \d+\.\d+\.\d+/, shell_output("#{bin}/ai9s version"))
  end
end
