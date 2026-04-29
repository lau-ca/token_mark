const crypto = require('crypto');
const https = require('https');

const API_KEY = process.env.OPEN_API_KEY || 'your-api-key';
const API_SECRET = process.env.OPEN_API_SECRET || 'your-secret';
const BASE_URL = process.env.OPEN_API_BASE_URL || 'http://localhost:3000';

function sign(timestamp, nonce, method, path, body) {
  const msg = `${timestamp}\n${nonce}\n${method}\n${path}\n${body}`;
  return `hmac-sha256=${crypto.createHmac('sha256', API_SECRET).update(msg).digest('hex')}`;
}

function request(method, path, data) {
  return new Promise((resolve, reject) => {
    const timestamp = Math.floor(Date.now() / 1000);
    const nonce = crypto.randomUUID();
    const body = data ? JSON.stringify(data) : '';
    const url = new URL(BASE_URL + path);

    const req = https.request({
      hostname: url.hostname,
      port: url.port || 443,
      path: url.pathname + url.search,
      method,
      headers: {
        'Content-Type': 'application/json',
        'X-Api-Key': API_KEY,
        'X-Timestamp': timestamp.toString(),
        'X-Nonce': nonce,
        'X-Signature': sign(timestamp, nonce, method, url.pathname + url.search, body),
      },
    }, res => {
      let d = '';
      res.on('data', c => d += c);
      res.on('end', () => resolve(JSON.parse(d)));
    });
    req.on('error', reject);
    if (body) req.write(body);
    req.end();
  });
}

// 接口
const createToken = params => request('POST', '/api/token/open', params);
const updateTokenAmount = (key, amount) => request('POST', '/api/token/open/amount', { key, remain_amount: amount });

// 示例
(async () => {
  const r1 = await createToken({ name: 'test', remain_amount: 100 });
  console.log(r1);

  if (r1.success) {
    const r2 = await updateTokenAmount(r1.key, 200);
    console.log(r2);
  }
})();
