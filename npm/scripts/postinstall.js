#!/usr/bin/env node

const https = require('https');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const VERSION = '2.0.0';
const REPO = 'visvioce/ccg';
const BINARY_NAME = 'ccg';

function getPlatform() {
  const platform = process.platform;
  const arch = process.arch;

  const platformMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'windows'
  };

  const archMap = {
    'x64': 'amd64',
    'arm64': 'arm64'
  };

  const os = platformMap[platform];
  const architecture = archMap[arch];

  if (!os || !architecture) {
    throw new Error(`Unsupported platform: ${platform}/${arch}`);
  }

  return { os, arch: architecture };
}

function getDownloadUrl(platform) {
  const ext = platform.os === 'windows' ? '.exe' : '';
  return `https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY_NAME}-${platform.os}-${platform.arch}${ext}`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);

    const request = (url) => {
      https.get(url, (response) => {
        if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
          request(response.headers.location);
          return;
        }

        if (response.statusCode !== 200) {
          reject(new Error(`Download failed: ${response.statusCode}`));
          return;
        }

        response.pipe(file);
        file.on('finish', () => {
          file.close();
          resolve();
        });
      }).on('error', (err) => {
        fs.unlink(dest, () => {});
        reject(err);
      });
    };

    request(url);
  });
}

async function main() {
  try {
    console.log('Installing CCG...');

    const platform = getPlatform();
    console.log(`Detected platform: ${platform.os}/${platform.arch}`);

    const url = getDownloadUrl(platform);
    console.log(`Downloading from: ${url}`);

    const binDir = path.join(__dirname, '..', 'bin');
    if (!fs.existsSync(binDir)) {
      fs.mkdirSync(binDir, { recursive: true });
    }

    const ext = platform.os === 'windows' ? '.exe' : '';
    const binaryPath = path.join(binDir, `ccg${ext}`);

    await download(url, binaryPath);
    fs.chmodSync(binaryPath, 0o755);

    console.log('');
    console.log('CCG installed successfully!');
    console.log('');
    console.log('Usage:');
    console.log('  ccg start       - Start the CCG server');
    console.log('  ccg stop        - Stop the CCG server');
    console.log('  ccg status      - Show server status');
    console.log('  ccg tui         - Open Terminal UI');
    console.log('');
  } catch (error) {
    console.error('Installation failed:', error.message);
    process.exit(1);
  }
}

main();
