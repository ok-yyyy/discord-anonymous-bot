import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib/core';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import { SqsEventSource } from 'aws-cdk-lib/aws-lambda-event-sources';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as sqs from 'aws-cdk-lib/aws-sqs';
import { Construct } from 'constructs';

/** Discordから受け取る秘匿値。 */
export interface DiscordSecrets {
  /** InteractionのEd25519署名検証に使う公開鍵 (16進文字列)。 */
  readonly publicKey: string;
  /** Webhookの取得・作成とパネル投稿に使う。 */
  readonly botToken: string;
  /** followupの送信先に使う。 */
  readonly applicationId: string;
}

export interface DiscordAnonymousBotStackProps extends cdk.StackProps {
  readonly discord: DiscordSecrets;
  /** 匿名IDの導出に使うsalt。 */
  readonly anonymousSalt: string;
}

/**
 * Discordは3秒以内の応答を求めるため、受信側はこの時間で打ち切る。
 * これより長くしてもDiscordが先に諦めるだけで、課金が増え異常に気付きにくくなる。
 */
const INTERACTION_TIMEOUT = cdk.Duration.seconds(3);

/** Discord APIを数回呼ぶ余裕を持たせた時間。 */
const WORKER_TIMEOUT = cdk.Duration.seconds(30);

/**
 * 可視性タイムアウトはLambdaのタイムアウトの6倍がAWSの推奨。
 * 短いと、処理中のメッセージが再配信されて二重に実行される。
 */
const QUEUE_VISIBILITY_TIMEOUT = cdk.Duration.seconds(WORKER_TIMEOUT.toSeconds() * 6);

/**
 * 128MBでも動くが、LambdaはCPUがメモリに比例する。
 * コールドスタートを短くする目的で256MBにしている。
 */
const MEMORY_SIZE = 256;

/**
 * ビルド成果物の置き場所。
 * `cd lambda && go run ./tools/build` が出力する。
 * */
const BUILD_DIR = path.join(__dirname, '..', 'lambda', 'build');

export class DiscordAnonymousBotStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: DiscordAnonymousBotStackProps) {
    super(scope, id, props);

    const queue = this.interactionQueue();

    const worker = this.goFunction('WorkerFunction', {
      target: 'worker',
      timeout: WORKER_TIMEOUT,
      environment: {
        DISCORD_BOT_TOKEN: props.discord.botToken,
        DISCORD_APPLICATION_ID: props.discord.applicationId,
        ANONYMOUS_SALT: props.anonymousSalt,
      },
    });

    // 1件ずつ処理する。部分失敗の扱いを考えずに済み、DLQに送られる単位も明確になる。
    worker.addEventSource(new SqsEventSource(queue, { batchSize: 1 }));

    const interaction = this.goFunction('InteractionFunction', {
      target: 'interaction',
      timeout: INTERACTION_TIMEOUT,
      // BotトークンとsaltはここでもRegistryを引くだけなので渡さない。
      environment: {
        DISCORD_PUBLIC_KEY: props.discord.publicKey,
        QUEUE_URL: queue.queueUrl,
      },
    });
    queue.grantSendMessages(interaction);

    // Discordは署名で認証するため、URL自体の認証は掛けない。
    const url = interaction.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    new cdk.CfnOutput(this, 'InteractionsEndpointUrl', {
      value: url.url,
    });
  }

  /**
   * 非同期処理を積むキューを作る。
   *
   * FIFOにして、MessageGroupIdにチャンネルIDを渡すことで同一チャンネルへの投稿順を保つ。
   * 重複排除はInteraction IDで行う。
   */
  private interactionQueue(): sqs.Queue {
    const deadLetterQueue = new sqs.Queue(this, 'DeadLetterQueue', {
      fifo: true,
      retentionPeriod: cdk.Duration.days(14),
    });

    return new sqs.Queue(this, 'InteractionQueue', {
      fifo: true,
      visibilityTimeout: QUEUE_VISIBILITY_TIMEOUT,
      deadLetterQueue: {
        queue: deadLetterQueue,
        // 再配信しない。
        // workerはDiscordへの投稿を1回だけ試みるため、やり直すと投稿が重複しうる。
        // 失敗はそのままDLQに送る。
        maxReceiveCount: 1,
      },
    });
  }

  /**
   * ビルド済みのGoバイナリからLambdaを作る。
   *
   * ビルドは`lambda/tools/build`に任せる。
   * ここでバンドルしないのは、ビルド方法を1か所にまとめ、Dockerを要らなくするため。
   */
  private goFunction(
    id: string,
    options: {
      target: string;
      timeout: cdk.Duration;
      environment: Record<string, string>;
    },
  ): lambda.Function {
    return new lambda.Function(this, id, {
      runtime: lambda.Runtime.PROVIDED_AL2023,
      architecture: lambda.Architecture.ARM_64,
      // provided.al2023はbootstrapという名前の実行ファイルを起動する。
      handler: 'bootstrap',
      code: lambda.Code.fromAsset(path.join(BUILD_DIR, options.target)),
      timeout: options.timeout,
      memorySize: MEMORY_SIZE,
      environment: options.environment,
      logGroup: new logs.LogGroup(this, `${id}Logs`, {
        retention: logs.RetentionDays.ONE_MONTH,
        removalPolicy: cdk.RemovalPolicy.DESTROY,
      }),
    });
  }
}
