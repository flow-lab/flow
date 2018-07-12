class Flow < Formula
  desc "AWS tooling for faster development."
  homepage ""
  url "https://github.com/flow-lab/flow/releases/download/v0.1.0/flow_0.1.0_darwin_amd64.tar.gz", :using => GitHubPrivateRepositoryReleaseDownloadStrategy
  version "0.1.0"
  sha256 "851359e485a7007ac43b6bbd187f2e1169d588226077f3e1d7320cedf3ab2f03"
  
  depends_on "git"
  depends_on "zsh"

  def install
    bin.install "flow"
  end
end
