class CsghubLite < Formula
  desc "Lightweight tool for running LLMs locally with CSGHub platform"
  homepage "https://github.com/opencsgs/csghub-lite"
  version "0.8.98"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin-arm64.tar.gz"
      sha256 "b869fdeb3ebc47ad6cb2a49fa3a9ee80d519873d4e7e00a9ae53e00ed08d7966"
    end

    on_intel do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_darwin-amd64.tar.gz"
      sha256 "5436f76d12c21ef0bf73c5c7374b83779b2ba6b453a4c513032610a330e75ef4"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux-arm64.tar.gz"
      sha256 "b13efb2739fe30c59963c13467411b202a8bcdb79294c9fb88bacbef1e9f02f3"
    end

    on_intel do
      url "https://github.com/OpenCSGs/csghub-lite/releases/download/v#{version}/csghub-lite_#{version}_linux-amd64.tar.gz"
      sha256 "ee02bd6e3f5d485d56e43e7516b9c4c2b9859222c2a46820c2f236d1c33a55c3"
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
