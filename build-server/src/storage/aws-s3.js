const { S3Client, PutObjectCommand } = require('@aws-sdk/client-s3');

class S3StorageAdapter {
  constructor(env = process.env) {
    if (!env.AWS_S3_BUCKET) throw new Error('Missing AWS_S3_BUCKET');
    this.bucket = env.AWS_S3_BUCKET;
    this.client = new S3Client({
      region: env.AWS_REGION,
      credentials: {
        accessKeyId: env.AWS_ACCESS_KEY_ID,
        secretAccessKey: env.AWS_SECRET_ACCESS_KEY,
      },
    });
  }

  async putFile(key, body, contentType) {
    await this.client.send(new PutObjectCommand({ Bucket: this.bucket, Key: key, Body: body, ContentType: contentType }));
  }
}

module.exports = { S3StorageAdapter };
