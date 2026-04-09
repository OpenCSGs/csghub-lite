class CsghubLite < Formula
  desc "Lightweight tool for running LLMs locally with CSGHub platform"
  homepage "https://github.com/opencsgs/csghub-lite"
  version "0.7.9"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin-arm64.tar.gz"
      sha256 "36338b1c9321bbafc8620f2d7a9d66ee0ae1de940806888226de55b2bf3a2f81"
    end

    on_intel do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin-amd64.tar.gz"
      sha256 "16aeb66a4bef0e1ed2fa25272e12a26a19feacc298e2086a1c837f56d0edac3a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux-arm64.tar.gz"
      sha256 "5da0440c46eaa3f510b3545ea37692e223d7c16b21219ab5c380dd7f5451711c"
    end

    on_intel do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux-amd64.tar.gz"
      sha256 "99754e994b590d87d0b3e5f757812bd58609f266f7cafc76d685656147bfbee9"
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
