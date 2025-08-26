#!/usr/bin/env node

const { execSync, spawn } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');

/**
 * Post-install script for AIMem
 * Builds binary if Go is available, or downloads pre-built binary
 */

function hasGo() {
  try {
    execSync('go version', { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

function buildFromSource() {
  console.log('🔨 Building AIMem from source...');
  
  try {
    const buildCmd = 'go build -o dist/aimem cmd/aimem/main.go';
    execSync(buildCmd, { 
      stdio: 'inherit',
      cwd: path.join(__dirname, '..')
    });
    
    console.log('✅ Build completed successfully');
    return true;
  } catch (error) {
    console.error('❌ Build failed:', error.message);
    return false;
  }
}

function downloadPrebuiltBinary() {
  console.log('📦 Attempting to use pre-built binary...');
  
  const platform = os.platform();
  const arch = os.arch();
  
  // Map to binary names
  const platformMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'win32'
  };
  
  const archMap = {
    'x64': 'x64',
    'arm64': 'arm64'
  };
  
  const mappedPlatform = platformMap[platform];
  const mappedArch = archMap[arch];
  
  if (!mappedPlatform || !mappedArch) {
    console.error(`❌ No pre-built binary for ${platform}-${arch}`);
    return false;
  }
  
  // For now, we'll just create a placeholder
  // In production, you'd download from GitHub releases or npm registry
  const binaryName = platform === 'win32' ? 'aimem.exe' : 'aimem';
  const distPath = path.join(__dirname, '..', 'dist');
  const binaryPath = path.join(distPath, binaryName);
  
  // If we already have a local binary, copy it
  const localBinary = path.join(__dirname, '..', 'aimem');
  if (fs.existsSync(localBinary)) {
    fs.copyFileSync(localBinary, binaryPath);
    fs.chmodSync(binaryPath, 0o755);
    console.log('✅ Local binary copied to dist/');
    return true;
  }
  
  console.log('ℹ️  No pre-built binary available');
  console.log('💡 Please build manually with: npm run build');
  return false;
}

function verifyInstallation() {
  const binaryPath = path.join(__dirname, '..', 'dist', 'aimem');
  
  if (fs.existsSync(binaryPath)) {
    try {
      // Test binary execution
      execSync(`"${binaryPath}" -version`, { stdio: 'ignore', timeout: 5000 });
      console.log('✅ Binary verification successful');
      return true;
    } catch (error) {
      console.log('⚠️  Binary verification failed');
      return false;
    }
  }
  
  console.log('ℹ️  Binary not found in dist/');
  return false;
}

function createConfigFiles() {
  const configPath = path.join(__dirname, '..', 'aimem.yaml');
  
  if (!fs.existsSync(configPath)) {
    console.log('📝 Creating default configuration...');
    
    const defaultConfig = `# AIMem Configuration File
# AI Memory Management Server Settings

# Redis Configuration
redis:
  host: "localhost:6379"
  password: ""
  db: 0
  pool_size: 10

# Memory Management Settings
memory:
  max_session_size: "10MB"
  chunk_size: 1024
  max_chunks_per_query: 5
  ttl_default: "24h"

# Embedding Service Configuration
embedding:
  model: "all-MiniLM-L6-v2"
  cache_size: 1000
  batch_size: 32

# Performance Tuning
performance:
  compression_enabled: true
  async_processing: true
  cache_embeddings: true

# MCP Server Information
mcp:
  server_name: "AIMem"
  version: "1.0.0"
  description: "AI Memory Management Server - Intelligent context storage and retrieval"
`;
    
    fs.writeFileSync(configPath, defaultConfig);
    console.log('✅ Default configuration created');
  }
}

function showPostInstallMessage() {
  const packageJson = require('../package.json');
  
  console.log('\n🎉 AIMem installation completed!\n');
  console.log(`📦 Package: ${packageJson.name}@${packageJson.version}`);
  console.log('\n📋 Quick Start:');
  console.log('  1. Start Redis: redis-server');
  console.log('  2. Run AIMem: npx @ybo/aimem');
  console.log('  3. Add to MCP client configuration');
  console.log('\n📖 For detailed setup instructions:');
  console.log('  https://github.com/tarkank/aimem#readme');
  console.log('\n🧠 Smart Context Manager ready for intelligent AI development!');
}

function main() {
  console.log('⚙️  AIMem post-installation...\n');
  
  let success = false;
  
  if (hasGo()) {
    success = buildFromSource();
  }
  
  if (!success) {
    success = downloadPrebuiltBinary();
  }
  
  createConfigFiles();
  
  if (success) {
    verifyInstallation();
  }
  
  showPostInstallMessage();
  
  if (!success) {
    console.log('\n⚠️  Installation completed with warnings');
    console.log('💡 You may need to build manually or check system requirements');
    process.exit(0); // Don't fail installation, just warn
  }
}

if (require.main === module) {
  main();
}