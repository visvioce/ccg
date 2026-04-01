# CCG Output Handler System Spec

## Why
CCR has a comprehensive output handler system that supports multiple output destinations (console, webhook, temp-file) for token statistics and other metrics. CCG currently lacks this functionality, limiting the ability to monitor and log token usage effectively.

## What Changes
- Add OutputHandler interface and base types
- Implement ConsoleOutputHandler for console output
- Implement WebhookOutputHandler for HTTP webhook output
- Implement TempFileOutputHandler for file output
- Add OutputManager to manage multiple handlers
- Integrate with existing plugin system

## Impact
- Affected specs: plugin system, token-speed plugin
- Affected code: internal/plugin/, internal/server/

## ADDED Requirements

### Requirement: Output Handler Interface
The system SHALL provide a unified interface for output handlers.

#### Scenario: Handler registration
- **WHEN** an output handler is created
- **THEN** it implements the OutputHandler interface
- **AND** provides type, output method, and configuration

#### Scenario: Handler output
- **WHEN** data needs to be output
- **THEN** the handler formats and sends data to destination
- **AND** returns success/failure status

### Requirement: Console Output Handler
The system SHALL support console output for token statistics.

#### Scenario: Console output with colors
- **WHEN** console output is enabled
- **THEN** output formatted message to console
- **AND** support colored output for better readability

#### Scenario: Console output without colors
- **WHEN** colors are disabled
- **THEN** output plain text message

### Requirement: Webhook Output Handler
The system SHALL support webhook output for token statistics.

#### Scenario: Webhook POST request
- **WHEN** webhook output is enabled
- **THEN** send POST request to configured URL
- **AND** include token stats in JSON format
- **AND** handle HTTP errors gracefully

#### Scenario: Webhook retry
- **WHEN** webhook request fails
- **THEN** retry up to configured max retries
- **AND** log failure after all retries exhausted

### Requirement: Temp File Output Handler
The system SHALL support file output for token statistics.

#### Scenario: File creation
- **WHEN** file output is enabled
- **THEN** create JSON file in temp directory
- **AND** include timestamp in filename
- **AND** write formatted JSON data

#### Scenario: File rotation
- **WHEN** file size exceeds limit
- **THEN** create new file
- **AND** maintain configured number of backup files

### Requirement: Output Manager
The system SHALL manage multiple output handlers.

#### Scenario: Handler registration
- **WHEN** output handlers are configured
- **THEN** register all enabled handlers
- **AND** validate handler configurations

#### Scenario: Broadcast output
- **WHEN** data needs to be output
- **THEN** send to all registered handlers
- **AND** track success/failure for each handler

#### Scenario: Type-specific output
- **WHEN** output to specific type is requested
- **THEN** send only to handlers of that type
- **AND** return results per handler

## MODIFIED Requirements
None - this is a new feature addition.

## REMOVED Requirements
None - this is a new feature addition.
