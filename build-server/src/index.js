const fs = require('fs');
const path = require('path');
const mime = require('mime-types');
const { spawn } = require('child_process');
const { Publisher } = require('./bus/kafka');
const { getBuildCommand } = require('./build');
const { createStorage } = require('./storage/factory');

function run(cmd, cwd, onLine) {
  return new Promise((resolve, reject) => {
    const child = spawn('bash', ['-lc', cmd], { cwd });
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
    child.on('close', (code) => (code === 0 ? resolve(code) : reject(new Error(`exit code ${code}`))));
    child.on('error', reject);
  });
}

async function uploadDir(storage, dir, keyPrefix, publish) {
  const entries = fs.readdirSync(dir, { recursive: true });
  for (const entry of entries) {
    const filePath = path.join(dir, entry);
    if (!fs.lstatSync(filePath).isDirectory()) {
      await storage.putFile(`${keyPrefix}/${entry.split(path.sep).join('/')}`, fs.createReadStream(filePath), mime.lookup(filePath));
      await publish(`Uploaded ${entry}`);
    }
  }
  return entries.filter((e) => !fs.lstatSync(path.join(dir, e)).isDirectory()).length;
}

async function main() {
  const publisher = Publisher.fromEnv();
  const storage = createStorage();
  const outDir = path.join(__dirname, '..', 'output');

  await publisher.connect();
  await publisher.status('IN_PROGRESS');
  await publisher.log(`Using build command: ${getBuildCommand(process.env.FRAMEWORK)}`);

  await run(getBuildCommand(process.env.FRAMEWORK), outDir, (line) => {
    console.log(line);
    void publisher.log(line).catch((err) => console.error('publish failed:', err.message));
  });

  const distDir = path.join(outDir, 'dist');
  if (!fs.existsSync(distDir)) throw new Error('Build output directory not found');

  const count = await uploadDir(storage, distDir, `${process.env.OUTPUT_PREFIX || '__output'}/${process.env.DEPLOYMENT_ID}`, (msg) => publisher.log(msg));
  await publisher.log(`Build output uploaded successfully (${count} files)`);
  await publisher.status('COMPLETED');
  process.exit(0);
}

main().catch(async (err) => {
  console.error('Build failed:', err.message);
  try {
    const publisher = Publisher.fromEnv();
    await publisher.connect();
    await publisher.log(`Error: ${err.message}`);
    await publisher.status('FAILED');
  } catch (publishErr) {
    console.error('Could not publish failure:', publishErr.message);
  }
  process.exit(1);
});

module.exports = { run, uploadDir };
