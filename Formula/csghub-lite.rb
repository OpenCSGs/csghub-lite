class CsghubLite < Formula
  desc "Lightweight tool for running LLMs locally with CSGHub platform"
  homepage "https://github.com/opencsgs/csghub-lite"
  version "0.8.22"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin-arm64.tar.gz"
      sha256 "0784f83f52b5c16cb922c085ef4801de7d2bb2078135f59c52518168112c3c9a"
    end

    on_intel do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin-amd64.tar.gz"
      sha256 "64e4007eef4047cf8f4771c6e62eda7867ec8922891222d85973fa4289c07ba0"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux-arm64.tar.gz"
      sha256 "688707ff30ad1ea4603fb9faf3589ed3a05bd535008d7552b4ec8b24985476fb"
    end

    on_intel do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux-amd64.tar.gz"
      sha256 "c0eb560d6b3cf1d3f3f6dc60b0bec62adf55c0d2918f08f03a69cc8ac4233664"
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
