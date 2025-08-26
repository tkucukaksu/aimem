#!/usr/bin/env node

const os = require('os');
const path = require('path');
const fs = require('fs');

/**
 * Pre-install script for AIMem
 * Checks system requirements and Go availability
 */

function checkSystemRequirements() {
  console.log('🔍 Checking system requirements...');
  
  const platform = os.platform();
  const arch = os.arch();
  
  console.log(`Platform: ${platform}`);
  console.log(`Architecture: ${arch}`);
  
  // Check supported platforms
  const supportedPlatforms = ['darwin', 'linux', 'win32'];
  const supportedArchs = ['x64', 'arm64'];
  
  if (!supportedPlatforms.includes(platform)) {
    console.warn(`⚠️  Platform ${platform} is not officially supported`);
    console.warn('AIMem may still work, but you might need to build from source');
  }
  
  if (!supportedArchs.includes(arch)) {
    console.warn(`⚠️  Architecture ${arch} is not officially supported`);
    console.warn('AIMem may still work, but you might need to build from source');
  }
  
  console.log('✅ System requirements check completed');
}

function checkGoInstallation() {
  const { execSync } = require('child_process');
  
  try {
    const goVersion = execSync('go version', { encoding: 'utf-8' }).trim();
    console.log(`✅ Go found: ${goVersion}`);
    return true;
  } catch (error) {
    console.log('ℹ️  Go not found in PATH');
    console.log('🔧 Pre-built binaries will be used instead');
    return false;
  }
}

function createDistDirectory() {
  const distPath = path.join(__dirname, '..', 'dist');
  
  if (!fs.existsSync(distPath)) {
    fs.mkdirSync(distPath, { recursive: true });
    console.log('📁 Created dist directory');
  }
}

function main() {
  console.log('🚀 AIMem pre-installation...\n');
  
  try {
    checkSystemRequirements();
    checkGoInstallation();
    createDistDirectory();
    
    console.log('\n✅ Pre-installation completed successfully');
  } catch (error) {
    console.error('❌ Pre-installation failed:', error.message);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}