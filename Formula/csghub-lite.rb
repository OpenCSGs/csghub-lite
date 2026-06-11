class CsghubLite < Formula
  desc "Lightweight tool for running LLMs locally with CSGHub platform"
  homepage "https://github.com/opencsgs/csghub-lite"
  version "0.8.90"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin-arm64.tar.gz"
      sha256 "878051393aa9a6a02ab31dca10acf8f63b0e47764fa63d34a5aee4c7d3b61367"
    end

    on_intel do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin-amd64.tar.gz"
      sha256 "cb3c3476ca075a46767fc624463f3b67c75b9942a836b5eb15397f515c3a921e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux-arm64.tar.gz"
      sha256 "45e130718544d93f4a13c0aac0974a0bf96279f27f72435f67b94255b52f4a97"
    end

    on_intel do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux-amd64.tar.gz"
      sha256 "e0f248809a400166e5b7178f13c5d36f6926f0ab5d880a7c523bbf34a42de050"
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
