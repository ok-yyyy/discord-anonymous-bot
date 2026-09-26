#!/usr/bin/env node
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib/core';
import { DiscordAnonymousBotStack } from '../lib/discord-anonymous-bot-stack';

/** 秘匿値を置くファイル。コミットしない。 */
const ENV_FILE = path.join(__dirname, '..', '.env');

/** Lambdaに渡す必要がある環境変数。欠けていたらsynthの時点で止める。 */
const REQUIRED_KEYS = ['DISCORD_PUBLIC_KEY'] as const;

const env = loadEnv();

const app = new cdk.App();
new DiscordAnonymousBotStack(app, 'DiscordAnonymousBotStack', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
  discord: {
    publicKey: env.DISCORD_PUBLIC_KEY,
  },
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

function parseEnvFile(file: string): Record<string, string> {
  if (!fs.existsSync(file)) {
    return {};
  }

  const values: Record<string, string> = {};
  for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const trimmed = line.trim();
    if (trimmed === '' || trimmed.startsWith('#')) {
      continue;
    }

    const separator = trimmed.indexOf('=');
    if (separator < 1) {
      continue;
    }

    const key = trimmed.slice(0, separator).trim();
    // 値が引用符で囲まれていれば外す。
    const value = trimmed
      .slice(separator + 1)
      .trim()
      .replace(/^(['"])(.*)\1$/, '$2');
    if (value !== '') {
      values[key] = value;
    }
  }
  return values;
}
