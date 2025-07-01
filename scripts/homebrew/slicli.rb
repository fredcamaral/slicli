class Slicli < Formula
  desc "CLI-based slide presentation generator with plugin architecture"
  homepage "https://github.com/fredcamaral/slicli"
  url "https://github.com/fredcamaral/slicli/archive/v1.0.0.tar.gz"
  sha256 "SHA256_HASH_PLACEHOLDER"
  license "MIT"
  head "https://github.com/fredcamaral/slicli.git", branch: "main"

  depends_on "go" => :build
  depends_on "make" => :build

  def install
    # Build the main binary
    system "make", "build"
    bin.install "bin/slicli"

    # Build and install plugins
    system "make", "build-plugins"
    
    # Create plugin directory
    plugin_dir = prefix/"lib/slicli/plugins"
    plugin_dir.mkpath
    
    # Install plugins
    Dir["plugins/*/*.so"].each do |plugin|
      plugin_dir.install plugin
    end
    
    # Install built plugin binaries from subdirectories
    Dir["plugins/*/build/*.so"].each do |plugin|
      plugin_dir.install plugin
    end

    # Install themes
    theme_dir = prefix/"share/slicli/themes"
    theme_dir.mkpath
    (theme_dir).install Dir["themes/*"]

    # Install default configuration
    config_dir = prefix/"etc/slicli"
    config_dir.mkpath
    config_dir.install "configs/default.toml"

    # Install examples
    doc.install Dir["examples/*"]
    
    # Install documentation
    doc.install "README.md"
    doc.install "LICENSE"
  end

  def post_install
    # Create user config directory
    config_dir = "#{Dir.home}/.config/slicli"
    FileUtils.mkdir_p(config_dir) unless Dir.exist?(config_dir)
    
    # Copy default config if it doesn't exist
    user_config = "#{config_dir}/config.toml"
    default_config = "#{etc}/slicli/default.toml"
    
    unless File.exist?(user_config)
      FileUtils.cp(default_config, user_config) if File.exist?(default_config)
    end
    
    # Create symlinks to themes and plugins
    themes_link = "#{config_dir}/themes"
    plugins_link = "#{config_dir}/plugins"
    
    unless File.exist?(themes_link)
      FileUtils.ln_sf("#{share}/slicli/themes", themes_link)
    end
    
    unless File.exist?(plugins_link)
      FileUtils.ln_sf("#{lib}/slicli/plugins", plugins_link)
    end
    
    puts ""
    puts "🚀 slicli installed successfully!"
    puts ""
    puts "📋 Next steps:"
    puts "  1. Run: slicli --version"
    puts "  2. Get help: slicli --help"
    puts "  3. Try example: slicli serve #{doc}/simple-ppt/presentation.md"
    puts ""
    puts "📁 Configuration: ~/.config/slicli/"
    puts "📖 Documentation: #{doc}/"
    
    # Check for Chrome/Chromium
    chrome_found = system("which google-chrome > /dev/null 2>&1") ||
                  system("which chromium > /dev/null 2>&1") ||
                  system("which chromium-browser > /dev/null 2>&1")
    
    unless chrome_found
      puts ""
      puts "⚠️  Chrome/Chromium not found:"
      puts "   Install for full PDF/image export: brew install --cask google-chrome"
    end
  end

  test do
    # Test binary runs and shows version
    assert_match version.to_s, shell_output("#{bin}/slicli --version")
    
    # Test that it can show help
    assert_match "CLI-based slide presentation generator", shell_output("#{bin}/slicli --help")
    
    # Test configuration directory creation
    config_test_dir = testpath/".config/slicli"
    system bin/"slicli", "--config-dir", config_test_dir.to_s, "--version"
    assert_predicate config_test_dir, :exist?
  end
end