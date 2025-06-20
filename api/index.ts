import express, {Request, Response} from 'express';
import { Kafka } from 'kafkajs';
import cors from 'cors';
import {generateSlug} from 'random-word-slugs';
import {PrismaClient} from './generated/prisma';
import {v4 as uuidv4} from 'uuid';
import {createClient} from '@clickhouse/client'
import path from 'path';
import fs from 'fs';
import {ECSClient, RunTaskCommand} from '@aws-sdk/client-ecs';
import {z} from 'zod';
import {withAccelerate} from '@prisma/extension-accelerate'
import dotenv from 'dotenv';

dotenv.config();

const app = express();
const port = process.env.PORT
app.use(express.json());
app.use(cors());

//Initialize Prisma Client
const prisma = new PrismaClient().$extends(withAccelerate());

//Initialize ClickHouse Client
const clickhouse = createClient({
    url: `${process.env.CLICKHOUSE_URL}`,
    database: `${process.env.CLICKHOUSE_DATABASE}`
});

//Initialize Kafka Client
const kafka = new Kafka({
    clientId: `api-server`,
    brokers: [`${process.env.KAFKA_BROKER}`],
    sasl: {
        username: `${process.env.KAFKA_USERNAME}`,
        password: `${process.env.KAFKA_PASSWORD}`,
        mechanism: 'plain'
    },
    ssl: {
        ca: fs.readFileSync(path.join(__dirname, 'ca.pem'), 'utf-8')
    }
});

//Initialize Kafka Consumer
const consumer = kafka.consumer({
    groupId: `api-server-logs-consumer`,
})

//Initialize ECS Client
const ecsClient = new ECSClient({
    region: `${process.env.AWS_REGION}`,
    credentials: {
        accessKeyId: `${process.env.AWS_ACCESS_KEY_ID}`,
        secretAccessKey: `${process.env.AWS_SECRET_ACCESS_KEY}`
    }
})

//Create POST at /project
app.post('/project', async (req: Request, res: Response) => {
    //Validate request body with zod
    const schema = z.object({
        name: z.string().min(1),
        gitRepoUrl: z.string().url(),
        framework: z.enum(['NEXT', 'REACT', 'VUE', 'ANGULAR', 'SVELTE', 'OTHER', 'VITE'])
    })
    const safeData = schema.safeParse(req.body);
    if (!safeData.success) {
        res.status(400).json({
            error: safeData.error.message
        });
        return;
    }
    const {name, gitRepoUrl, framework} = safeData.data;

    try {
        const project = await prisma.project.create({
            data: {
                name,
                gitRepoUrl,
                framework,
                slug: generateSlug()
            }
        })
        res.status(201).json({
            status: 'success',
            data: {
                project
            }
        });
    } catch (error) {
        console.error('Error creating project:', error);
        res.status(500).json({
            status: 'error',
            message: 'Failed to create project'
        });
    }
})

//Create POST at /deploy
app.post('/deploy', async (req: Request, res: Response) => {
    //Validate request body with zod for projectId
    const schema = z.object({
        projectId: z.string().uuid()
    })
    const safeData = schema.safeParse(req.body);
    if (!safeData.success) {
        res.status(400).json({
            error: safeData.error.message
        });
        return;
    }
    const {projectId} = safeData.data;

    //Check for the project in db
    const project = await prisma.project.findUnique({
        where: {
            id: projectId
        }
    })
    if (!project) {
        res.status(404).json({
            error: 'Project not found'
        });
        return;
    }

    //Create a new deployment with the associated project
    const deployment = await prisma.deployment.create({
        data: {
            project: {
                connect: {
                    id: projectId
                }
            },
            status: 'QUEUED'
        }
    });

    await prisma.project.update({
        where: { id: projectId },
        data: { latestDeploymentId: deployment.id }
      });

    //Initiate a ECS Task to build and store the project in S3
    const command = new RunTaskCommand({
        cluster: `${process.env.AWS_ECS_CLUSTER}`,
        taskDefinition: `${process.env.AWS_ECS_TASK_DEFINITION}`,
        launchType: 'FARGATE',
        count: 1,
        networkConfiguration: {
            awsvpcConfiguration: {
                subnets: `${process.env.AWS_ECS_SUBNETS}`.split(','),
                securityGroups: `${process.env.AWS_ECS_SECURITY_GROUPS}`.split(","),
                assignPublicIp: 'ENABLED'
            }
        },
        overrides: {
            containerOverrides: [
                {
                    name: `${process.env.AWS_ECS_CONTAINER_NAME}`,
                    environment: [ 
                        {
                            name: 'PROJECT_ID',
                            value: project.id
                        },
                        {
                            name: 'DEPLOYMENT_ID',
                            value: deployment.id
                        },
                        {
                            name: 'GIT_REPO_URL',
                            value: project.gitRepoUrl
                        },
                        {
                            name: 'FRAMEWORK',
                            value: project.framework
                        }
                    ]
                }
            ]
        }
    })

    
    //Run the ECS Task
    try {
        await ecsClient.send(command);
        res.status(202).json({
            status: 'success',
            data: {
                deploymentId: deployment.id,
                deploymentUrl: `http://${deployment.id}.yok.ninja/`
            }
        });
    } catch (error) {
        console.error('Error running ECS Task:', error);
        res.status(500).json({
            status: 'error',
            message: 'Failed to deploy project'
        });
        await prisma.deployment.update({
            where: {
                id: deployment.id
            },
            data: {
                status: 'FAILED'
            }
        })
    }
})

//Create GET at /logs/:deploymentId
app.get('/logs/:id', async (req: Request, res: Response) => {
    //Validate request params with zod for deploymentId
    const schema = z.object({
        id: z.string().uuid()
    })
    const safeData = schema.safeParse(req.params);
    if (!safeData.success) {
        res.status(400).json({ 
            error: safeData.error.message
        });
        return;
    }
    const {id} = safeData.data;

    //Get logs from ClickHouse
    const logs = await clickhouse.query({
        query: `SELECT event_id, deployment_id, log, timestamp from log_events where deployment_id = {deployment_id:String} ORDER BY timestamp ASC`,
        query_params: {
            deployment_id: id
        },
        format: 'JSONEachRow'
    });

    const rawLogs = await logs.json();

    //Send logs to client
    res.status(200).json({
        status: 'success',
        data: {
            logs: rawLogs
        }
    });

})

// Update deployment status based on log content
const updateDeploymentStatus = async (deploymentId: string, log: string) => {
    if (log.includes('Starting')) {
        await prisma.deployment.update({
            where: { id: deploymentId },
            data: { status: 'IN_PROGRESS' }
        });
        console.log(`Updated deployment ${deploymentId} to IN_PROGRESS`);
    } else if (log === 'UploadedBuild process completed with exit code 0') {
        await prisma.deployment.update({
            where: { id: deploymentId },
            data: { status: 'COMPLETED' }
        });
        
        console.log(`Updated deployment ${deploymentId} to COMPLETED`);
    } else if (log.includes('error')) {
        await prisma.deployment.update({
            where: { id: deploymentId },
            data: { status: 'FAILED' }
        });
        console.log(`Updated deployment ${deploymentId} to FAILED`);
    }
};

// Store log in ClickHouse
const storeLogInClickHouse = async (deploymentId: string, log: string) => {
    await clickhouse.insert({
        table: 'log_events',
        values: [{
            event_id: uuidv4(),
            deployment_id: deploymentId,
            log: log
        }],
        format: 'JSONEachRow'
    });
};

// Main function to consume logs from Kafka
const consumeLogs = async () => {
    await consumer.connect();
    await consumer.subscribe({
        topic: `${process.env.KAFKA_TOPIC}`,
        fromBeginning: true
    });

    await consumer.run({
        eachBatch: async ({ batch, heartbeat, commitOffsetsIfNecessary, resolveOffset }) => {
            const messages = batch.messages;
            
            // Process messages in batch
            for (const message of messages) {
                if (!message.value) continue;
                
                const msgString = message.value.toString();
                try {
                    const { DEPLOYMENT_ID, log } = JSON.parse(msgString);
                    console.log({ deploymentId: DEPLOYMENT_ID, log });
                    
                    // Process the message
                    await updateDeploymentStatus(DEPLOYMENT_ID, log);
                    await storeLogInClickHouse(DEPLOYMENT_ID, log);
                    
                    // Handle Kafka offset management
                    resolveOffset(message.offset);
                } catch (error) {
                    console.error('Error processing log message:', error);
                }
            }
            
            // Handle batch-level operations after processing all messages
            await commitOffsetsIfNecessary();
            await heartbeat();
        }
    });
};

// Create GET at /resolve/:slug
app.get('/resolve/:slug', async (req: Request, res: Response) => {
    const schema = z.object({
        slug: z.string().min(1)
    })
    const safeData = schema.safeParse(req.params);
    if (!safeData.success) {
        res.status(400).json({
            error: safeData.error.message
        });
        return;
    }
    const {slug} = safeData.data;

    // Find the project with the given slug
    try {
        const project = await prisma.project.findUnique({
            where: {
                slug: slug
            }
        })
        if (!project || !project.latestDeploymentId) {
            res.status(404).json({
                error: 'Project or latest deployment not found'
            });
            return;
        }
        console.log(`Resolved slug ${slug} to deployment ${project.latestDeploymentId}`);
        res.status(200).json({
            deploymentId : project.latestDeploymentId
        })
        
    } catch(error) {
        console.error('Error resolving slug:', error);
        res.status(500).json({
            error: 'Failed to resolve slug'
        });
    }
    
})

// Create GET for health check
app.get('/health', (req: Request, res: Response) => {
    res.status(200).json({
        status: 'ok'
    });
})

app.listen(port, async () => {
    console.log(`Server is running on port ${port}`);
    try {
        await consumeLogs();
        console.log('Successfully connected to Kafka and started consuming logs');
    } catch (error) {
        console.error('Failed to connect to Kafka producer:', error);
    }
})