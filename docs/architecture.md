# Bindplane Loader Architecture

This document provides a comprehensive overview of the bindplane-loader application architecture, covering the main components, their responsibilities, and how they interact to create a robust log generation and forwarding system.

## Overview

Bindplane Loader is a high-performance log generation and forwarding application designed to simulate realistic log traffic for testing and benchmarking purposes. The application follows a modular architecture with clear separation of concerns, making it extensible and maintainable.

## Architecture Diagram

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   main.go       │    │   Service       │    │   Generator     │
│                 │    │                 │    │                 │
│ • Lifecycle     │───▶│ • Orchestration │───▶│ • Data Creation │
│ • Configuration │    │ • Start/Stop    │    │ • Worker Mgmt   │
│ • Signal Handle │    │ • Error Handle  │    │ • Rate Control  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       ▼
         │                       │              ┌─────────────────┐
         │                       │              │   Output        │
         │                       │              │                 │
         │                       └─────────────▶│ • TCP/UDP       │
         │                                      │ • Worker Mgmt   │
         │                                      │ • Connection    │
         │                                      │ • Retry Logic   │
         │                                      └─────────────────┘
         │
         ▼
┌─────────────────┐
│   Config        │
│                 │
│ • Validation    │
│ • Overrides     │
│ • File/Env/Flag │
└─────────────────┘
```

## Core Components

### 1. main.go - Application Lifecycle Management

**Location:** `cmd/loader/main.go`

The main.go file serves as the application entry point and manages the entire application lifecycle. It handles:

#### Key Responsibilities:
- **Configuration Management**: Parses command-line flags, environment variables, and configuration files
- **Component Initialization**: Creates and configures logger, generator, and output components
- **Signal Handling**: Manages graceful shutdown on SIGINT/SIGTERM signals
- **Error Handling**: Provides comprehensive error handling with proper exit codes
- **Lifecycle Orchestration**: Coordinates startup and shutdown of all components

#### Lifecycle Flow:
1. **Parse Configuration**: Process flags, environment variables, and config files
2. **Validate Configuration**: Ensure all required settings are valid
3. **Initialize Logger**: Set up structured logging with appropriate levels
4. **Create Signal Context**: Set up graceful shutdown handling
5. **Initialize Components**: Create generator and output instances
6. **Start Service**: Begin log generation and forwarding
7. **Wait for Shutdown**: Block until shutdown signal received
8. **Graceful Shutdown**: Stop all components cleanly

#### Configuration Priority:
1. Command-line flags (highest priority)
2. Environment variables
3. Configuration file (when `--config` specified)
4. Default values (lowest priority)

### 2. internal/config Package - Configuration Management

**Location:** `internal/config/`

The config package provides a comprehensive configuration system with validation, overrides, and multiple input sources.

#### Key Components:

##### Config Structure (`config.go`)
```go
type Config struct {
    Logging   Logging   `yaml:"logging,omitempty"`
    Generator Generator `yaml:"generator,omitempty"`
    Output    Output    `yaml:"output,omitempty"`
}
```

##### Override System (`override.go`)
- **Override Structure**: Defines configuration overrides with field mapping, flags, and environment variables
- **Flag Generation**: Automatically creates command-line flags from configuration fields
- **Environment Mapping**: Maps configuration fields to environment variables with `BINDPLANE_` prefix
- **Validation**: Ensures configuration values meet requirements

##### Configuration Types:
- **Logging**: Output destination and log level configuration
- **Generator**: Generator type and specific configuration (JSON generator)
- **Output**: Output type and specific configuration (TCP/UDP)

#### Features:
- **Multi-source Configuration**: Supports YAML files, environment variables, and command-line flags
- **Validation**: Comprehensive validation with detailed error messages
- **Type Safety**: Strong typing with proper validation constraints
- **Extensibility**: Easy to add new configuration options

### 3. internal/service Package - Service Orchestration

**Location:** `internal/service/service.go`

The service package provides high-level orchestration of the generator and output components.

#### Service Structure:
```go
type Service struct {
    Logger    *zap.Logger
    Generator generator.Generator
    Output    output.Output
}
```

#### Key Responsibilities:
- **Component Coordination**: Manages the interaction between generator and output
- **Lifecycle Management**: Handles start and stop operations for all components
- **Error Propagation**: Ensures errors from components are properly handled
- **Graceful Shutdown**: Coordinates shutdown with timeout handling

#### Lifecycle Methods:
- **Start()**: Initiates the generator, which begins producing logs
- **Stop()**: Stops both generator and output with 30-second timeout

### 4. generator Package - Log Data Generation

**Location:** `generator/`

The generator package creates realistic log data with configurable patterns and rates.

#### Generator Interface:
```go
type Generator interface {
    Start(writer generatorWriter) error
    Stop(ctx context.Context) error
}
```

#### NOP Generator Implementation (`nop.go`):

##### Key Features:
- **No Operation**: Performs no work and generates no data
- **Testing Utility**: Useful for testing application infrastructure without generating actual logs
- **Minimal Resource Usage**: Consumes minimal CPU and memory resources
- **No Configuration**: Requires no additional configuration options

##### Use Cases:
- **Infrastructure Testing**: Test the application startup and shutdown without generating data
- **Development**: Quick testing of configuration changes without data generation
- **CI/CD**: Automated testing without external dependencies

#### JSON Generator Implementation (`json.go`):

##### Key Features:
- **Realistic Log Data**: Generates JSON logs with realistic fields (timestamp, level, environment, location, message)
- **Configurable Workers**: Supports multiple worker goroutines for parallel generation
- **Rate Control**: Configurable generation rate with exponential backoff
- **Rich Content**: 100+ unique log messages of ~500 bytes each
- **Randomization**: Random selection of log levels, environments, and locations

##### Worker Management:
- **Concurrent Workers**: Multiple goroutines generate logs simultaneously
- **Exponential Backoff**: Automatic retry with increasing delays on failures
- **Graceful Shutdown**: Clean worker termination with context cancellation
- **Error Handling**: Comprehensive error logging and recovery

##### Log Structure:
```json
{
    "timestamp": "2024-01-15T10:30:45Z",
    "level": "INFO",
    "environment": "production",
    "location": "us-east1",
    "message": "User authentication failed for user_id=12345..."
}
```

### 5. output Package - Data Forwarding

**Location:** `output/`

The output package handles forwarding generated logs to external destinations via TCP or UDP.

#### Output Interface:
```go
type Output interface {
    Write(ctx context.Context, data []byte) error
    Stop(ctx context.Context) error
}
```

#### NOP Output Implementation (`nop.go`):

##### Key Features:
- **No Operation**: Performs no work and discards all data
- **Testing Utility**: Useful for testing application infrastructure without sending data to external destinations
- **Minimal Resource Usage**: Consumes minimal CPU and memory resources
- **No Configuration**: Requires no additional configuration options

##### Use Cases:
- **Infrastructure Testing**: Test the application startup and shutdown without external dependencies
- **Development**: Quick testing of configuration changes without network requirements
- **CI/CD**: Automated testing without external service dependencies

#### TCP Implementation (`tcp.go`):

##### Key Features:
- **Persistent Connections**: Maintains TCP connections for efficient data transfer
- **Worker Management**: Multiple worker goroutines handle concurrent connections
- **Automatic Reconnection**: Failed connections are automatically re-established
- **Timeout Handling**: Configurable timeouts for connection and write operations
- **Data Formatting**: Appends newlines to log data for proper line separation

##### Connection Management:
- **Connection Pool**: Each worker maintains its own TCP connection
- **Error Recovery**: Failed connections trigger worker restart with backoff
- **Graceful Shutdown**: Clean connection closure on shutdown

#### UDP Implementation (`udp.go`):

##### Key Features:
- **Connectionless Protocol**: Uses UDP for high-throughput, low-latency forwarding
- **Worker Management**: Multiple worker goroutines for parallel data transmission
- **Automatic Reconnection**: Failed connections are automatically re-established
- **Timeout Handling**: Configurable write timeouts
- **No Data Formatting**: Raw data transmission without modification

#### Worker Management Integration:

Both TCP and UDP implementations use the `internal/workermanager` package for robust worker management:

- **Automatic Restart**: Failed workers are automatically restarted
- **Exponential Backoff**: Increasing delays between restart attempts
- **Context Awareness**: Workers respect shutdown signals
- **Resource Management**: Proper cleanup and resource tracking

### 6. internal/workermanager Package - Worker Management

**Location:** `internal/workermanager/workermanager.go`

The workermanager package provides a robust worker management system for handling potentially failing operations.

#### Key Features:
- **Automatic Restart**: Failed workers are automatically restarted with exponential backoff
- **Graceful Shutdown**: Context-aware shutdown with proper cleanup
- **Resource Tracking**: Thread-safe worker count tracking
- **Comprehensive Logging**: Detailed logging of failures and retry attempts
- **Configurable Policies**: Customizable retry policies with sane defaults

#### Worker Lifecycle:
1. **Start**: Worker begins execution
2. **Failure Detection**: Worker exits on failure
3. **Backoff Calculation**: Exponential backoff delay calculated
4. **Retry**: Worker restarted after delay
5. **Shutdown**: Clean termination on context cancellation

## Data Flow

### 1. Configuration Loading
```
Command Line → Environment Variables → Config File → Defaults
```

### 2. Component Initialization
```
main.go → Service → Generator + Output → WorkerManager
```

### 3. Log Generation and Forwarding
```
Generator Workers → JSON Log Creation → Output Channel → Output Workers → Network
```

### 4. Error Handling and Recovery
```
Worker Failure → Exponential Backoff → Worker Restart → Continued Operation
```

## Concurrency Model

### Worker Architecture:
- **Generator Workers**: Multiple goroutines generate logs concurrently
- **Output Workers**: Multiple goroutines handle network I/O concurrently
- **Channel Communication**: Buffered channels coordinate between components
- **Context Propagation**: Context cancellation propagates through all workers

### Synchronization:
- **WaitGroups**: Ensure proper worker cleanup
- **Mutexes**: Protect shared state in worker managers
- **Channels**: Coordinate data flow and shutdown signals
- **Context**: Provide cancellation and timeout handling

## Error Handling Strategy

### Configuration Errors:
- **Validation Failures**: Detailed error messages with specific field information
- **File Read Errors**: Clear error messages for missing or invalid config files
- **Early Exit**: Application exits with appropriate error codes

### Runtime Errors:
- **Worker Failures**: Automatic restart with exponential backoff
- **Network Errors**: Connection retry with increasing delays
- **Resource Exhaustion**: Graceful degradation and error reporting

### Shutdown Errors:
- **Timeout Handling**: 30-second timeout for graceful shutdown
- **Resource Cleanup**: Proper cleanup of connections and goroutines
- **Error Propagation**: Shutdown errors are logged and reported

## Performance Characteristics

### Scalability:
- **Horizontal Scaling**: Multiple workers for both generation and output
- **Configurable Concurrency**: Adjustable worker counts for different loads
- **Efficient I/O**: Buffered channels and persistent connections

### Reliability:
- **Automatic Recovery**: Failed operations are automatically retried
- **Graceful Degradation**: Partial failures don't stop the entire system
- **Resource Management**: Proper cleanup prevents resource leaks

### Observability:
- **Structured Logging**: Comprehensive logging with contextual information
- **Metrics**: Worker counts, failure rates, and performance indicators
- **Error Tracking**: Detailed error logging for troubleshooting

## Extension Points

### Adding New Generators:
1. Implement the `Generator` interface
2. Add configuration types to the config package
3. Update main.go to handle the new generator type
4. Add validation rules and tests

### Adding New Outputs:
1. Implement the `Output` interface
2. Add configuration types to the config package
3. Update main.go to handle the new output type
4. Integrate with the worker manager for robust operation

### Configuration Extensions:
1. Add new fields to configuration structures
2. Create override definitions for flags and environment variables
3. Add validation rules
4. Update documentation

This architecture provides a solid foundation for a high-performance, reliable log generation and forwarding system that can be easily extended and maintained.
