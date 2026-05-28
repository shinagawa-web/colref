#!/usr/bin/env ruby
# frozen_string_literal: true

# Build platform-specific gems that embed the colref binary.
#
# Usage: ruby scripts/build_gems.rb <version> <artifacts_dir>
#   version       — release tag (e.g. v0.7.4 or 0.7.4)
#   artifacts_dir — directory containing colref_<version>_<os>_<arch>.tar.gz

require "rubygems"
require "rubygems/package"
require "rubygems/specification"
require "tmpdir"
require "fileutils"
require "pathname"

PLATFORMS = [
  { goos: "linux",   goarch: "amd64",  gem_platform: "x86_64-linux",   bin: "colref",     windows: false },
  { goos: "linux",   goarch: "arm64",  gem_platform: "aarch64-linux",  bin: "colref",     windows: false },
  { goos: "darwin",  goarch: "amd64",  gem_platform: "x86_64-darwin",  bin: "colref",     windows: false },
  { goos: "darwin",  goarch: "arm64",  gem_platform: "arm64-darwin",   bin: "colref",     windows: false },
  { goos: "windows", goarch: "amd64",  gem_platform: "x64-mingw-ucrt", bin: "colref.exe", windows: true  },
].freeze

EXE_WRAPPER_UNIX = <<~RUBY
  #!/usr/bin/env ruby
  # frozen_string_literal: true
  exec File.expand_path("../libexec/colref", __dir__), *ARGV
RUBY

EXE_WRAPPER_WINDOWS = <<~RUBY
  #!/usr/bin/env ruby
  # frozen_string_literal: true
  require "English"
  system File.expand_path("../libexec/colref.exe", __dir__), *ARGV
  exit($CHILD_STATUS.exitstatus || 1)
RUBY

ROOT = Pathname.new(__dir__).parent

def normalize_version(raw_version)
  raw_version
    .sub(/-rc(\d+)$/,    '.rc.\1')
    .sub(/-beta(\d+)$/,  '.beta.\1')
    .sub(/-alpha(\d+)$/, '.alpha.\1')
    .sub(/-pre(\d+)$/,   '.pre.\1')
end

def build_gem(raw_version, platform, artifacts_dir, out_dir)
  tarball = artifacts_dir / "colref_#{raw_version}_#{platform[:goos]}_#{platform[:goarch]}.tar.gz"
  unless tarball.exist?
    warn "  ERROR: #{tarball.basename} not found"
    return false
  end

  Dir.mktmpdir do |tmpdir|
    tmpdir = Pathname.new(tmpdir)

    system("tar", "xzf", tarball.to_s, "-C", tmpdir.to_s, platform[:bin], exception: true)
    (tmpdir / platform[:bin]).chmod(0o755)

    gem_root = tmpdir / "gem"
    (gem_root / "exe").mkpath
    (gem_root / "libexec").mkpath
    (gem_root / "lib").mkpath

    wrapper = platform[:windows] ? EXE_WRAPPER_WINDOWS : EXE_WRAPPER_UNIX
    (gem_root / "exe" / "colref").write(wrapper)
    (gem_root / "exe" / "colref").chmod(0o755)

    FileUtils.cp(tmpdir / platform[:bin], gem_root / "libexec" / platform[:bin])
    (gem_root / "libexec" / platform[:bin]).chmod(0o755)

    (gem_root / "lib" / "colref.rb").write("# colref #{raw_version}\n")
    FileUtils.cp(ROOT / "LICENSE", gem_root / "LICENSE")

    spec = Gem::Specification.new do |s|
      s.name                  = "colref"
      s.version               = normalize_version(raw_version)
      s.platform              = platform[:gem_platform]
      s.summary               = "Check whether a database column is still referenced in your codebase before you delete it"
      s.description           = s.summary
      s.homepage              = "https://github.com/shinagawa-web/colref"
      s.license               = "MIT"
      s.authors               = ["Kazutomo Deguchi"]
      s.bindir                = "exe"
      s.executables           = ["colref"]
      s.files                 = ["exe/colref", "libexec/#{platform[:bin]}", "lib/colref.rb", "LICENSE"]
      s.require_paths         = ["lib"]
      s.required_ruby_version = ">= 3.1"
      s.metadata = {
        "source_code_uri" => "https://github.com/shinagawa-web/colref",
        "bug_tracker_uri" => "https://github.com/shinagawa-web/colref/issues",
      }
    end

    gem_file = nil
    Dir.chdir(gem_root) do
      gem_file = Gem::Package.build(spec)
    end

    dest = out_dir / gem_file
    FileUtils.mv(gem_root / gem_file, dest)
    puts "  built #{dest.basename}"
    true
  end
end

raw_version = ARGV[0]&.sub(/^v/, "") or abort "Usage: #{$PROGRAM_NAME} <version> <artifacts_dir>"
artifacts_dir = Pathname.new(ARGV[1] || abort("Usage: #{$PROGRAM_NAME} <version> <artifacts_dir>"))
out_dir = Pathname.new("pkg").tap(&:mkpath)

failed = PLATFORMS.count { |p| build_gem(raw_version, p, artifacts_dir, out_dir) == false }
abort "#{failed} platform(s) failed — aborting" if failed > 0
