#!/usr/bin/env node
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib/core';
import { DiscordAnonymousBotStack } from '../lib/discord-anonymous-bot-stack';

/** 秘匿値を置くファイル。コミットしない。 */
const ENV_FILE = path.join(__dirname, '..', '.env');

/** Lambdaに渡す必要がある環境変数。欠けていたらsynthの時点で止める。 */
const REQUIRED_KEYS = [
  'DISCORD_PUBLIC_KEY',
  'DISCORD_BOT_TOKEN',
  'DISCORD_APPLICATION_ID',
  'ANONYMOUS_SALT',
] as const;

const env = loadEnv();

const app = new cdk.App();
new DiscordAnonymousBotStack(app, 'DiscordAnonymousBotStack', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
  discord: {
    publicKey: env.DISCORD_PUBLIC_KEY,
    botToken: env.DISCORD_BOT_TOKEN,
    applicationId: env.DISCORD_APPLICATION_ID,
  },
  anonymousSalt: env.ANONYMOUS_SALT,
});

/**
 * .envと環境変数から設定を読む。loadEnvFileは既に設定されている環境変数を上書きしないので、そちらが優先される。
 *
 * 足りない値があればsynthの時点で止める。空のままdeployすると、署名検証が通らないLambdaが動き続けることになる。
 */
function loadEnv(): Record<(typeof REQUIRED_KEYS)[number], string> {
  if (fs.existsSync(ENV_FILE)) {
    process.loadEnvFile(ENV_FILE);
  }

  const missing = REQUIRED_KEYS.filter((key) => !process.env[key]?.trim());
  if (missing.length > 0) {
    throw new Error(
      `必要な環境変数が設定されていません: ${missing.join(', ')}\n` +
      `${ENV_FILE} に記載するか、環境変数として渡してください。`,
    );
  }

  return process.env as Record<(typeof REQUIRED_KEYS)[number], string>;
}
