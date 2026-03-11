class CsghubLite < Formula
  desc "Lightweight tool for running LLMs locally with CSGHub platform"
  homepage "https://github.com/opencsgs/csghub-lite"
  version "0.1.0"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/opencsgs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin_arm64.tar.gz"
      # sha256 will be filled in by goreleaser or manually after release
    end
    on_intel do
      url "https://github.com/opencsgs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin_amd64.tar.gz"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/opencsgs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux_arm64.tar.gz"
    end
    on_intel do
      url "https://github.com/opencsgs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux_amd64.tar.gz"
    end
  end

  depends_on "llama.cpp" => :recommended

  def install
    bin.install "csghub-lite"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/csghub-lite --version")
  end
end
