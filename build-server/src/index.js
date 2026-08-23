const fs = require('fs');
const path = require('path');
const mime = require('mime-types');
const { spawn } = require('child_process');
const { Publisher } = require('./bus/kafka');
const { getBuildCommand } = require('./build');
const { createStorage } = require('./storage/factory');

function requiredEnv(name) {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required environment variable: ${name}`);
  return value;
}

function run(cmd, cwd, onLine, timeoutMs) {
  return new Promise((resolve, reject) => {
    const child = spawn('bash', ['-lc', cmd], { cwd });
    let settled = false;
    const settle = (fn, value) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      fn(value);
    };

    // Watchdog: a hung build must not leave its deployment in progress forever.
    const timer = setTimeout(() => {
      child.kill('SIGKILL');
      settle(reject, new Error(`build timed out after ${Math.round(timeoutMs / 1000)}s`));
    }, timeoutMs);

    const forward = (stream) => {
      stream.setEncoding('utf8');
      let buffer = '';
      stream.on('data', (chunk) => {
        buffer += chunk;
        const lines = buffer.split('\n');
        buffer = lines.pop() ?? '';
        for (const line of lines) onLine(line);
      });
      stream.on('end', () => {
        if (buffer) onLine(buffer);
      });
    };
    forward(child.stdout);
    forward(child.stderr);

    child.on('close', (code) => (code === 0 ? settle(resolve, code) : settle(reject, new Error(`exit code ${code}`))));
    child.on('error', (err) => settle(reject, err));
  });
}

// Locates the directory containing the built site. Frameworks differ on where
// they emit output, so probe the common candidates instead of hardcoding one.
function findOutputDir(outDir) {
  for (const candidate of ['dist', 'build', 'out', 'public']) {
    if (fs.existsSync(path.join(outDir, candidate))) return path.join(outDir, candidate);
  }
  // Angular emits dist/<project-name>/.
  const dist = path.join(outDir, 'dist');
  if (fs.existsSync(dist)) {
    const nested = fs.readdirSync(dist).find((entry) => fs.existsSync(path.join(dist, entry, 'index.html')));
    if (nested) return path.join(dist, nested);
  }
  return null;
}

async function uploadDir(storage, dir, keyPrefix, publish) {
  const entries = fs.readdirSync(dir, { recursive: true });
  let count = 0;
  for (const entry of entries) {
    const filePath = path.join(dir, entry);
    if (!fs.statSync(filePath).isFile()) continue;
    await storage.putFile(
      `${keyPrefix}/${entry.split(path.sep).join('/')}`,
      fs.createReadStream(filePath),
      mime.lookup(filePath) || 'application/octet-stream'
    );
    await publish(`Uploaded ${entry}`);
    count++;
  }
  return count;
}

// Log sends are fire-and-forget while the build streams, but they are tracked
// here so completion can drain them first - otherwise trailing logs can be
// lost or observed after the final status event.
function logTracker() {
  const pending = new Set();
  const publish = (publisher, message) => {
    const sent = publisher.log(message).catch((err) => console.error('publish failed:', err.message));
    pending.add(sent);
    sent.finally(() => pending.delete(sent));
    return sent;
  };
  return { pending, publish };
}

async function deployArtifacts({ publisher, storage, outDir, deploymentId }) {
  const { pending, publish } = logTracker();

  publish(publisher, `Using build command: ${getBuildCommand(process.env.FRAMEWORK)}`);
  await run(
    getBuildCommand(process.env.FRAMEWORK),
    outDir,
    (line) => {
      console.log(line);
      publish(publisher, line);
    },
    Number(process.env.BUILD_TIMEOUT_SECONDS || 900) * 1000
  );

  const outputDir = findOutputDir(outDir);
  if (!outputDir) throw new Error('Build output directory not found');

  const count = await uploadDir(
    storage,
    outputDir,
    `${process.env.OUTPUT_PREFIX || '__output'}/${deploymentId}`,
    (message) => publish(publisher, message)
  );
  publish(publisher, `Build output uploaded successfully (${count} files)`);

  await Promise.all(pending);
  await publisher.status('COMPLETED');
}

async function main() {
  const deploymentId = requiredEnv('DEPLOYMENT_ID');
  requiredEnv('PROJECT_ID');
  requiredEnv('KAFKA_BROKER');
  requiredEnv('KAFKA_TOPIC');

  const publisher = Publisher.fromEnv();
  const storage = createStorage();
  const outDir = path.join(__dirname, '..', 'output');
  let connected = false;

  try {
    await publisher.connect();
    connected = true;
    await publisher.status('IN_PROGRESS');
    await deployArtifacts({ publisher, storage, outDir, deploymentId });
  } catch (err) {
    console.error('Build failed:', err.message);
    if (connected) {
      try {
        await publisher.log(`Error: ${err.message}`);
        await publisher.status('FAILED');
      } catch (publishErr) {
        console.error('Could not publish failure:', publishErr.message);
      }
    }
    process.exitCode = 1;
  } finally {
    if (connected) await publisher.disconnect().catch(() => undefined);
  }
}

module.exports = { run, uploadDir };

if (require.main === module) {
  main();
}
