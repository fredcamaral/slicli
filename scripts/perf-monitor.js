#!/usr/bin/env node

/**
 * Performance Monitoring Script for slicli
 * Tracks key business and technical metrics for performance optimization
 */

const { performance } = require('perf_hooks');
const fs = require('fs');
const path = require('path');

class PerformanceMonitor {
  constructor() {
    this.metrics = {
      presentations: new Map(),  // presentation loading times
      plugins: new Map(),        // plugin execution times
      themes: new Map(),         // theme loading times
      exports: new Map(),        // export generation times
      serverStartup: [],         // server startup times
      memoryUsage: [],          // memory usage samples
      errors: new Map(),        // error tracking
      userSessions: new Map()   // user session data
    };
    
    this.businessMetrics = {
      presentationsCreated: 0,
      pluginsUsed: new Map(),
      themesApplied: new Map(),
      exportsGenerated: new Map(),
      averageSessionDuration: 0,
      // Community engagement metrics
      pluginDownloads: new Map(),       // plugin_id -> download count
      pluginInstalls: new Map(),        // plugin_id -> install count  
      themeDownloads: new Map(),        // theme_id -> download count
      themeApplies: new Map(),          // theme_id -> apply count
      activeContributors: new Set(),    // set of active contributor IDs
      communityProjects: new Set(),     // set of community project IDs
      totalDownloads: 0,                // total community downloads
      monthlyDownloads: new Map(),      // month -> downloads
      communityFunnel: {                // community engagement tracking
        visitors: 0,
        users: 0,
        contributors: 0,
        maintainers: 0
      },
      userEngagement: {                 // engagement metrics
        dailyActiveUsers: new Set(),
        weeklyActiveUsers: new Set(), 
        monthlyActiveUsers: new Set()
      }
    };
  }

  // Track presentation loading performance
  trackPresentationLoad(filename, duration, slideCount) {
    if (!this.metrics.presentations.has(filename)) {
      this.metrics.presentations.set(filename, []);
    }
    
    this.metrics.presentations.get(filename).push({
      duration,
      slideCount,
      timestamp: Date.now(),
      slidesPerSecond: slideCount / (duration / 1000)
    });
    
    this.businessMetrics.presentationsCreated++;
  }

  // Track plugin execution performance
  trackPluginExecution(pluginName, duration, success = true) {
    if (!this.metrics.plugins.has(pluginName)) {
      this.metrics.plugins.set(pluginName, []);
    }
    
    this.metrics.plugins.get(pluginName).push({
      duration,
      success,
      timestamp: Date.now()
    });
    
    // Track business usage
    const count = this.businessMetrics.pluginsUsed.get(pluginName) || 0;
    this.businessMetrics.pluginsUsed.set(pluginName, count + 1);
  }

  // Track theme loading performance
  trackThemeLoad(themeName, duration, assetCount) {
    if (!this.metrics.themes.has(themeName)) {
      this.metrics.themes.set(themeName, []);
    }
    
    this.metrics.themes.get(themeName).push({
      duration,
      assetCount,
      timestamp: Date.now()
    });
    
    // Track business usage
    const count = this.businessMetrics.themesApplied.get(themeName) || 0;
    this.businessMetrics.themesApplied.set(themeName, count + 1);
  }

  // Track export performance
  trackExport(format, duration, fileSize, success = true) {
    if (!this.metrics.exports.has(format)) {
      this.metrics.exports.set(format, []);
    }
    
    this.metrics.exports.get(format).push({
      duration,
      fileSize,
      success,
      timestamp: Date.now(),
      throughput: fileSize / (duration / 1000) // bytes per second
    });
    
    // Track business usage
    const count = this.businessMetrics.exportsGenerated.get(format) || 0;
    this.businessMetrics.exportsGenerated.set(format, count + 1);
  }

  // Track server startup performance
  trackServerStartup(duration, port) {
    this.metrics.serverStartup.push({
      duration,
      port,
      timestamp: Date.now()
    });
  }

  // Track memory usage
  trackMemoryUsage() {
    const memInfo = process.memoryUsage();
    this.metrics.memoryUsage.push({
      ...memInfo,
      timestamp: Date.now()
    });
    
    // Keep only last 100 samples
    if (this.metrics.memoryUsage.length > 100) {
      this.metrics.memoryUsage.shift();
    }
  }

  // Track errors
  trackError(errorType, message, context = {}) {
    if (!this.metrics.errors.has(errorType)) {
      this.metrics.errors.set(errorType, []);
    }
    
    this.metrics.errors.get(errorType).push({
      message,
      context,
      timestamp: Date.now()
    });
  }

  // Track plugin download
  trackPluginDownload(pluginId, userId = null) {
    const count = this.businessMetrics.pluginDownloads.get(pluginId) || 0;
    this.businessMetrics.pluginDownloads.set(pluginId, count + 1);
    this.businessMetrics.totalDownloads++;
    
    const currentMonth = new Date().toISOString().slice(0, 7); // YYYY-MM
    const monthlyCount = this.businessMetrics.monthlyDownloads.get(currentMonth) || 0;
    this.businessMetrics.monthlyDownloads.set(currentMonth, monthlyCount + 1);
    
    if (userId) {
      this.trackUserEngagement(userId);
    }
  }

  // Track theme download
  trackThemeDownload(themeId, userId = null) {
    const count = this.businessMetrics.themeDownloads.get(themeId) || 0;
    this.businessMetrics.themeDownloads.set(themeId, count + 1);
    this.businessMetrics.totalDownloads++;
    
    const currentMonth = new Date().toISOString().slice(0, 7); // YYYY-MM
    const monthlyCount = this.businessMetrics.monthlyDownloads.get(currentMonth) || 0;
    this.businessMetrics.monthlyDownloads.set(currentMonth, monthlyCount + 1);
    
    if (userId) {
      this.trackUserEngagement(userId);
    }
  }

  // Track community contribution
  trackCommunityContribution(userId, contributionType = 'code') {
    this.businessMetrics.activeContributors.add(userId);
    
    if (userId) {
      this.trackUserEngagement(userId);
      this.trackCommunityLevel(userId, contributionType);
    }
  }

  // Track community engagement levels
  trackCommunityLevel(userId, level) {
    switch (level) {
      case 'visitor':
        this.businessMetrics.communityFunnel.visitors++;
        break;
      case 'user':
        this.businessMetrics.communityFunnel.users++;
        break;
      case 'contributor':
        this.businessMetrics.communityFunnel.contributors++;
        this.businessMetrics.activeContributors.add(userId);
        break;
      case 'maintainer':
        this.businessMetrics.communityFunnel.maintainers++;
        this.businessMetrics.activeContributors.add(userId);
        break;
    }
  }

  // Track user engagement
  trackUserEngagement(userId) {
    const now = new Date();
    const today = now.toISOString().slice(0, 10); // YYYY-MM-DD
    const thisWeek = getWeekKey(now);
    const thisMonth = now.toISOString().slice(0, 7); // YYYY-MM
    
    this.businessMetrics.userEngagement.dailyActiveUsers.add(`${today}:${userId}`);
    this.businessMetrics.userEngagement.weeklyActiveUsers.add(`${thisWeek}:${userId}`);
    this.businessMetrics.userEngagement.monthlyActiveUsers.add(`${thisMonth}:${userId}`);
  }

  // Track marketplace activity
  trackMarketplaceActivity(action, itemType, itemId, userId = null) {
    const activityKey = `${action}_${itemType}`; // e.g., 'view_plugin', 'download_theme'
    
    if (!this.metrics.marketplace) {
      this.metrics.marketplace = new Map();
    }
    
    if (!this.metrics.marketplace.has(activityKey)) {
      this.metrics.marketplace.set(activityKey, []);
    }
    
    this.metrics.marketplace.get(activityKey).push({
      itemId,
      userId,
      timestamp: Date.now()
    });
    
    if (userId) {
      this.trackUserEngagement(userId);
    }
  }

  // Track user session
  startSession(sessionId, userId = null) {
    this.metrics.userSessions.set(sessionId, {
      startTime: Date.now(),
      userId: userId,
      presentationsViewed: 0,
      pluginsUsed: new Set(),
      themesUsed: new Set()
    });
    
    if (userId) {
      this.trackUserEngagement(userId);
    }
  }

  endSession(sessionId) {
    const session = this.metrics.userSessions.get(sessionId);
    if (session) {
      session.endTime = Date.now();
      session.duration = session.endTime - session.startTime;
      
      // Update average session duration
      const sessions = Array.from(this.metrics.userSessions.values())
        .filter(s => s.endTime);
      
      if (sessions.length > 0) {
        const totalDuration = sessions.reduce((sum, s) => sum + s.duration, 0);
        this.businessMetrics.averageSessionDuration = totalDuration / sessions.length;
      }
    }
  }

  // Generate performance report
  generateReport() {
    const report = {
      timestamp: new Date().toISOString(),
      performance: this.generatePerformanceMetrics(),
      business: this.generateBusinessMetrics(),
      health: this.calculateHealthScore(),
      recommendations: this.generateRecommendations()
    };
    
    return report;
  }

  generatePerformanceMetrics() {
    return {
      presentations: this.analyzePerformanceMap(this.metrics.presentations, 'presentation loading'),
      plugins: this.analyzePerformanceMap(this.metrics.plugins, 'plugin execution'),
      themes: this.analyzePerformanceMap(this.metrics.themes, 'theme loading'),
      exports: this.analyzePerformanceMap(this.metrics.exports, 'export generation'),
      serverStartup: this.analyzeArray(this.metrics.serverStartup, 'duration', 'server startup'),
      memory: this.analyzeMemoryUsage(),
      errors: this.analyzeErrors()
    };
  }

  generateBusinessMetrics() {
    const now = new Date();
    const today = now.toISOString().slice(0, 10);
    const thisWeek = getWeekKey(now);
    const thisMonth = now.toISOString().slice(0, 7);
    
    return {
      // Core usage metrics
      presentationsCreated: this.businessMetrics.presentationsCreated,
      mostUsedPlugins: this.getTopUsed(this.businessMetrics.pluginsUsed, 5),
      mostUsedThemes: this.getTopUsed(this.businessMetrics.themesApplied, 5),
      mostUsedExportFormats: this.getTopUsed(this.businessMetrics.exportsGenerated, 5),
      averageSessionDuration: Math.round(this.businessMetrics.averageSessionDuration / 1000), // seconds
      activeUsers: this.metrics.userSessions.size,
      
      // Community metrics
      totalDownloads: this.businessMetrics.totalDownloads,
      monthlyDownloads: this.getCurrentMonthDownloads(),
      topDownloadedPlugins: this.getTopUsed(this.businessMetrics.pluginDownloads, 5),
      topDownloadedThemes: this.getTopUsed(this.businessMetrics.themeDownloads, 5),
      
      // Community engagement funnel
      communityFunnel: {
        visitors: this.businessMetrics.communityFunnel.visitors,
        users: this.businessMetrics.communityFunnel.users,  
        contributors: this.businessMetrics.communityFunnel.contributors,
        maintainers: this.businessMetrics.communityFunnel.maintainers,
        contributionRate: this.calculateContributionRate()
      },
      
      // Engagement metrics
      userEngagement: {
        dailyActiveUsers: this.countUsersByPeriod(this.businessMetrics.userEngagement.dailyActiveUsers, today),
        weeklyActiveUsers: this.countUsersByPeriod(this.businessMetrics.userEngagement.weeklyActiveUsers, thisWeek),
        monthlyActiveUsers: this.countUsersByPeriod(this.businessMetrics.userEngagement.monthlyActiveUsers, thisMonth)
      },
      
      // Community health indicators
      communityHealth: {
        totalActiveContributors: this.businessMetrics.activeContributors.size,
        totalCommunityProjects: this.businessMetrics.communityProjects.size,
        contributorGrowthRate: this.calculateContributorGrowthRate()
      },
      
      // Marketplace metrics
      marketplace: {
        totalPluginDownloads: this.sumMapValues(this.businessMetrics.pluginDownloads),
        totalThemeDownloads: this.sumMapValues(this.businessMetrics.themeDownloads),
        totalInstalls: this.sumMapValues(this.businessMetrics.pluginInstalls),
        totalThemeApplies: this.sumMapValues(this.businessMetrics.themeApplies)
      }
    };
  }

  analyzePerformanceMap(performanceMap, label) {
    const allMetrics = [];
    const byItem = {};
    
    performanceMap.forEach((metrics, item) => {
      const durations = metrics.map(m => m.duration);
      allMetrics.push(...durations);
      
      byItem[item] = this.calculateStats(durations);
    });
    
    return {
      overall: this.calculateStats(allMetrics),
      byItem,
      slowItems: this.findSlowItems(byItem, label)
    };
  }

  analyzeArray(array, field, label) {
    const values = array.map(item => item[field]);
    return {
      stats: this.calculateStats(values),
      recent: values.slice(-10), // Last 10 measurements
      trend: this.calculateTrend(values)
    };
  }

  analyzeMemoryUsage() {
    if (this.metrics.memoryUsage.length === 0) return null;
    
    const recent = this.metrics.memoryUsage.slice(-10);
    const rssValues = recent.map(m => m.rss);
    const heapUsedValues = recent.map(m => m.heapUsed);
    
    return {
      rss: this.calculateStats(rssValues),
      heapUsed: this.calculateStats(heapUsedValues),
      trend: {
        rss: this.calculateTrend(rssValues),
        heapUsed: this.calculateTrend(heapUsedValues)
      },
      current: recent[recent.length - 1]
    };
  }

  analyzeErrors() {
    const errorSummary = {};
    let totalErrors = 0;
    
    this.metrics.errors.forEach((errors, type) => {
      errorSummary[type] = {
        count: errors.length,
        recent: errors.slice(-5).map(e => ({
          message: e.message,
          timestamp: new Date(e.timestamp).toISOString()
        }))
      };
      totalErrors += errors.length;
    });
    
    return {
      totalErrors,
      byType: errorSummary,
      errorRate: this.calculateErrorRate()
    };
  }

  calculateStats(values) {
    if (values.length === 0) return null;
    
    const sorted = [...values].sort((a, b) => a - b);
    const sum = values.reduce((a, b) => a + b, 0);
    
    return {
      count: values.length,
      min: sorted[0],
      max: sorted[sorted.length - 1],
      avg: sum / values.length,
      median: sorted[Math.floor(sorted.length / 2)],
      p95: sorted[Math.floor(sorted.length * 0.95)],
      p99: sorted[Math.floor(sorted.length * 0.99)]
    };
  }

  calculateTrend(values) {
    if (values.length < 2) return 'insufficient_data';
    
    const recent = values.slice(-5);
    const older = values.slice(-10, -5);
    
    if (older.length === 0) return 'insufficient_data';
    
    const recentAvg = recent.reduce((a, b) => a + b, 0) / recent.length;
    const olderAvg = older.reduce((a, b) => a + b, 0) / older.length;
    
    const change = ((recentAvg - olderAvg) / olderAvg) * 100;
    
    if (Math.abs(change) < 5) return 'stable';
    return change > 0 ? 'increasing' : 'decreasing';
  }

  findSlowItems(byItem, label, threshold = 1000) {
    const slowItems = [];
    
    Object.entries(byItem).forEach(([item, stats]) => {
      if (stats && stats.avg > threshold) {
        slowItems.push({
          item,
          avgDuration: Math.round(stats.avg),
          p95Duration: Math.round(stats.p95),
          count: stats.count
        });
      }
    });
    
    return slowItems.sort((a, b) => b.avgDuration - a.avgDuration);
  }

  getTopUsed(usageMap, limit = 5) {
    return Array.from(usageMap.entries())
      .sort((a, b) => b[1] - a[1])
      .slice(0, limit)
      .map(([item, count]) => ({ item, count }));
  }

  getCurrentMonthDownloads() {
    const currentMonth = new Date().toISOString().slice(0, 7);
    return this.businessMetrics.monthlyDownloads.get(currentMonth) || 0;
  }

  calculateContributionRate() {
    const total = this.businessMetrics.communityFunnel.visitors + 
                  this.businessMetrics.communityFunnel.users +
                  this.businessMetrics.communityFunnel.contributors +
                  this.businessMetrics.communityFunnel.maintainers;
    
    if (total === 0) return 0;
    
    const contributors = this.businessMetrics.communityFunnel.contributors +
                        this.businessMetrics.communityFunnel.maintainers;
    
    return (contributors / total) * 100;
  }

  calculateContributorGrowthRate() {
    // Simple implementation - could be enhanced with historical data
    const totalUsers = this.businessMetrics.communityFunnel.users;
    const totalContributors = this.businessMetrics.activeContributors.size;
    
    if (totalUsers === 0) return 0;
    
    return (totalContributors / totalUsers) * 100;
  }

  countUsersByPeriod(userSet, period) {
    let count = 0;
    userSet.forEach(entry => {
      if (entry.startsWith(period + ':')) {
        count++;
      }
    });
    return count;
  }

  sumMapValues(map) {
    let sum = 0;
    map.forEach(value => sum += value);
    return sum;
  }

  calculateErrorRate() {
    const totalOperations = this.businessMetrics.presentationsCreated;
    const totalErrors = Array.from(this.metrics.errors.values())
      .reduce((sum, errors) => sum + errors.length, 0);
    
    return totalOperations > 0 ? (totalErrors / totalOperations) * 100 : 0;
  }

  calculateHealthScore() {
    let score = 100;
    
    // Deduct for slow presentations (avg > 1000ms)
    const presentationStats = this.analyzePerformanceMap(this.metrics.presentations, 'presentations');
    if (presentationStats.overall && presentationStats.overall.avg > 1000) {
      score -= Math.min(20, (presentationStats.overall.avg - 1000) / 100);
    }
    
    // Deduct for plugin failures
    const pluginStats = this.analyzePerformanceMap(this.metrics.plugins, 'plugins');
    const failedPlugins = Object.values(pluginStats.byItem || {})
      .filter(stats => stats.successRate && stats.successRate < 95).length;
    score -= failedPlugins * 5;
    
    // Deduct for errors
    const errorRate = this.calculateErrorRate();
    score -= Math.min(30, errorRate * 10);
    
    // Deduct for memory issues
    const memoryStats = this.analyzeMemoryUsage();
    if (memoryStats && memoryStats.heapUsed.avg > 100 * 1024 * 1024) { // 100MB
      score -= 10;
    }
    
    return Math.max(0, Math.round(score));
  }

  generateRecommendations() {
    const recommendations = [];
    
    // Performance recommendations
    const presentationStats = this.analyzePerformanceMap(this.metrics.presentations, 'presentations');
    if (presentationStats.overall && presentationStats.overall.avg > 1000) {
      recommendations.push({
        type: 'performance',
        priority: 'high',
        message: `Presentation loading is slow (avg: ${Math.round(presentationStats.overall.avg)}ms). Consider optimizing markdown parsing or implementing caching.`
      });
    }
    
    // Plugin recommendations
    const slowPlugins = this.findSlowItems(
      this.analyzePerformanceMap(this.metrics.plugins, 'plugins').byItem,
      'plugins',
      500
    );
    
    if (slowPlugins.length > 0) {
      recommendations.push({
        type: 'performance',
        priority: 'medium',
        message: `Slow plugins detected: ${slowPlugins.map(p => p.item).join(', ')}. Consider optimizing plugin execution.`
      });
    }
    
    // Business recommendations
    const unusedPlugins = Array.from(this.businessMetrics.pluginsUsed.entries())
      .filter(([, count]) => count === 0);
    
    if (unusedPlugins.length > 0) {
      recommendations.push({
        type: 'business',
        priority: 'low',
        message: `Unused plugins detected: ${unusedPlugins.map(([name]) => name).join(', ')}. Consider removing or promoting these plugins.`
      });
    }
    
    // Memory recommendations
    const memoryStats = this.analyzeMemoryUsage();
    if (memoryStats && memoryStats.trend.heapUsed === 'increasing') {
      recommendations.push({
        type: 'performance',
        priority: 'medium',
        message: 'Memory usage is increasing. Check for memory leaks in plugin execution or file watching.'
      });
    }
    
    // Community recommendations
    const businessMetrics = this.generateBusinessMetrics();
    
    // Community growth recommendations
    if (businessMetrics.totalDownloads < 1000) {
      recommendations.push({
        type: 'community',
        priority: 'high',
        message: `Low download count (${businessMetrics.totalDownloads}). Focus on documentation, examples, and community outreach.`
      });
    }
    
    // Contribution funnel recommendations  
    if (businessMetrics.communityFunnel.contributionRate < 5) {
      recommendations.push({
        type: 'community',
        priority: 'medium',
        message: `Low contribution rate (${businessMetrics.communityFunnel.contributionRate.toFixed(1)}%). Improve contributor onboarding and good first issues.`
      });
    }
    
    // User engagement recommendations
    if (businessMetrics.userEngagement.dailyActiveUsers < 100) {
      recommendations.push({
        type: 'community', 
        priority: 'medium',
        message: `Low daily active users (${businessMetrics.userEngagement.dailyActiveUsers}). Increase content marketing and community engagement.`
      });
    }
    
    // Marketplace recommendations
    if (businessMetrics.marketplace.totalPluginDownloads === 0) {
      recommendations.push({
        type: 'community',
        priority: 'high',
        message: 'No plugin downloads detected. Improve plugin discoverability and create showcase examples.'
      });
    }
    
    return recommendations;
  }

  // Save report to file
  saveReport(filepath) {
    const report = this.generateReport();
    const reportJson = JSON.stringify(report, null, 2);
    
    try {
      fs.writeFileSync(filepath, reportJson);
      console.log(`Performance report saved to: ${filepath}`);
    } catch (error) {
      console.error(`Failed to save report: ${error.message}`);
    }
    
    return report;
  }

  // Start monitoring with periodic reports
  startMonitoring(options = {}) {
    const {
      interval = 60000,  // 1 minute
      reportFile = './performance-report.json',
      logToConsole = true
    } = options;
    
    console.log('Starting slicli performance monitoring...');
    console.log(`Report interval: ${interval}ms`);
    console.log(`Report file: ${reportFile}`);
    
    // Track memory usage periodically
    setInterval(() => {
      this.trackMemoryUsage();
    }, 10000); // Every 10 seconds
    
    // Generate reports periodically
    setInterval(() => {
      const report = this.generateReport();
      
      if (logToConsole) {
        console.log('\n--- Performance Report ---');
        console.log(`Health Score: ${report.health}/100`);
        console.log(`Presentations Created: ${report.business.presentationsCreated}`);
        console.log(`Active Users: ${report.business.activeUsers}`);
        
        if (report.recommendations.length > 0) {
          console.log('\nRecommendations:');
          report.recommendations.forEach(rec => {
            console.log(`  [${rec.priority.toUpperCase()}] ${rec.message}`);
          });
        }
        console.log('-------------------------\n');
      }
      
      // Save to file
      this.saveReport(reportFile);
      
    }, interval);
  }
}

// Simulate monitoring if run directly
if (require.main === module) {
  const monitor = new PerformanceMonitor();
  
  // Simulate some usage for demonstration
  console.log('Simulating slicli usage for demonstration...');
  
  // Simulate server startup
  monitor.trackServerStartup(450, 3000);
  
  // Simulate presentation loading
  monitor.trackPresentationLoad('demo.md', 340, 12);
  monitor.trackPresentationLoad('technical-talk.md', 890, 25);
  
  // Simulate plugin usage
  monitor.trackPluginExecution('mermaid', 120, true);
  monitor.trackPluginExecution('code-exec', 340, true);
  monitor.trackPluginExecution('syntax-highlight', 45, true);
  
  // Simulate theme loading
  monitor.trackThemeLoad('default', 89, 5);
  monitor.trackThemeLoad('minimal', 67, 3);
  
  // Simulate exports
  monitor.trackExport('pdf', 2300, 1024 * 500, true); // 500KB PDF
  monitor.trackExport('png', 890, 1024 * 150, true);  // 150KB PNG
  
  // Simulate user session
  monitor.startSession('demo-user-1');
  setTimeout(() => {
    monitor.endSession('demo-user-1');
  }, 5000);
  
  // Start monitoring
  monitor.startMonitoring({
    interval: 15000,  // 15 seconds for demo
    logToConsole: true,
    reportFile: './demo-performance-report.json'
  });
}

// Helper function to get week key
function getWeekKey(date) {
  const year = date.getFullYear();
  const startOfYear = new Date(year, 0, 1);
  const dayOfYear = Math.floor((date - startOfYear) / (24 * 60 * 60 * 1000));
  const weekNumber = Math.ceil((dayOfYear + startOfYear.getDay() + 1) / 7);
  return `${year}-W${weekNumber.toString().padStart(2, '0')}`;
}

module.exports = PerformanceMonitor;