# Push Validator CLI - Commands Reference

## Global Flags

These flags are available on **all** commands:

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--home` | | string | `~/.pchain` | Node home directory (overrides env) |
| `--bin` | | string | | Path to pchaind binary (overrides env) |
| `--rpc` | | string | `http://127.0.0.1:26657` | Local RPC base URL |
| `--genesis-domain` | | string | | Genesis RPC domain or URL |
| `--output` | `-o` | string | `text` | Output format: `json`\|`yaml`\|`text` |
| `--verbose` | | bool | `false` | Verbose output |
| `--quiet` | `-q` | bool | `false` | Quiet mode: minimal output |
| `--debug` | `-d` | bool | `false` | Debug output: extra diagnostic logs |
| `--no-color` | | bool | `false` | Disable ANSI colors |
| `--no-emoji` | | bool | `false` | Disable emoji output |
| `--yes` | `-y` | bool | `false` | Assume yes for all prompts |
| `--non-interactive` | | bool | `false` | Fail instead of prompting |

---

## Quick Start Commands

### `start`

Start the node process. Auto-initializes on first run (downloads genesis, snapshot, keys).

```bash
push-validator start [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--bin` | string | | Path to pchaind binary |
| `--no-prompt` | bool | `false` | Skip post-start prompts (for scripts) |

---

### `status`

Show node/RPC/sync status with process info, connectivity, and validator details.

```bash
push-validator status [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--strict` | bool | `false` | Exit non-zero if node has issues |

**Output fields (JSON):** `running`, `pid`, `rpc_listening`, `catching_up`, `height`, `remote_height`, `sync_progress`, `is_validator`, `peers`, `latency_ms`, `node_id`, `moniker`, `network`

---

### `dashboard`

Interactive terminal dashboard with real-time validator metrics.

```bash
push-validator dashboard [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--refresh-interval` | duration | `2s` | Dashboard refresh interval |
| `--rpc-timeout` | duration | `15s` | RPC request timeout |
| `--debug` | bool | `false` | Enable debug mode |

---

## Operations

### `stop`

Stop the node process gracefully.

```bash
push-validator stop
```

---

### `restart`

Restart the node process (stop then start).

```bash
push-validator restart [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--bin` | string | | Path to pchaind binary |

---

### `logs`

View node logs with interactive TUI (search, filtering) or tail in non-interactive mode.

```bash
push-validator logs
```

---

### `sync`

Monitor blockchain sync progress with real-time speed and ETA.

```bash
push-validator sync [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--compact` | bool | `false` | Compact output format |
| `--window` | int | `30` | Moving average window (headers) for speed calculation |
| `--rpc` | string | | Local RPC base URL |
| `--remote` | string | | Remote RPC base for sync target |
| `--interval` | duration | `120ms` | Update interval |
| `--skip-final-message` | bool | `false` | Suppress completion message (automation) |
| `--stuck-timeout` | duration | `0` | Stuck detection timeout (`0` = default or env `PNM_SYNC_STUCK_TIMEOUT`) |

---

## Validator Commands

### `validators`

List all active validators on the network.

```bash
push-validator validators
```

**Output:** Table with Moniker, Cosmos Address, Status, Stake, Commission %, Rewards, EVM Address. Use `--output json` for raw chain data.

---

### `balance`

Check account balance.

```bash
push-validator balance [address] [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--address` | string | | Account address (alternative to positional arg) |

If no address provided, uses the validator key from config.

---

### `register-validator`

Register this node as a validator on the network. Interactive flow prompts for moniker, commission rate, and stake amount.

```bash
push-validator register-validator [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--check-only` | bool | `false` | Exit after reporting registration status |

**Requirements:** 2+ PC tokens, node must be synced.

---

### `unjail`

Restore a jailed validator to active status (after jail period expires).

```bash
push-validator unjail
```

---

### `withdraw-rewards`

Withdraw accumulated delegation rewards and optionally validator commission.

```bash
push-validator withdraw-rewards
```

**Aliases:** `withdraw`, `claim-rewards`

---

### `increase-stake`

Delegate additional tokens to increase validator stake and voting power.

```bash
push-validator increase-stake
```

---

### `restake-rewards`

Automatically withdraw all rewards (commission + outstanding) and restake them.

```bash
push-validator restake-rewards
```

**Aliases:** `restake`

---

## Maintenance

### `backup`

Create a backup archive (tar.gz) of node configuration and validator state.

```bash
push-validator backup
```

**Output:** Archive saved to `~/push-node-backups/`

---

### `reset`

Reset chain data while preserving the address book. Requires confirmation.

```bash
push-validator reset
```

Use `--yes` to skip confirmation prompt.

---

### `full-reset`

Complete reset deleting ALL data including validator keys. Creates a new validator identity.

```bash
push-validator full-reset
```

Use `--yes` to skip confirmation prompt. Use `--non-interactive` mode requires `--yes`.

---

### `doctor`

Run comprehensive health checks on validator setup.

```bash
push-validator doctor
```

**Checks:** Process status, RPC accessibility, config files, P2P network, remote connectivity, disk space, file permissions, sync status, Cosmovisor status.

---

## Utilities

### `peers`

Show connected peer information.

```bash
push-validator peers
```

---

### `update`

Check for and install the latest version of push-validator CLI.

```bash
push-validator update [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--check` | bool | `false` | Only check for updates, don't install |
| `--force` | bool | `false` | Skip confirmation prompt |
| `--version` | string | | Install specific version (e.g., `v1.2.0`) |
| `--no-verify` | bool | `false` | Skip checksum verification |

---

## Chain Binary Management

### `chain install`

Download and install the pchaind chain binary from GitHub releases.

```bash
push-validator chain install [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--version` | string | | Install specific version (e.g., `v0.0.2`) |
| `--force` | bool | `false` | Force reinstall even if installed |
| `--no-verify` | bool | `false` | Skip checksum verification |

---

## Snapshot Management

### `snapshot download`

Download a blockchain snapshot to the local cache.

```bash
push-validator snapshot download [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--snapshot-url` | string | | Snapshot download URL (default: from config) |
| `--no-cache` | bool | `false` | Force fresh download, bypass cache |

---

### `snapshot extract`

Extract the cached blockchain snapshot to the node's data directory.

```bash
push-validator snapshot extract [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--target` | string | `~/.pchain/data` | Target directory for extraction |
| `--force` | bool | `false` | Force extraction even if exists |

---

## Cosmovisor Management

### `cosmovisor status`

Show Cosmovisor status including versions and pending upgrades.

```bash
push-validator cosmovisor status
```

---

### `cosmovisor upgrade-info`

Generate upgrade-info.json with SHA256 checksums for all supported platforms.

```bash
push-validator cosmovisor upgrade-info [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--version` | string | **(required)** | Version to generate info for |
| `--url` | string | **(required)** | Release download URL base |
| `--height` | int64 | `0` | Upgrade height |

---

## Setup (Hidden)

### `init`

Initialize local node home directory. Used internally by `install.sh` and `start` auto-init.

```bash
push-validator init [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--moniker` | string | `push-validator` | Validator moniker (env: `MONIKER`) |
| `--chain-id` | string | | Chain ID |
| `--snapshot-url` | string | | Snapshot download base URL |
| `--skip-snapshot` | bool | `false` | Skip snapshot download |

---

## Shell Completion

### `completion`

Generate shell completion scripts.

```bash
push-validator completion bash
push-validator completion zsh
push-validator completion fish
push-validator completion powershell
```

---

## Version

### `version`

Show CLI version, commit hash, and build date.

```bash
push-validator version
```

Supports `--output json` and `--output yaml`.

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MONIKER` | Validator moniker | `push-validator` |
| `KEY_NAME` | Key name for signing | `validator-key` |
| `STAKE_AMOUNT` | Initial stake (smallest units) | `1500000000000000000` (1.5 PC) |
| `PCHAIND` | Path to pchaind binary | auto-detected |
| `PCHAIN_BIN` | Alternative pchaind path | |
| `HOME_DIR` | Node home directory | `~/.pchain` |
| `NO_COLOR` | Disable colors globally | |
| `PNM_SYNC_STUCK_TIMEOUT` | Sync stuck detection timeout | |

---

## File Locations

| Path | Purpose |
|------|---------|
| `~/.pchain/config/` | Node configuration |
| `~/.pchain/data/` | Blockchain data |
| `~/.pchain/logs/` | Node logs |
| `~/.pchain/cosmovisor/` | Cosmovisor binaries |
| `~/push-node-backups/` | Backup archives |
