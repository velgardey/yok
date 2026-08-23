const { S3StorageAdapter } = require('./aws-s3');

function createStorage(env = process.env) {
  switch (env.STORAGE_PROVIDER || 'aws') {
    case 'aws':
      return new S3StorageAdapter(env);
    default:
      throw new Error(`Unsupported STORAGE_PROVIDER: ${env.STORAGE_PROVIDER}`);
  }
}

module.exports = { createStorage };
