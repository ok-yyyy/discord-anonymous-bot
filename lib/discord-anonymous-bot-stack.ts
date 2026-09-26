import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib/core';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import { Construct } from 'constructs';

/** Discordから受け取る秘匿値。 */
export interface DiscordSecrets {
  /** InteractionのEd25519署名検証に使う公開鍵 (16進文字列)。 */
  readonly publicKey: string;
}

export interface DiscordAnonymousBotStackProps extends cdk.StackProps {
  readonly discord: DiscordSecrets;
}

/**
 * Discordは3秒以内の応答を求めるため、受信側はこの時間で打ち切る。
 * これより長くしてもDiscordが先に諦めるだけで、課金が増え異常に気付きにくくなる。
 */
const INTERACTION_TIMEOUT = cdk.Duration.seconds(3);

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

    const interaction = this.goFunction('InteractionFunction', {
      target: 'interaction',
      timeout: INTERACTION_TIMEOUT,
      environment: {
        DISCORD_PUBLIC_KEY: props.discord.publicKey,
      },
    });

    // Discordは署名で認証するため、URL自体の認証は掛けない。
    const url = interaction.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    new cdk.CfnOutput(this, 'InteractionsEndpointUrl', {
      value: url.url,
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
      // provided.al2023 は bootstrap という名前の実行ファイルを起動する。
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
