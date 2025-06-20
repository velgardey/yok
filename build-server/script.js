const { exec } = require('child_process');
const fs = require('fs');
const path = require('path');
const mime = require('mime-types');
const { S3Client, PutObjectCommand } = require('@aws-sdk/client-s3');
const {Kafka, Partitioners} = require('kafkajs');

//Get Environment Variables
const PROJECT_ID = process.env.PROJECT_ID;
const DEPLOYMENT_ID = process.env.DEPLOYMENT_ID;
const FRAMEWORK = process.env.FRAMEWORK;

//Initialize S3 Client
const s3Client = new S3Client({
    region: process.env.AWS_REGION,
    credentials: {
        accessKeyId: process.env.AWS_ACCESS_KEY_ID,
        secretAccessKey: process.env.AWS_SECRET_ACCESS_KEY
    }
});

//Initialize Kafka Client
const kafka = new Kafka({
    clientId: `build-server-${DEPLOYMENT_ID}`,
    brokers: [process.env.KAFKA_BROKER],
    ssl: {
        ca: [fs.readFileSync(path.join(__dirname, 'ca.pem'), 'utf-8')],
    },
    sasl: {
        username: process.env.KAFKA_USERNAME,
        password: process.env.KAFKA_PASSWORD,
        mechanism: 'plain',
    }
})

//Initialize Kafka Producer
const producer = kafka.producer({
    createPartitioner: Partitioners.DefaultPartitioner,
    allowAutoTopicCreation: false
});

//Create Logs to Send to Kafka Producer
const pubLog = async (log) => {
    await producer.send({
        topic: `${process.env.KAFKA_TOPIC}`,
        messages: [{
            key: `log`,
            value: JSON.stringify({
                PROJECT_ID,
                DEPLOYMENT_ID,
                log
            })
        }]
    })
}

//Generate separate build commands for each framework
const getBuildCommand = (framework) => {
    switch (framework) {
        case 'NEXT':
            return 'pnpm install && pnpm run build && pnpm dlx next export -o dist';
        case 'VITE':
        case 'REACT':
        case 'VUE':
        case 'ANGULAR':
        case 'SVELTE':
        default:
            return 'pnpm install && pnpm run build';
    }
}

// Connect to Kafka Producer
const connectToKafka = async () => {
    await producer.connect();
    console.log('Connected to Kafka');
    console.log(`Starting build process...`);
    await pubLog('Starting build process...');
};

// Prepare the output directory and create it if it doesn't exist
const prepareOutputDirectory = () => {
    const outDirPath = path.join(__dirname, 'output');
    if (!fs.existsSync(outDirPath)) {
        console.log(`Creating output directory... at ${outDirPath}`);
        fs.mkdirSync(outDirPath, { recursive: true });
    }
    return outDirPath;
};

// Execute the build process
const executeBuild = async (outDirPath) => {
    return new Promise( async (resolve, reject) => {
        const buildCommand = getBuildCommand(FRAMEWORK);
        console.log(`Using build command for ${FRAMEWORK}: ${buildCommand}`);
        await pubLog(`Using build command for ${FRAMEWORK}: ${buildCommand}`);

        const p = exec(`cd ${outDirPath} && ${buildCommand}`);

        p.stdout.on('data', async (data) => {
            console.log(data.toString());
            await pubLog(data.toString());
        });

        p.stderr.on('data', async (data) => {
            console.error(data.toString());
            await pubLog(data.toString());
        });

        p.on('close', async (exitCode) => {
            console.log(`Build process completed with exit code ${exitCode}`);
            await pubLog(`Build process completed with exit code ${exitCode}`);

            if (exitCode !== 0) {
                reject(new Error(`Build failed with exit code ${exitCode}`));
                return;
            }

            resolve(exitCode);
        });
    });
};

// Verify build output directory exists
const verifyBuildOutput = (outDirPath) => {
    const distDir = path.join(outDirPath, 'dist');
    if (!fs.existsSync(distDir)) {
        throw new Error('Build output directory not found');
    }
    return distDir;
};

// Upload build output to S3
const uploadToS3 = async (distDir) => {
    const distFiles = fs.readdirSync(distDir, { recursive: true });
    console.log(`Starting to upload ${distFiles.length} files to S3`);
    await pubLog(`Starting to upload ${distFiles.length} files to S3`);

    for (const file of distFiles) {
        const filePath = path.join(distDir, file);
        if (fs.lstatSync(filePath).isDirectory()) continue;

        console.log(`Uploading ${file} to S3`);
        await pubLog(`Uploading ${file} to S3`);

        const command = new PutObjectCommand({
            Bucket: process.env.AWS_BUCKET_NAME,
            Key: `__output/${DEPLOYMENT_ID}/${file}`,
            Body: fs.createReadStream(filePath),
            ContentType: mime.lookup(filePath)
        });

        await s3Client.send(command);
        console.log(`Uploaded ${file} to S3`);
        await pubLog(`Uploaded ${file} to S3`);
    }
};

const main = async () => {
    try {
        await connectToKafka();
        const outDirPath = prepareOutputDirectory();
        await executeBuild(outDirPath);
        const distDir = verifyBuildOutput(outDirPath);
        await uploadToS3(distDir);

        console.log('Build output uploaded to S3 successfully');
        await pubLog('Build output uploaded to S3 successfully');
        process.exit(0);
    } catch (error) {
        console.error('Error:', error.message);
        await pubLog(`Error: ${error.message}`);
        process.exit(1);
    }
};

main().catch(err => {
    console.error('Unhandled error in main function:', err);
    process.exit(1);
});