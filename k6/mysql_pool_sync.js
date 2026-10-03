import http from 'k6/http';
import { check } from 'k6';

const baseURL = (__ENV.BASE_URL || 'http://127.0.0.1:8081').replace(/\/$/, '');
const conversationID = __ENV.CONVERSATION_ID;
const token = __ENV.K6_TOKEN;

if (!conversationID || !token) {
  throw new Error('请通过 CONVERSATION_ID 和 K6_TOKEN 提供测试会话与访问令牌');
}

export const options = {
  scenarios: {
    sync: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '8s', target: 50 },
        { duration: '15s', target: 50 },
        { duration: '8s', target: 100 },
        { duration: '15s', target: 100 },
        { duration: '8s', target: 200 },
        { duration: '15s', target: 200 },
        { duration: '8s', target: 500 },
        { duration: '15s', target: 500 },
        { duration: '5s', target: 0 },
      ],
      gracefulRampDown: '2s',
    },
  },
  thresholds: {
    checks: ['rate>0.99'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const url = `${baseURL}/api/v1/messages/sync?conversationId=${encodeURIComponent(conversationID)}&afterSeq=0`;
  const response = http.get(url, {
    headers: { Authorization: `Bearer ${token}` },
    tags: { endpoint: 'messages_sync' },
  });
  check(response, { 'sync 返回 200': (res) => res.status === 200 });
}
