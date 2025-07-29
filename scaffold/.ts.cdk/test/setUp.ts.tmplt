import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { App } from 'aws-cdk-lib';

const testDir = path.dirname(fileURLToPath(import.meta.url));

export const cdkDir = path.dirname(testDir);

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function getContextForTest(): { [key: string]: any } {
  let context: { [key: string]: any } = {}; // eslint-disable-line @typescript-eslint/no-explicit-any
  let filePath = path.join(cdkDir, 'cdk.json');
  if (!fs.existsSync(filePath)) {
    throw new Error('Could not find file `cdk.json`. It is mandatory for a CDK application.');
  }
  const cdkJson = JSON.parse(fs.readFileSync(filePath, { encoding: 'utf-8' }));
  if (Object.hasOwn(cdkJson, 'context')) {
    context = cdkJson.context;
  }
  filePath = path.join(cdkDir, 'cdk.context.json');
  if (!fs.existsSync(filePath)) {
    console.error(
      'Could not find cached CDK context value file `cdk.context.json`. CDK tests without ' +
        'cached context values could break for the lack of correct AWS credentials at runtime ' +
        'or be flaky.',
    );
  }
  return context;
}

export const app = new App({ context: getContextForTest() });
