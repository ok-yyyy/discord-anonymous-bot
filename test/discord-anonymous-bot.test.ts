import * as fs from 'node:fs';
import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib/core';
import { Match, Template } from 'aws-cdk-lib/assertions';
import { DiscordAnonymousBotStack } from '../lib/discord-anonymous-bot-stack';

/**
 * Code.fromAssetは実在するディレクトリを要求する。
 * ビルド済みかどうかでテストの結果が変わらないよう、空でも中身を用意しておく。
 */
function ensureBuildDir(): void {
  const dir = path.join(__dirname, '..', 'lambda', 'build', 'interaction');
  fs.mkdirSync(dir, { recursive: true });

  const bootstrap = path.join(dir, 'bootstrap');
  if (!fs.existsSync(bootstrap)) {
    fs.writeFileSync(bootstrap, '');
  }
}

function synth(): Template {
  ensureBuildDir();

  const app = new cdk.App();
  const stack = new DiscordAnonymousBotStack(app, 'TestStack', {
    discord: { publicKey: 'test-public-key' },
  });
  return Template.fromStack(stack);
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

  // Discordは3秒で諦めるため、長くしても待たされるだけになる。
  test('受信側のタイムアウトは3秒', () => {
    template.hasResourceProperties('AWS::Lambda::Function', {
      Timeout: 3,
    });
  });

  test('署名検証に使う公開鍵を渡す', () => {
    template.hasResourceProperties('AWS::Lambda::Function', {
      Environment: {
        Variables: Match.objectLike({ DISCORD_PUBLIC_KEY: 'test-public-key' }),
      },
    });
  });

  // 認証はDiscordの署名で行うため、URL自体には掛けない。
  test('Function URLは認証なしで公開する', () => {
    template.hasResourceProperties('AWS::Lambda::Url', {
      AuthType: 'NONE',
    });
  });

  test('ログの保持期間を指定する', () => {
    template.allResourcesProperties('AWS::Logs::LogGroup', {
      RetentionInDays: 30,
    });
  });

  // Interactions Endpoint URLに設定する値なので、出力しないと調べる手間が増える。
  test('Function URLを出力する', () => {
    const outputs = template.findOutputs('InteractionsEndpointUrl');
    expect(Object.keys(outputs)).toHaveLength(1);
  });
});
