import * as fs from 'node:fs';
import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib/core';
import { Match, Template } from 'aws-cdk-lib/assertions';
import { DiscordAnonymousBotStack } from '../lib/discord-anonymous-bot-stack';

/** Lambdaにデプロイする関数。 */
const TARGETS = ['interaction', 'worker'];

/**
 * Code.fromAssetは実在するディレクトリを要求する。
 * ビルド済みかどうかでテストの結果が変わらないよう、空でも中身を用意しておく。
 */
function ensureBuildDir(): void {
  for (const target of TARGETS) {
    const dir = path.join(__dirname, '..', 'lambda', 'build', target);
    fs.mkdirSync(dir, { recursive: true });

    const bootstrap = path.join(dir, 'bootstrap');
    if (!fs.existsSync(bootstrap)) {
      fs.writeFileSync(bootstrap, '');
    }
  }
}

function synth(): Template {
  ensureBuildDir();

  const app = new cdk.App();
  const stack = new DiscordAnonymousBotStack(app, 'TestStack', {
    discord: {
      publicKey: 'test-public-key',
      botToken: 'test-bot-token',
      applicationId: 'test-application-id',
    },
    anonymousSalt: 'test-salt',
  });
  return Template.fromStack(stack);
}

/** 指定したタイムアウトを持つLambdaの環境変数を返す。 */
function environmentOf(template: Template, timeout: number): Record<string, string> {
  const functions = template.findResources('AWS::Lambda::Function', {
    Properties: { Timeout: timeout },
  });

  const found = Object.values(functions);
  expect(found).toHaveLength(1);
  return found[0].Properties.Environment.Variables;
}

describe('DiscordAnonymousBotStack', () => {
  const template = synth();

  test('Lambdaはarm64のprovided.al2023で動く', () => {
    template.allResourcesProperties('AWS::Lambda::Function', {
      Runtime: 'provided.al2023',
      Architectures: ['arm64'],
      Handler: 'bootstrap',
    });
  });

  test('受信側とworkerの2つのLambdaを作る', () => {
    template.resourceCountIs('AWS::Lambda::Function', 2);
  });

  // Discordは3秒で諦めるため、長くしても待たされるだけになる。
  test('受信側のタイムアウトは3秒', () => {
    template.hasResourceProperties('AWS::Lambda::Function', {
      Timeout: 3,
    });
  });

  // 認証はDiscordの署名で行うため、URL自体には掛けない。
  test('Function URLは認証なしで公開する', () => {
    template.hasResourceProperties('AWS::Lambda::Url', {
      AuthType: 'NONE',
    });
  });

  // 可視性タイムアウトがworkerのタイムアウトより短いと、処理中に再配信される。
  test('可視性タイムアウトはworkerのタイムアウトの6倍', () => {
    template.hasResourceProperties('AWS::SQS::Queue', {
      FifoQueue: true,
      VisibilityTimeout: 180,
    });
  });

  // 再配信されるとDiscordへの投稿が重複しうる。1回でDLQに送る。
  test('失敗したメッセージは再配信せずDLQに送る', () => {
    template.hasResourceProperties('AWS::SQS::Queue', {
      RedrivePolicy: Match.objectLike({ maxReceiveCount: 1 }),
    });
  });

  test('DLQもFIFOにする', () => {
    template.resourceCountIs('AWS::SQS::Queue', 2);
    template.allResourcesProperties('AWS::SQS::Queue', {
      FifoQueue: true,
    });
  });

  test('SQSからは1件ずつ取り出す', () => {
    template.hasResourceProperties('AWS::Lambda::EventSourceMapping', {
      BatchSize: 1,
    });
  });

  test('受信側はキューのURLを知っている', () => {
    expect(environmentOf(template, 3)).toHaveProperty('QUEUE_URL');
  });

  // 受信側は署名検証とenqueueしかしない。漏れる面を狭くしておく。
  test('受信側のLambdaには秘匿値を渡さない', () => {
    const variables = environmentOf(template, 3);

    expect(variables).not.toHaveProperty('DISCORD_BOT_TOKEN');
    expect(variables).not.toHaveProperty('ANONYMOUS_SALT');
  });

  test('workerは投稿に必要な値を持つ', () => {
    const variables = environmentOf(template, 30);

    expect(variables).toMatchObject({
      DISCORD_BOT_TOKEN: 'test-bot-token',
      DISCORD_APPLICATION_ID: 'test-application-id',
      ANONYMOUS_SALT: 'test-salt',
    });
  });

  // どのリソースがこのBotのものかを請求やコンソールから辿れるようにする。
  // タグに対応しないリソース (Lambda::Url など) はCDKが読み飛ばす。
  test.each([
    'AWS::Lambda::Function',
    'AWS::SQS::Queue',
    'AWS::Logs::LogGroup',
    'AWS::IAM::Role',
  ])('%s にアプリ名のタグが付く', (type) => {
    template.allResourcesProperties(type, {
      Tags: Match.arrayWith([{ Key: 'app', Value: 'discord-anonymous-bot' }]),
    });
  });

  test('ログの保持期間を指定する', () => {
    template.allResourcesProperties('AWS::Logs::LogGroup', {
      RetentionInDays: 30,
    });
  });

  // Interactions Endpoint URLに設定する値なので、出力しないと調べる手間が増える。
  test('Function URLを出力する', () => {
    expect(Object.keys(template.findOutputs('InteractionsEndpointUrl'))).toHaveLength(1);
  });
});
