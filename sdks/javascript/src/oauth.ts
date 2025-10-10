import * as http from 'http';
import { URL } from 'url';
import axios from 'axios';
import open from 'open';

export type OAuthProvider = 'google' | 'microsoft' | 'okta';

export interface OAuthConfig {
  provider: OAuthProvider;
  clientId: string;
  clientSecret: string;
  redirectUrl: string;
  scopes: string[];
}

export interface OAuthToken {
  accessToken: string;
  refreshToken?: string;
  tokenType: string;
  expiresIn?: number;
}

const OAUTH_ENDPOINTS: Record<OAuthProvider, { authUrl: string; tokenUrl: string }> = {
  google: {
    authUrl: 'https://accounts.google.com/o/oauth2/v2/auth',
    tokenUrl: 'https://oauth2.googleapis.com/token',
  },
  microsoft: {
    authUrl: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
    tokenUrl: 'https://login.microsoftonline.com/common/oauth2/v2.0/token',
  },
  okta: {
    authUrl: process.env.OKTA_DOMAIN ? `https://${process.env.OKTA_DOMAIN}/oauth2/v1/authorize` : '',
    tokenUrl: process.env.OKTA_DOMAIN ? `https://${process.env.OKTA_DOMAIN}/oauth2/v1/token` : '',
  },
};

/**
 * Generate OAuth authorization URL
 */
export function getAuthorizationUrl(provider: OAuthProvider, clientId: string, redirectUrl: string): string {
  const endpoints = OAUTH_ENDPOINTS[provider];
  const scopes = ['openid', 'profile', 'email'];
  const state = generateRandomState();

  const params = new URLSearchParams({
    client_id: clientId,
    redirect_uri: redirectUrl,
    response_type: 'code',
    scope: scopes.join(' '),
    state,
  });

  return `${endpoints.authUrl}?${params.toString()}`;
}

/**
 * Start local HTTP server to receive OAuth callback
 */
export function startCallbackServer(port: number = 8080): Promise<string> {
  return new Promise((resolve, reject) => {
    const server = http.createServer((req, res) => {
      const url = new URL(req.url!, `http://localhost:${port}`);

      if (url.pathname === '/callback') {
        const code = url.searchParams.get('code');
        const error = url.searchParams.get('error');

        if (error) {
          res.writeHead(400, { 'Content-Type': 'text/html' });
          res.end(`<h1>❌ Authorization Failed</h1><p>${error}</p>`);
          reject(new Error(`OAuth error: ${error}`));
        } else if (code) {
          res.writeHead(200, { 'Content-Type': 'text/html' });
          res.end('<h1>✅ Authorization Successful!</h1><p>You can close this window.</p>');
          resolve(code);
        }

        server.close();
      }
    });

    server.listen(port, () => {
      console.log(`🔐 OAuth callback server listening on http://localhost:${port}/callback`);
    });

    // Timeout after 5 minutes
    setTimeout(() => {
      server.close();
      reject(new Error('OAuth flow timeout'));
    }, 5 * 60 * 1000);
  });
}

/**
 * Exchange authorization code for access token
 */
export async function exchangeCodeForToken(
  provider: OAuthProvider,
  code: string,
  clientId: string,
  clientSecret: string,
  redirectUrl: string
): Promise<OAuthToken> {
  const endpoints = OAUTH_ENDPOINTS[provider];

  const params = new URLSearchParams({
    grant_type: 'authorization_code',
    code,
    client_id: clientId,
    client_secret: clientSecret,
    redirect_uri: redirectUrl,
  });

  const response = await axios.post(endpoints.tokenUrl, params.toString(), {
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  });

  return {
    accessToken: response.data.access_token,
    refreshToken: response.data.refresh_token,
    tokenType: response.data.token_type,
    expiresIn: response.data.expires_in,
  };
}

/**
 * Open browser for OAuth authorization
 */
export async function openBrowser(url: string): Promise<void> {
  console.log(`🔐 Opening browser for authorization:\n${url}`);
  await open(url);
}

/**
 * Generate random state for CSRF protection
 */
function generateRandomState(): string {
  return Math.random().toString(36).substring(2, 15) +
         Math.random().toString(36).substring(2, 15);
}
