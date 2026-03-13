export interface GatewayClientOptions {
  baseURL: string
  apiKey: string
}

export class GatewayClient {
  private readonly baseURL: string
  private readonly apiKey: string

  constructor(options: GatewayClientOptions) {
    this.baseURL = options.baseURL
    this.apiKey = options.apiKey
  }

  getHeaders(): Record<string, string> {
    return {
      Authorization: `Bearer ${this.apiKey}`,
      'Content-Type': 'application/json',
    }
  }

  getBaseURL(): string {
    return this.baseURL
  }
}
