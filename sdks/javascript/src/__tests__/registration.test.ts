/**
 * Tests for secure() and registerAgent() functions
 *
 * The secure() function is an alias for registerAgent() that provides
 * a more intuitive API for enterprise users. These tests verify that
 * both functions behave identically.
 */

import axios from 'axios';
import { secure, registerAgent, secureWithOAuth, registerAgentWithOAuth } from '../registration';
import * as credentials from '../credentials';

// Mock axios
jest.mock('axios');
const mockedAxios = axios as jest.Mocked<typeof axios>;

// Mock credentials module
jest.mock('../credentials', () => ({
  storeCredentials: jest.fn(),
  storeOAuthToken: jest.fn(),
  loadCredentials: jest.fn(),
  hasCredentials: jest.fn(),
  clearCredentials: jest.fn(),
}));

describe('secure() alias', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('secure() is defined', () => {
    expect(secure).toBeDefined();
    expect(typeof secure).toBe('function');
  });

  test('secure() registers agent successfully', async () => {
    // Mock successful registration response
    const mockResponse = {
      data: {
        id: 'test-agent-id',
        name: 'test-agent',
        apiKey: 'test-api-key',
        publicKey: 'test-public-key',
      },
    };

    mockedAxios.post.mockResolvedValueOnce(mockResponse);

    // Call secure()
    const result = await secure('https://aim.example.com', {
      name: 'test-agent',
      type: 'ai_agent',
    });

    // Verify result
    expect(result.id).toBe('test-agent-id');
    expect(result.name).toBe('test-agent');
    expect(result.apiKey).toBe('test-api-key');

    // Verify API call was made
    expect(mockedAxios.post).toHaveBeenCalledTimes(1);
    expect(mockedAxios.post).toHaveBeenCalledWith(
      'https://aim.example.com/api/v1/agents/register',
      expect.objectContaining({
        name: 'test-agent',
        type: 'ai_agent',
      }),
      expect.any(Object)
    );

    // Verify credentials were stored
    expect(credentials.storeCredentials).toHaveBeenCalledWith(
      expect.objectContaining({
        agentId: 'test-agent-id',
        apiKey: 'test-api-key',
      })
    );
  });

  test('secure() and registerAgent() behave identically', async () => {
    // Mock successful registration response
    const mockResponse = {
      data: {
        id: 'test-agent-id',
        name: 'test-agent',
        apiKey: 'test-api-key',
        publicKey: 'test-public-key',
      },
    };

    mockedAxios.post.mockResolvedValue(mockResponse);

    // Call both functions with same arguments
    const result1 = await secure('https://aim.example.com', {
      name: 'test-agent-1',
      type: 'ai_agent',
    });

    const result2 = await registerAgent('https://aim.example.com', {
      name: 'test-agent-2',
      type: 'ai_agent',
    });

    // Both should return same structure
    expect(result1).toEqual(expect.objectContaining({
      id: expect.any(String),
      name: expect.any(String),
      apiKey: expect.any(String),
      publicKey: expect.any(String),
    }));

    expect(result2).toEqual(expect.objectContaining({
      id: expect.any(String),
      name: expect.any(String),
      apiKey: expect.any(String),
      publicKey: expect.any(String),
    }));

    // Both should call API
    expect(mockedAxios.post).toHaveBeenCalledTimes(2);

    // Both should store credentials
    expect(credentials.storeCredentials).toHaveBeenCalledTimes(2);
  });

  test('secure() uses default type when not specified', async () => {
    // Mock successful registration response
    const mockResponse = {
      data: {
        id: 'test-agent-id',
        name: 'test-agent',
        apiKey: 'test-api-key',
        publicKey: 'test-public-key',
      },
    };

    mockedAxios.post.mockResolvedValueOnce(mockResponse);

    // Call without specifying type
    await secure('https://aim.example.com', {
      name: 'test-agent',
    });

    // Verify default type "ai_agent" was used
    expect(mockedAxios.post).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        type: 'ai_agent',
      }),
      expect.any(Object)
    );
  });

  test('secure() respects custom agent type', async () => {
    // Mock successful registration response
    const mockResponse = {
      data: {
        id: 'test-agent-id',
        name: 'test-agent',
        apiKey: 'test-api-key',
        publicKey: 'test-public-key',
      },
    };

    mockedAxios.post.mockResolvedValueOnce(mockResponse);

    // Call with custom type
    await secure('https://aim.example.com', {
      name: 'test-agent',
      type: 'mcp_server',
    });

    // Verify custom type was used
    expect(mockedAxios.post).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        type: 'mcp_server',
      }),
      expect.any(Object)
    );
  });

  test('secure() handles errors', async () => {
    // Mock API error
    const mockError = new Error('Registration failed');
    mockedAxios.post.mockRejectedValueOnce(mockError);

    // Expect error to be thrown
    await expect(
      secure('https://aim.example.com', {
        name: 'test-agent',
      })
    ).rejects.toThrow('Registration failed');

    // Credentials should not be stored on error
    expect(credentials.storeCredentials).not.toHaveBeenCalled();
  });

  test('secure() includes signature in payload', async () => {
    // Mock successful registration response
    const mockResponse = {
      data: {
        id: 'test-agent-id',
        name: 'test-agent',
        apiKey: 'test-api-key',
        publicKey: 'test-public-key',
      },
    };

    mockedAxios.post.mockResolvedValueOnce(mockResponse);

    // Call secure()
    await secure('https://aim.example.com', {
      name: 'test-agent',
    });

    // Verify signature was included
    expect(mockedAxios.post).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        signature: expect.any(String),
      }),
      expect.any(Object)
    );
  });

  test('secure() includes public key in payload', async () => {
    // Mock successful registration response
    const mockResponse = {
      data: {
        id: 'test-agent-id',
        name: 'test-agent',
        apiKey: 'test-api-key',
        publicKey: 'test-public-key',
      },
    };

    mockedAxios.post.mockResolvedValueOnce(mockResponse);

    // Call secure()
    await secure('https://aim.example.com', {
      name: 'test-agent',
    });

    // Verify public key was included
    expect(mockedAxios.post).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        public_key: expect.any(String),
      }),
      expect.any(Object)
    );
  });
});

describe('secureWithOAuth() alias', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('secureWithOAuth() is defined', () => {
    expect(secureWithOAuth).toBeDefined();
    expect(typeof secureWithOAuth).toBe('function');
  });

  test('secureWithOAuth() requires OAuth provider', async () => {
    // Expect error when OAuth provider is not provided
    await expect(
      secureWithOAuth('https://aim.example.com', {
        name: 'test-agent',
      })
    ).rejects.toThrow('OAuth provider is required');
  });
});

describe('registerAgent()', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('registerAgent() is defined', () => {
    expect(registerAgent).toBeDefined();
    expect(typeof registerAgent).toBe('function');
  });

  test('registerAgent() uses default type', async () => {
    // Mock successful registration response
    const mockResponse = {
      data: {
        id: 'test-agent-id',
        name: 'test-agent',
        apiKey: 'test-api-key',
        publicKey: 'test-public-key',
      },
    };

    mockedAxios.post.mockResolvedValueOnce(mockResponse);

    // Call without type
    await registerAgent('https://aim.example.com', {
      name: 'test-agent',
    });

    // Verify default type was used
    expect(mockedAxios.post).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        type: 'ai_agent',
      }),
      expect.any(Object)
    );
  });

  test('registerAgent() respects custom type', async () => {
    // Mock successful registration response
    const mockResponse = {
      data: {
        id: 'test-agent-id',
        name: 'test-agent',
        apiKey: 'test-api-key',
        publicKey: 'test-public-key',
      },
    };

    mockedAxios.post.mockResolvedValueOnce(mockResponse);

    // Call with custom type
    await registerAgent('https://aim.example.com', {
      name: 'test-agent',
      type: 'mcp_server',
    });

    // Verify custom type was used
    expect(mockedAxios.post).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        type: 'mcp_server',
      }),
      expect.any(Object)
    );
  });
});
