class Flow < Formula
  desc "AWS tooling for faster development."
  homepage ""
  url "https://github.com/flow-lab/flow/releases/download/v0.1.0/flow_0.1.0_darwin_amd64.tar.gz", :using => GitHubPrivateRepositoryReleaseDownloadStrategy
  version "0.1.0"
  sha256 "170615de2c58dc279efcb1381ae534639e2d8b44f463a3b69b9217d632b9c929"
  
  depends_on "git"
  depends_on "zsh"

  def install
    bin.install "flow"
  end
end
