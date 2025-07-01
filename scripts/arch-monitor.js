#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

/**
 * Architecture Health Monitor for slicli
 * 
 * Monitors the health and metrics of the slicli architecture including:
 * - Component analysis and metrics
 * - Plugin system health
 * - Theme system status
 * - Security posture
 * - Performance characteristics
 */
class ArchitectureMonitor {
  constructor() {
    this.projectRoot = path.resolve(__dirname, '..');
    this.components = [
      'cmd/slicli',
      'internal/domain/services',
      'internal/adapters/primary/http',
      'internal/adapters/secondary/plugin',
      'internal/adapters/secondary/theme',
      'plugins'
    ];
    this.metrics = {
      componentHealth: new Map(),
      pluginCount: 0,
      themeCount: 0,
      apiEndpoints: 0,
      testCoverage: 0,
      securityScore: 0
    };
  }

  async analyzeArchitecture() {
    console.log('🔍 Starting slicli architecture analysis...\n');

    const analysis = {
      timestamp: new Date().toISOString(),
      project: 'slicli',
      version: await this.getProjectVersion(),
      architecture: await this.analyzeComponents(),
      plugins: await this.analyzePlugins(),
      themes: await this.analyzeThemes(),
      security: await this.analyzeSecurityPosture(),
      performance: await this.analyzePerformance(),
      codeMetrics: await this.analyzeCodeMetrics(),
      healthScore: 0
    };

    analysis.healthScore = this.calculateHealthScore(analysis);

    console.log('📊 Architecture Analysis Results:\n');
    console.log(JSON.stringify(analysis, null, 2));

    // Save detailed report
    const reportPath = path.join(this.projectRoot, 'docs/code-review/architecture-health.json');
    fs.writeFileSync(reportPath, JSON.stringify(analysis, null, 2));
    console.log(`\n📄 Detailed report saved to: ${reportPath}`);

    return analysis;
  }

  async getProjectVersion() {
    try {
      const goMod = fs.readFileSync(path.join(this.projectRoot, 'go.mod'), 'utf8');
      const match = goMod.match(/module\s+(.+)/);
      return match ? match[1] : 'unknown';
    } catch (error) {
      return 'unknown';
    }
  }

  async analyzeComponents() {
    const components = {};

    for (const component of this.components) {
      const componentPath = path.join(this.projectRoot, component);
      
      try {
        const stats = fs.statSync(componentPath);
        if (stats.isDirectory()) {
          components[component] = {
            path: componentPath,
            files: this.countGoFiles(componentPath),
            linesOfCode: this.countLinesOfCode(componentPath),
            hasTests: this.hasTests(componentPath),
            testCoverage: this.getTestCoverage(componentPath),
            lastModified: stats.mtime
          };
        }
      } catch (error) {
        console.warn(`⚠️  Could not analyze component ${component}: ${error.message}`);
        components[component] = { error: error.message };
      }
    }

    return components;
  }

  async analyzePlugins() {
    const pluginsDir = path.join(this.projectRoot, 'plugins');
    const plugins = {};

    try {
      const entries = fs.readdirSync(pluginsDir);
      
      for (const entry of entries) {
        const pluginPath = path.join(pluginsDir, entry);
        const stats = fs.statSync(pluginPath);
        
        if (stats.isDirectory() && entry !== 'README.md') {
          plugins[entry] = await this.analyzePlugin(pluginPath);
        }
      }

      this.metrics.pluginCount = Object.keys(plugins).length;
    } catch (error) {
      console.warn(`⚠️  Could not analyze plugins: ${error.message}`);
    }

    return plugins;
  }

  async analyzePlugin(pluginPath) {
    const pluginName = path.basename(pluginPath);
    
    return {
      name: pluginName,
      hasMainGo: fs.existsSync(path.join(pluginPath, 'main.go')),
      hasMakefile: fs.existsSync(path.join(pluginPath, 'Makefile')),
      hasTests: this.hasTests(pluginPath),
      hasBinary: fs.existsSync(path.join(pluginPath, `${pluginName}.so`)),
      linesOfCode: this.countLinesOfCode(pluginPath),
      lastModified: fs.statSync(pluginPath).mtime
    };
  }

  async analyzeThemes() {
    const themesDir = path.join(this.projectRoot, 'themes');
    const themes = {};

    try {
      const entries = fs.readdirSync(themesDir);
      
      for (const entry of entries) {
        const themePath = path.join(themesDir, entry);
        const stats = fs.statSync(themePath);
        
        if (stats.isDirectory() && entry !== 'README.md') {
          themes[entry] = await this.analyzeTheme(themePath);
        }
      }

      this.metrics.themeCount = Object.keys(themes).length;
    } catch (error) {
      console.warn(`⚠️  Could not analyze themes: ${error.message}`);
    }

    return themes;
  }

  async analyzeTheme(themePath) {
    const themeName = path.basename(themePath);
    
    return {
      name: themeName,
      hasConfig: fs.existsSync(path.join(themePath, 'theme.toml')),
      hasTemplates: fs.existsSync(path.join(themePath, 'templates')),
      hasAssets: fs.existsSync(path.join(themePath, 'assets')),
      templateCount: this.countTemplates(themePath),
      assetCount: this.countAssets(themePath),
      isBuiltIn: ['default', 'minimal', 'dark'].includes(themeName),
      lastModified: fs.statSync(themePath).mtime
    };
  }

  async analyzeSecurityPosture() {
    const security = {
      hasSecurityMiddleware: this.checkSecurityMiddleware(),
      hasCORSConfig: this.checkCORSConfig(),
      hasRateLimiting: this.checkRateLimiting(),
      hasInputSanitization: this.checkInputSanitization(),
      hasPathValidation: this.checkPathValidation(),
      securityTestCoverage: this.getSecurityTestCoverage(),
      vulnerabilities: await this.checkVulnerabilities()
    };

    // Calculate security score
    const securityChecks = Object.values(security).filter(v => typeof v === 'boolean');
    const passed = securityChecks.filter(Boolean).length;
    this.metrics.securityScore = Math.round((passed / securityChecks.length) * 100);

    return security;
  }

  async analyzePerformance() {
    return {
      hasPerformanceTests: this.hasPerformanceTests(),
      hasProfilerSupport: this.checkProfilerSupport(),
      hasCaching: this.checkCaching(),
      hasAssetOptimization: this.checkAssetOptimization(),
      estimatedStartupTime: this.estimateStartupTime(),
      pluginLoadingStrategy: this.analyzePluginLoadingStrategy()
    };
  }

  async analyzeCodeMetrics() {
    try {
      const totalFiles = this.countGoFiles(this.projectRoot);
      const totalLOC = this.countLinesOfCode(this.projectRoot);
      const testFiles = this.countTestFiles(this.projectRoot);
      
      return {
        totalGoFiles: totalFiles,
        totalLinesOfCode: totalLOC,
        testFiles: testFiles,
        avgLinesPerFile: totalFiles > 0 ? Math.round(totalLOC / totalFiles) : 0,
        testToCodeRatio: totalFiles > 0 ? Math.round((testFiles / totalFiles) * 100) : 0,
        hasGolangciLint: this.checkGolangciLint(),
        hasSecurityChecks: this.checkSecurityChecks()
      };
    } catch (error) {
      return { error: 'Could not analyze code metrics' };
    }
  }

  // Helper methods for analysis

  countGoFiles(dir) {
    try {
      const result = execSync(`find "${dir}" -name "*.go" -not -path "*/vendor/*" | wc -l`, { encoding: 'utf8' });
      return parseInt(result.trim()) || 0;
    } catch (error) {
      return 0;
    }
  }

  countLinesOfCode(dir) {
    try {
      const result = execSync(`find "${dir}" -name "*.go" -not -path "*/vendor/*" -exec wc -l {} + | tail -1`, { encoding: 'utf8' });
      const match = result.trim().match(/(\d+)\s+total/);
      return match ? parseInt(match[1]) : 0;
    } catch (error) {
      return 0;
    }
  }

  countTestFiles(dir) {
    try {
      const result = execSync(`find "${dir}" -name "*_test.go" -not -path "*/vendor/*" | wc -l`, { encoding: 'utf8' });
      return parseInt(result.trim()) || 0;
    } catch (error) {
      return 0;
    }
  }

  hasTests(dir) {
    try {
      const result = execSync(`find "${dir}" -name "*_test.go" | wc -l`, { encoding: 'utf8' });
      return parseInt(result.trim()) > 0;
    } catch (error) {
      return false;
    }
  }

  getTestCoverage(dir) {
    try {
      // Simplified coverage check - could be enhanced with actual go test -cover
      const testFiles = this.countTestFiles(dir);
      const goFiles = this.countGoFiles(dir);
      return goFiles > 0 ? Math.round((testFiles / goFiles) * 100) : 0;
    } catch (error) {
      return 0;
    }
  }

  countTemplates(themePath) {
    try {
      const templatesDir = path.join(themePath, 'templates');
      if (!fs.existsSync(templatesDir)) return 0;
      const result = execSync(`find "${templatesDir}" -name "*.html" | wc -l`, { encoding: 'utf8' });
      return parseInt(result.trim()) || 0;
    } catch (error) {
      return 0;
    }
  }

  countAssets(themePath) {
    try {
      const assetsDir = path.join(themePath, 'assets');
      if (!fs.existsSync(assetsDir)) return 0;
      const result = execSync(`find "${assetsDir}" -type f | wc -l`, { encoding: 'utf8' });
      return parseInt(result.trim()) || 0;
    } catch (error) {
      return 0;
    }
  }

  // Security analysis methods

  checkSecurityMiddleware() {
    try {
      const middlewarePath = path.join(this.projectRoot, 'internal/adapters/primary/http/middleware.go');
      const content = fs.readFileSync(middlewarePath, 'utf8');
      return content.includes('securityHeadersMiddleware') && content.includes('rateLimitMiddleware');
    } catch (error) {
      return false;
    }
  }

  checkCORSConfig() {
    try {
      const serverPath = path.join(this.projectRoot, 'internal/adapters/primary/http/server.go');
      const content = fs.readFileSync(serverPath, 'utf8');
      return content.includes('cors.New') && content.includes('AllowedOrigins');
    } catch (error) {
      return false;
    }
  }

  checkRateLimiting() {
    try {
      const middlewarePath = path.join(this.projectRoot, 'internal/adapters/primary/http/middleware.go');
      const content = fs.readFileSync(middlewarePath, 'utf8');
      return content.includes('rateLimitMiddleware') || content.includes('rate limit');
    } catch (error) {
      return false;
    }
  }

  checkInputSanitization() {
    try {
      const result = execSync(`grep -r "bluemonday\\|sanitize" "${this.projectRoot}/internal" || true`, { encoding: 'utf8' });
      return result.trim().length > 0;
    } catch (error) {
      return false;
    }
  }

  checkPathValidation() {
    try {
      const serverPath = path.join(this.projectRoot, 'internal/adapters/primary/http/server.go');
      const content = fs.readFileSync(serverPath, 'utf8');
      return content.includes('secureFileServer') && content.includes('filepath.Clean');
    } catch (error) {
      return false;
    }
  }

  getSecurityTestCoverage() {
    try {
      const result = execSync(`find "${this.projectRoot}" -name "*_test.go" -exec grep -l "security\\|sanitiz\\|cors\\|rate.*limit" {} \\; | wc -l`, { encoding: 'utf8' });
      return parseInt(result.trim()) || 0;
    } catch (error) {
      return 0;
    }
  }

  async checkVulnerabilities() {
    try {
      // Check if govulncheck is available and run it
      execSync('which govulncheck', { encoding: 'utf8' });
      const result = execSync('govulncheck ./...', { 
        cwd: this.projectRoot,
        encoding: 'utf8',
        timeout: 30000 
      });
      return result.includes('No vulnerabilities found') ? 0 : 1;
    } catch (error) {
      return -1; // Unknown (tool not available or error)
    }
  }

  // Performance analysis methods

  hasPerformanceTests() {
    try {
      const result = execSync(`find "${this.projectRoot}" -name "*_test.go" -exec grep -l "Benchmark\\|benchmark" {} \\; | wc -l`, { encoding: 'utf8' });
      return parseInt(result.trim()) > 0;
    } catch (error) {
      return false;
    }
  }

  checkProfilerSupport() {
    try {
      const result = execSync(`grep -r "pprof" "${this.projectRoot}" || true`, { encoding: 'utf8' });
      return result.trim().length > 0;
    } catch (error) {
      return false;
    }
  }

  checkCaching() {
    try {
      const result = execSync(`grep -r "cache\\|Cache" "${this.projectRoot}/internal" || true`, { encoding: 'utf8' });
      return result.trim().length > 0;
    } catch (error) {
      return false;
    }
  }

  checkAssetOptimization() {
    try {
      const makefilePath = path.join(this.projectRoot, 'Makefile');
      const content = fs.readFileSync(makefilePath, 'utf8');
      return content.includes('minify') || content.includes('compress');
    } catch (error) {
      return false;
    }
  }

  estimateStartupTime() {
    // Rough estimation based on component count and plugin count
    const baseTime = 50; // ms
    const pluginOverhead = this.metrics.pluginCount * 20; // ms per plugin
    const themeOverhead = this.metrics.themeCount * 10; // ms per theme
    return baseTime + pluginOverhead + themeOverhead;
  }

  analyzePluginLoadingStrategy() {
    try {
      const loaderPath = path.join(this.projectRoot, 'internal/adapters/secondary/plugin/loader.go');
      const content = fs.readFileSync(loaderPath, 'utf8');
      
      if (content.includes('cache') || content.includes('Cache')) {
        return 'cached';
      } else if (content.includes('lazy') || content.includes('Lazy')) {
        return 'lazy';
      } else {
        return 'eager';
      }
    } catch (error) {
      return 'unknown';
    }
  }

  // Code quality methods

  checkGolangciLint() {
    try {
      return fs.existsSync(path.join(this.projectRoot, '.golangci.yml')) ||
             fs.existsSync(path.join(this.projectRoot, '.golangci.yaml'));
    } catch (error) {
      return false;
    }
  }

  checkSecurityChecks() {
    try {
      const makefilePath = path.join(this.projectRoot, 'Makefile');
      const content = fs.readFileSync(makefilePath, 'utf8');
      return content.includes('gosec') || content.includes('govulncheck');
    } catch (error) {
      return false;
    }
  }

  calculateHealthScore(analysis) {
    let score = 100;

    // Component health (30%)
    const components = analysis.architecture;
    const healthyComponents = Object.values(components).filter(c => !c.error && c.hasTests).length;
    const totalComponents = Object.keys(components).length;
    if (totalComponents > 0) {
      const componentScore = (healthyComponents / totalComponents) * 30;
      score = score - 30 + componentScore;
    }

    // Security posture (25%)
    const securityScore = (this.metrics.securityScore / 100) * 25;
    score = score - 25 + securityScore;

    // Test coverage (20%)
    const testRatio = analysis.codeMetrics.testToCodeRatio || 0;
    const testScore = Math.min(testRatio, 80) / 80 * 20; // Cap at 80% for full score
    score = score - 20 + testScore;

    // Plugin system health (15%)
    const pluginHealth = Object.values(analysis.plugins).filter(p => p.hasBinary && p.hasTests).length;
    const totalPlugins = Object.keys(analysis.plugins).length;
    if (totalPlugins > 0) {
      const pluginScore = (pluginHealth / totalPlugins) * 15;
      score = score - 15 + pluginScore;
    }

    // Code quality (10%)
    const qualityScore = (analysis.codeMetrics.hasGolangciLint ? 5 : 0) + 
                        (analysis.codeMetrics.hasSecurityChecks ? 5 : 0);
    score = score - 10 + qualityScore;

    return Math.max(0, Math.round(score));
  }
}

// CLI interface
if (require.main === module) {
  const monitor = new ArchitectureMonitor();
  monitor.analyzeArchitecture()
    .then(result => {
      console.log(`\n🎯 Architecture Health Score: ${result.healthScore}/100`);
      console.log(`📦 Components Analyzed: ${Object.keys(result.architecture).length}`);
      console.log(`🔌 Plugins Discovered: ${Object.keys(result.plugins).length}`);
      console.log(`🎨 Themes Available: ${Object.keys(result.themes).length}`);
      console.log(`🔒 Security Score: ${result.security ? monitor.metrics.securityScore : 'N/A'}/100`);
      console.log(`⚡ Estimated Startup Time: ${result.performance.estimatedStartupTime}ms`);
      
      if (result.healthScore >= 90) {
        console.log('✅ Architecture health is excellent!');
      } else if (result.healthScore >= 75) {
        console.log('⚠️  Architecture health is good, but could be improved.');
      } else {
        console.log('❌ Architecture health needs attention.');
      }
    })
    .catch(error => {
      console.error('❌ Error during architecture analysis:', error);
      process.exit(1);
    });
}

module.exports = ArchitectureMonitor;