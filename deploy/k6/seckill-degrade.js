import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const baseUrl = __ENV.BASE_URL || 'http://product:8000';
const seckillPath = __ENV.SECKILL_PATH || '/api/seckill';
const statusPath = __ENV.STATUS_PATH || '/api/seckill/status';
const userCount = Number(__ENV.USER_COUNT || 50000);
const startUserID = Number(__ENV.START_USER_ID || 1);
const sleepMs = Number(__ENV.SLEEP_MS || 0);
const statusTimeoutMs = Number(__ENV.STATUS_TIMEOUT_MS || 5000);
const pollIntervalMs = Number(__ENV.POLL_INTERVAL_MS || 200);

const queuedCount = new Counter('seckill_queued_total');
const successCount = new Counter('seckill_success_total');
const degradedCount = new Counter('seckill_degraded_total');
const soldOutCount = new Counter('seckill_sold_out_total');
const failedCount = new Counter('seckill_failed_total');
const rateLimitedCount = new Counter('seckill_rate_limited_total');
const statusPollTimeoutCount = new Counter('seckill_status_poll_timeout_total');
const statusRequestErrorCount = new Counter('seckill_status_request_error_total');
const endToEndDuration = new Trend('seckill_end_to_end_ms');

function buildStages() {
  if (__ENV.K6_STAGES) {
    return __ENV.K6_STAGES.split(',').map((entry) => {
      const parts = entry.split(':');
      if (parts.length !== 2 || !parts[0] || Number.isNaN(Number(parts[1]))) {
        throw new Error(`invalid K6_STAGES entry: "${entry}", expected format like 30s:200`);
      }
      return { duration: parts[0], target: Number(parts[1]) };
    });
  }

  return [
    { duration: '15s', target: 100 },
    { duration: '45s', target: 400 },
    { duration: '30s', target: 800 },
    { duration: '15s', target: 0 },
  ];
}

export const options = {
  stages: buildStages(),
  thresholds: {
    http_req_failed: ['rate<0.30'],
    http_req_duration: ['p(95)<1500'],
  },
};

function nextUserID() {
  return ((__VU * 1000000 + __ITER) % userCount) + startUserID;
}

function parseJSON(body) {
  if (!body) {
    return null;
  }
  try {
    return JSON.parse(body);
  } catch (_) {
    return null;
  }
}

function normalizeResult(value) {
  if (typeof value === 'string') {
    return value.toUpperCase();
  }
  if (value === 0) {
    return 'SUCCESS';
  }
  if (value === 1) {
    return 'FAILURE';
  }
  if (value === 2) {
    return 'QUEUED';
  }
  return 'UNKNOWN';
}

function normalizeStatus(value) {
  if (typeof value === 'string') {
    return value.toUpperCase();
  }

  switch (value) {
    case 0:
      return 'SECKILL_STATUS_WAITING';
    case 1:
      return 'SECKILL_STATUS_PROCESSING';
    case 2:
      return 'SECKILL_STATUS_SUCCESS';
    case 3:
      return 'SECKILL_STATUS_SOLD_OUT';
    case 4:
      return 'SECKILL_STATUS_DEGRADED';
    case 5:
      return 'SECKILL_STATUS_FAILED';
    default:
      return 'SECKILL_STATUS_UNKNOWN';
  }
}

function pollStatus(requestID, startedAt) {
  const deadline = Date.now() + statusTimeoutMs;

  while (Date.now() < deadline) {
    const res = http.post(`${baseUrl}${statusPath}`, JSON.stringify({ requestID }), {
      headers: { 'Content-Type': 'application/json' },
      timeout: __ENV.HTTP_TIMEOUT || '5s',
    });

    if (res.status >= 500) {
      statusRequestErrorCount.add(1);
      return;
    }

    if (res.status === 404) {
      sleep(pollIntervalMs / 1000);
      continue;
    }

    const body = parseJSON(res.body);
    const status = normalizeStatus(body && body.status);

    if (status === 'SECKILL_STATUS_WAITING' || status === 'SECKILL_STATUS_PROCESSING') {
      sleep(pollIntervalMs / 1000);
      continue;
    }

    const durationMs = Date.now() - startedAt;
    endToEndDuration.add(durationMs);

    if (status === 'SECKILL_STATUS_SUCCESS') {
      successCount.add(1);
      return;
    }
    if (status === 'SECKILL_STATUS_DEGRADED') {
      degradedCount.add(1);
      return;
    }
    if (status === 'SECKILL_STATUS_SOLD_OUT') {
      soldOutCount.add(1);
      return;
    }

    failedCount.add(1);
    return;
  }

  statusPollTimeoutCount.add(1);
  endToEndDuration.add(Date.now() - startedAt);
}

export default function () {
  const startedAt = Date.now();
  const payload = JSON.stringify({ userID: nextUserID() });
  const res = http.post(`${baseUrl}${seckillPath}`, payload, {
    headers: { 'Content-Type': 'application/json' },
    timeout: __ENV.HTTP_TIMEOUT || '5s',
  });

  check(res, {
    'seckill status < 500': (r) => r.status < 500,
    'seckill has body': (r) => r.body && r.body.length > 0,
  });

  if (res.status === 429) {
    rateLimitedCount.add(1);
  } else {
    const body = parseJSON(res.body);
    const result = normalizeResult(body && body.res);
    const requestID = body && body.requestID;

    if (result === 'QUEUED' && requestID) {
      queuedCount.add(1);
      pollStatus(requestID, startedAt);
    } else if (result === 'SUCCESS') {
      successCount.add(1);
      endToEndDuration.add(Date.now() - startedAt);
    } else {
      failedCount.add(1);
      endToEndDuration.add(Date.now() - startedAt);
    }
  }

  if (sleepMs > 0) {
    sleep(sleepMs / 1000);
  }
}
