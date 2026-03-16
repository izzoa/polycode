## ADDED Requirements

### Requirement: API key authentication
The system SHALL support API key-based authentication for all provider types. API keys SHALL be stored securely via the OS keyring (macOS Keychain, Linux secret-service) with fallback to an encrypted file.

#### Scenario: API key configured for provider
- **WHEN** a provider is configured with `auth: api_key`
- **THEN** the system retrieves the API key from the OS keyring and includes it in API requests to that provider

#### Scenario: API key not found in keyring
- **WHEN** a provider requires an API key but none is stored in the keyring
- **THEN** the system prompts the user to enter the key interactively and stores it in the keyring

#### Scenario: Keyring unavailable
- **WHEN** the OS keyring is not available
- **THEN** the system falls back to storing the API key in an encrypted file at `~/.config/polycode/credentials.enc`

### Requirement: OAuth device flow authentication
The system SHALL support OAuth 2.0 device authorization flow for providers that use OAuth (Anthropic Claude, Google Gemini). The flow presents the user with a URL and code to complete authentication in their browser.

#### Scenario: OAuth login initiated
- **WHEN** a provider is configured with `auth: oauth` and no valid token exists
- **THEN** the system initiates the device authorization flow, displaying a URL and user code for browser-based authentication

#### Scenario: OAuth token obtained
- **WHEN** the user completes the browser-based authorization
- **THEN** the system receives and stores the access token and refresh token in the OS keyring

#### Scenario: OAuth token refresh
- **WHEN** an OAuth access token has expired but a valid refresh token exists
- **THEN** the system automatically refreshes the access token without user interaction

#### Scenario: OAuth refresh token expired
- **WHEN** both the access token and refresh token have expired
- **THEN** the system re-initiates the device authorization flow and prompts the user to re-authenticate

### Requirement: No-auth support for local endpoints
The system SHALL support `auth: none` for providers that do not require authentication (e.g., locally hosted models).

#### Scenario: Local provider with no auth
- **WHEN** a provider is configured with `auth: none`
- **THEN** the system sends requests without any authentication headers

### Requirement: Credential management commands
The system SHALL provide CLI subcommands for managing stored credentials: `polycode auth login <provider>`, `polycode auth logout <provider>`, and `polycode auth status`.

#### Scenario: Login command
- **WHEN** the user runs `polycode auth login claude`
- **THEN** the system initiates the appropriate auth flow (OAuth or API key prompt) for the named provider

#### Scenario: Logout command
- **WHEN** the user runs `polycode auth logout claude`
- **THEN** the system removes stored credentials for that provider from the keyring

#### Scenario: Auth status command
- **WHEN** the user runs `polycode auth status`
- **THEN** the system displays each provider's auth method and whether valid credentials exist (without revealing secrets)
