const fs = require('fs');
const { Kafka, Partitioners } = require('kafkajs');

const VALID_STATUSES = ['PENDING', 'QUEUED', 'IN_PROGRESS', 'COMPLETED', 'FAILED'];

function createProducerFromEnv(env = process.env) {
  const ssl = env.KAFKA_CA_PATH
    ? { ca: [fs.readFileSync(env.KAFKA_CA_PATH, 'utf-8')] }
    : true;
  const kafka = new Kafka({
    clientId: `build-server-${env.DEPLOYMENT_ID}`,
    brokers: [env.KAFKA_BROKER],
    ssl,
    sasl: { username: env.KAFKA_USERNAME, password: env.KAFKA_PASSWORD, mechanism: 'plain' },
  });
  return kafka.producer({
    createPartitioner: Partitioners.DefaultPartitioner,
    allowAutoTopicCreation: false,
  });
}

function logMessage({ projectId, deploymentId, log }) {
  return { type: 'log', projectId, deploymentId, log };
}

function statusMessage({ projectId, deploymentId, status }) {
  if (!VALID_STATUSES.includes(status)) {
    throw new Error(`Invalid status: ${status}`);
  }
  return { type: 'status', projectId, deploymentId, status };
}

class Publisher {
  constructor(producer, topic, base) {
    this.producer = producer;
    this.topic = topic;
    this.base = base;
  }

  static fromEnv(env = process.env) {
    return new Publisher(createProducerFromEnv(env), env.KAFKA_TOPIC, {
      projectId: env.PROJECT_ID,
      deploymentId: env.DEPLOYMENT_ID,
    });
  }

  async connect() {
    await this.producer.connect();
  }

  async log(log) {
    await this.producer.send({
      topic: this.topic,
      messages: [{ key: 'log', value: JSON.stringify(logMessage({ ...this.base, log })) }],
    });
  }

  async status(status) {
    await this.producer.send({
      topic: this.topic,
      messages: [{ key: 'status', value: JSON.stringify(statusMessage({ ...this.base, status })) }],
    });
  }
}

module.exports = { Publisher, createProducerFromEnv, logMessage, statusMessage, VALID_STATUSES };
