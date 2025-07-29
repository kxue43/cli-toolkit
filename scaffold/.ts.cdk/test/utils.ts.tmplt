import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { SynthUtils } from '@aws-cdk/assert';
import { Stack } from 'aws-cdk-lib';
import { Template } from 'aws-cdk-lib/assertions';
import { expect } from 'vitest';
import { stringify } from 'yaml';

export function stripAssetHashes(stack: Stack): Template {
  let template = SynthUtils.toCloudFormation(stack);

  let key: string;
  let resource: any; // eslint-disable-line @typescript-eslint/no-explicit-any
  const lambdaCurrentVersionIDs: string[] = [];

  for ([key, resource] of Object.entries(template.Resources!)) {
    if (resource?.Properties?.Code?.S3Key) {
      resource.Properties.Code.S3Key = 'PLACEHOLDER_S3KEY';
    }
    if (resource?.Properties?.Code?.S3Bucket) {
      resource.Properties.Code.S3Bucket = 'PLACEHOLDER_S3BUCKET';
    }
    if (resource.Type === 'AWS::Lambda::Version') {
      lambdaCurrentVersionIDs.push(key);
    }
  }
  if (lambdaCurrentVersionIDs.length > 0) {
    let templateJson = JSON.stringify(template);
    lambdaCurrentVersionIDs.forEach((versionID, index) => {
      const prefix = versionID.slice(0, -32);
      const hashSuffix = versionID.slice(-32);
      // Lookbehind assertions are NOT capturing groups.
      templateJson = templateJson.replaceAll(new RegExp(`(?<=${prefix})(${hashSuffix})`, 'g'), `NormalizedHash${index}`);
    });
    template = JSON.parse(templateJson);
  }

  return Template.fromJSON(template);
}

export function snapshotAssetfulStack(stack: Stack) {
  const template = stripAssetHashes(stack);
  expect(stringify(template.toJSON(), { sortMapEntries: true })).toMatchSnapshot();
}

export function snapshotAssetlessStack(stack: Stack) {
  const template = Template.fromStack(stack);
  expect(stringify(template.toJSON(), { sortMapEntries: true })).toMatchSnapshot();
}

export function getTempDir(): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'cdk-test-'));
}

export function createFakeZipAsset(dir: string): string {
  dir = path.resolve(dir);
  const filePath = path.join(dir, 'asset.zip');
  fs.writeFileSync(filePath, 'placeholder');
  return filePath;
}

export function rmDir(dir: string) {
  fs.rmSync(dir, { recursive: true, force: true });
}

export function createFakeTarball(dir: string): string {
  dir = path.resolve(dir);
  const filePath = path.join(dir, 'asset.tar');
  fs.writeFileSync(filePath, 'placeholder');
  return filePath;
}

function equal(first: string[], second: string[]) {
  if (first.length !== second.length) {
    return false;
  }
  for (let i = 0; i < first.length; i++) {
    if (first[i] !== second[i]) {
      return false;
    }
  }
  return true;
}

expect.extend({
  toDeepEqualAny(received: string[], ...arrays: string[][]) {
    let pass = false;
    let message =
      `Expected ${JSON.stringify(received)} to equal one of ` + `${arrays.map(a => JSON.stringify(a)).join(', ')}, but found none.`;
    for (const array of arrays) {
      if (equal(received, array)) {
        pass = true;
        message = '';
        break;
      }
    }
    return {
      message: () => message,
      pass,
    };
  },
});
