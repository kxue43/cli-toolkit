import express from 'express';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

const fileDir = dirname(fileURLToPath(import.meta.url));

const app = express();
app.use(express.static(join(fileDir, '_build', 'html')));

const server = app.listen(8080, () => {
  const { port } = server.address();
  console.log('Server started at http://localhost:%s', port);
});
