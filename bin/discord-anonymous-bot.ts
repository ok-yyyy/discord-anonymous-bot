#!/usr/bin/env node
import * as fs from 'node:fs';
import * as path from 'node:path';
import { parseEnv } from 'node:util';
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
 * .envと環境変数から設定を読む。環境変数が設定されていればそちらを優先する。
 *
 * 足りない値があればsynthの時点で止める。空のままdeployすると、署名検証が通らないLambdaが動き続けることになる。
 */
function loadEnv(): Record<(typeof REQUIRED_KEYS)[number], string> {
  const values = { ...parseEnvFile(ENV_FILE), ...fromProcess() };

  const missing = REQUIRED_KEYS.filter((key) => !values[key]);
  if (missing.length > 0) {
    throw new Error(
      `必要な環境変数が設定されていません: ${missing.join(', ')}\n` +
      `${ENV_FILE} に記載するか、環境変数として渡してください。`,
    );
  }

  return values as Record<(typeof REQUIRED_KEYS)[number], string>;
}

function fromProcess(): Record<string, string> {
  const values: Record<string, string> = {};
  for (const key of REQUIRED_KEYS) {
    const value = process.env[key]?.trim();
    if (value) {
      values[key] = value;
    }
  }
  return values;
}

/** .envを読む。ファイルが無ければ空を返す。 */
function parseEnvFile(file: string): Record<string, string> {
  if (!fs.existsSync(file)) {
    return {};
  }
  return parseEnv(fs.readFileSync(file, 'utf8')) as Record<string, string>;
}
