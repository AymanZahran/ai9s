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
  url "https://github.com/AymanZahran/ai9s/archive/refs/tags/v1.0.1.tar.gz", using: Ai9sDownloadStrategy
  sha256 "34187da82b94105a53af9da1c5baa6f99e88693b35aa9d1c6052eef2af5d4fd8"
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
