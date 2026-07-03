import * as http from 'http';
import * as https from 'https';
import { URL } from 'url';

/** Maximum request/response body size (10 MB) */
const MAX_BODY_BYTES = 10 * 1024 * 1024;

/**
 * Buffer the full request body from an IncomingMessage.
 * Rejects with 413 if body exceeds MAX_BODY_BYTES.
 */
export function bufferBody(req: http.IncomingMessage): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let totalBytes = 0;
    req.on('data', (chunk: Buffer) => {
      totalBytes += chunk.length;
      if (totalBytes > MAX_BODY_BYTES) {
        req.destroy();
        reject(new Error('Request body too large'));
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

/**
 * Forward an HTTP request to an upstream target and pipe the response back.
 * Returns the upstream response and its body buffer (for inspection).
 */
export function forwardRequest(
  upstream: string,
  req: http.IncomingMessage,
  body: Buffer,
  originalPath: string,
): Promise<{ response: http.IncomingMessage; body: Buffer }> {
  return new Promise((resolve, reject) => {
    const target = new URL(originalPath, upstream);
    const isHttps = target.protocol === 'https:';
    const mod = isHttps ? https : http;

    const options: http.RequestOptions = {
      hostname: target.hostname,
      port: target.port || (isHttps ? 443 : 80),
      path: target.pathname + target.search,
      method: req.method,
      headers: copyHeaders(req.headers, target.hostname),
      timeout: 30000,
    };

    const proxyReq = mod.request(options, (proxyRes) => {
      const chunks: Buffer[] = [];
      let totalBytes = 0;
      proxyRes.on('data', (chunk: Buffer) => {
        totalBytes += chunk.length;
        if (totalBytes > MAX_BODY_BYTES) {
          proxyRes.destroy();
          reject(new Error('Upstream response too large'));
          return;
        }
        chunks.push(chunk);
      });
      proxyRes.on('end', () => {
        resolve({
          response: proxyRes,
          body: Buffer.concat(chunks),
        });
      });
      proxyRes.on('error', reject);
    });

    proxyReq.on('error', reject);
    proxyReq.on('timeout', () => {
      proxyReq.destroy();
      reject(new Error('Upstream request timed out'));
    });

    if (body.length > 0) {
      proxyReq.write(body);
    }
    proxyReq.end();
  });
}

/**
 * Copy headers from source to a plain object, updating Host header.
 */
function copyHeaders(
  source: http.IncomingHttpHeaders,
  targetHost: string,
): http.OutgoingHttpHeaders {
  const headers: http.OutgoingHttpHeaders = {};

  for (const [key, value] of Object.entries(source)) {
    if (key.toLowerCase() === 'host') {
      headers[key] = targetHost;
    } else if (key.toLowerCase() !== 'connection') {
      headers[key] = value;
    }
  }

  return headers;
}

/**
 * Write headers and body to the client response.
 */
export function sendResponse(
  res: http.ServerResponse,
  statusCode: number,
  headers: http.IncomingHttpHeaders,
  body: Buffer,
): void {
  for (const [key, value] of Object.entries(headers)) {
    if (value !== undefined && key.toLowerCase() !== 'transfer-encoding') {
      res.setHeader(key, value);
    }
  }
  res.writeHead(statusCode);
  res.end(body);
}

/**
 * Send an error response as JSON.
 */
export function sendError(
  res: http.ServerResponse,
  statusCode: number,
  message: string,
): void {
  const body = JSON.stringify({ error: message });
  res.writeHead(statusCode, {
    'content-type': 'application/json',
    'content-length': Buffer.byteLength(body),
  });
  res.end(body);
}
