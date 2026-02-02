# cmd/zap

This is the entry point for the ZAP application. It uses [Cobra](https://github.com/spf13/cobra) for CLI framework and [Viper](https://github.com/spf13/viper) for configuration management.

## Package Overview

```
cmd/zap/
└── main.go    # CLI setup, flag parsing, initialization, routes to TUI or CLI mode
```

## CLI Modes

ZAP supports two execution modes:

### Interactive Mode (Default)

Launches the full TUI for interactive API testing:

```bash
./zap
./zap --framework gin
```

### CLI Mode

Executes a saved request and exits (for automation):

```bash
./zap --request get-users --env prod
./zap -r get-users -e dev
```

## Command Line Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--framework` | `-f` | Set/update API framework (gin, fastapi, express, etc.) |
| `--request` | `-r` | Execute a saved request by name |
| `--env` | `-e` | Environment to use (dev, prod, staging) |
| `--config` | | Path to custom config file |
| `--help` | `-h` | Show help |

## Main Function Structure

```go
func main() {
    rootCmd := &cobra.Command{
        Use:   "zap",
        Short: "AI-powered API debugging assistant",
        Long:  `ZAP is a terminal-based AI assistant that tests and debugs APIs.`,
        Run:   run,
    }

    // Add flags
    rootCmd.Flags().StringP("framework", "f", "", "API framework")
    rootCmd.Flags().StringP("request", "r", "", "Execute saved request")
    rootCmd.Flags().StringP("env", "e", "", "Environment to use")
    rootCmd.Flags().String("config", "", "Config file path")

    // Execute
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

## Initialization Flow

```go
func run(cmd *cobra.Command, args []string) {
    // 1. Load environment variables from .env
    godotenv.Load()

    // 2. Get flag values
    framework, _ := cmd.Flags().GetString("framework")
    requestName, _ := cmd.Flags().GetString("request")
    envName, _ := cmd.Flags().GetString("env")

    // 3. Load or create config
    config := loadConfig()

    // 4. Update framework if provided
    if framework != "" {
        config.Framework = framework
        saveConfig(config)
        fmt.Printf("Updated framework to: %s\n", framework)
    }

    // 5. Route to appropriate mode
    if requestName != "" {
        // CLI mode: execute request and exit
        runCLI(config, requestName, envName)
    } else {
        // Interactive mode: launch TUI
        tui.Run()
    }
}
```

## CLI Mode Implementation

```go
func runCLI(config *core.Config, requestName, envName string) {
    // Load environment
    var variables map[string]string
    if envName != "" {
        env, err := storage.LoadEnvironment(
            filepath.Join(".zap/environments", envName+".yaml"),
        )
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error loading environment: %v\n", err)
            os.Exit(1)
        }
        variables = env.Variables
    }

    // Load request
    request, err := storage.LoadRequest(
        filepath.Join(".zap/requests", requestName+".yaml"),
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error loading request: %v\n", err)
        os.Exit(1)
    }

    // Substitute variables
    request = storage.SubstituteRequest(request, variables)

    // Execute request
    resp, err := executeHTTPRequest(request)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error executing request: %v\n", err)
        os.Exit(1)
    }

    // Output response
    renderResponse(resp)
}
```

## Configuration Loading

```go
func loadConfig() *core.Config {
    // Default path
    configPath := filepath.Join(".zap", "config.json")

    // Check for custom path
    if customPath := viper.GetString("config"); customPath != "" {
        configPath = customPath
    }

    // Check if config exists
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        return nil // Will trigger setup wizard in TUI
    }

    // Load config
    data, err := os.ReadFile(configPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
        os.Exit(1)
    }

    var config core.Config
    if err := json.Unmarshal(data, &config); err != nil {
        fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
        os.Exit(1)
    }

    return &config
}
```

## Environment Variables

ZAP loads environment variables from `.env`:

```env
# .env
OLLAMA_API_KEY=your-ollama-cloud-key
GEMINI_API_KEY=your-gemini-key
```

Loaded at startup:

```go
import "github.com/joho/godotenv"

func init() {
    // Load .env file (ignores errors if file doesn't exist)
    _ = godotenv.Load()
}
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (config, request loading, execution) |

## Usage Examples

### First Time Setup

```bash
# Interactive setup wizard
./zap

# Or specify framework directly
./zap --framework fastapi
```

### Daily Usage

```bash
# Interactive mode
./zap

# Execute saved request
./zap -r get-users -e dev
./zap --request create-user --env prod
```

### CI/CD Integration

```bash
#!/bin/bash
# Run API tests in CI

./zap --request health-check --env staging
if [ $? -ne 0 ]; then
    echo "Health check failed"
    exit 1
fi

./zap --request get-users --env staging
./zap --request create-user --env staging
```

### Framework Updates

```bash
# Change framework for a project
./zap --framework express

# Verify in config
cat .zap/config.json | jq .framework
# "express"
```

## Development

### Building

```bash
go build -o zap.exe ./cmd/zap
```

### Running Locally

```bash
go run ./cmd/zap
go run ./cmd/zap --framework gin
go run ./cmd/zap -r my-request -e dev
```

### Testing

```bash
# Test CLI parsing
go test ./cmd/zap/...
```

## Adding New Flags

To add a new CLI flag:

1. Add flag definition in `main.go`:

```go
rootCmd.Flags().Bool("verbose", false, "Enable verbose output")
```

2. Retrieve in `run()`:

```go
verbose, _ := cmd.Flags().GetBool("verbose")
if verbose {
    // Enable verbose mode
}
```

3. Update help text if needed:

```go
rootCmd.Long = `ZAP is a terminal-based AI assistant...

Flags:
  --verbose    Enable verbose output for debugging`
```
