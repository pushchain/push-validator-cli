#!/usr/bin/env bash
# Push Validator Manager (Go) — Installer with binary download + guided start
#
# This script installs the Push Validator Manager by downloading pre-built binaries
# from GitHub Releases. Use --dev for development builds from source.
#
# Examples:
#   bash install.sh                            # default: download binaries, init+start, wait for sync
#   bash install.sh --no-reset --no-start      # install only
#   bash install.sh --dev                      # build from current repo checkout (dev mode)
#   bash install.sh --dev-dir /path/to/repo    # build from specific directory
#   PNM_REF=v1.0.0 bash install.sh             # download specific version of push-validator
#   PCHAIN_REF=v1.0.0 bash install.sh          # download specific version of pchaind

set -euo pipefail
IFS=$'\n\t'

# Styling and output functions
CYAN='\033[0;36m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; RED='\033[0;31m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'
NO_COLOR="${NO_COLOR:-}"
VERBOSE="${VERBOSE:-no}"

# Disable colors if NO_COLOR is set or not a terminal
if [[ -n "$NO_COLOR" ]] || [[ ! -t 1 ]]; then
    CYAN=''; GREEN=''; YELLOW=''; RED=''; BOLD=''; DIM=''; NC=''
fi

# Prevent terminal background color queries from polluting output
# This tells charmbracelet/termenv libraries the background color,
# avoiding OSC 11 queries that cause ^[]11;rgb:... responses in iTerm2
export COLORFGBG="${COLORFGBG:-0;15}"

status() { echo -e "${CYAN}$*${NC}"; }
ok()     {
    if [[ $PHASE_START_TIME -gt 0 ]]; then
        local delta=$(($(date +%s) - PHASE_START_TIME))
        local unit="s"
        local time_val=$delta
        # Show milliseconds for sub-second times
        if [[ $delta -eq 0 ]]; then
            time_val="<1"
            unit="s"
        fi
        echo -e "  ${GREEN}✓ $* (${time_val}${unit})${NC}"
    else
        echo -e "  ${GREEN}✓ $*${NC}"
    fi
}
ok_sub() { echo -e "    ${GREEN}✓ $*${NC}"; }  # 4-space indent for nested sub-items
warn()   { echo -e "  ${YELLOW}⚠ $*${NC}"; }
err()    { echo -e "  ${RED}✗ $*${NC}"; }
phase()  { echo -e "\n${BOLD}${CYAN}▸ $*${NC}"; }
step()   { echo -e "  ${DIM}→${NC} $*"; }
step_sub() { echo -e "    ${DIM}→${NC} $*"; }  # 4-space indent for nested sub-steps
verbose() { [[ "$VERBOSE" = "yes" ]] && echo -e "  ${DIM}$*${NC}" || true; }

# Helper: Shorten path for display (replace $HOME with ~)
short_path() { echo "${1/#$HOME/\~}"; }

# Helper: Indent output lines (adds 2-space prefix)
indent_output() {
    while IFS= read -r line; do
        if [[ -n "$line" ]]; then
            echo "  $line"
        else
            echo ""
        fi
    done
}

# Helper: Find timeout command (macOS needs gtimeout)
timeout_cmd() {
    if command -v timeout >/dev/null 2>&1; then
        echo "timeout"
    elif command -v gtimeout >/dev/null 2>&1; then
        echo "gtimeout"
    else
        echo ""
    fi
}

# Helper: Get file size in bytes (portable: macOS + Linux)
get_file_size() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        echo 0
        return
    fi
    # macOS: stat -f%z, Linux: stat -c%s, fallback: wc -c
    stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || wc -c < "$file" | tr -d ' '
}

# Helper: Format byte count to human-readable (matches Go FormatBytes: 1024-based)
format_bytes() {
    awk "BEGIN {
        b = $1
        if (b >= 1073741824) printf \"%.1fGB\", b/1073741824
        else if (b >= 1048576) printf \"%.1fMB\", b/1048576
        else if (b >= 1024) printf \"%.1fKB\", b/1024
        else printf \"%dB\", b
    }"
}

# Helper: Format bytes/sec to human-readable speed (matches Go FormatSpeed)
format_speed() {
    awk "BEGIN {
        s = $1
        if (s >= 1073741824) printf \"%.1fGB/s\", s/1073741824
        else if (s >= 1048576) printf \"%.1fMB/s\", s/1048576
        else if (s >= 1024) printf \"%.1fKB/s\", s/1024
        else printf \"%.0fB/s\", s
    }"
}

# Helper: Format seconds to duration string (matches Go formatDuration)
format_eta() {
    local sec=$1
    if [[ $sec -lt 0 ]]; then
        printf "--"
    elif [[ $sec -lt 60 ]]; then
        printf "%ds" "$sec"
    elif [[ $sec -lt 3600 ]]; then
        printf "%dm%ds" "$((sec / 60))" "$((sec % 60))"
    else
        printf "%dh%dm" "$((sec / 3600))" "$(((sec % 3600) / 60))"
    fi
}

# Helper: Render one frame of the progress bar (matches Go ui.ProgressBar)
# Arguments: current_bytes total_bytes start_epoch
render_progress_bar() {
    local current=$1 total=$2 start_epoch=$3
    local indent="  "

    if [[ $total -le 0 ]]; then
        printf "\r%sDownloading... %s\033[K" "$indent" "$(format_bytes "$current")"
        return
    fi

    # Percentage
    local pct
    pct=$(awk "BEGIN {printf \"%.1f\", ($current / $total) * 100}")

    # Speed and ETA
    local now elapsed speed eta
    now=$(date +%s)
    elapsed=$((now - start_epoch))
    if [[ $elapsed -gt 0 ]]; then
        speed=$((current / elapsed))
    else
        speed=0
    fi

    if [[ $speed -gt 0 && $current -lt $total ]]; then
        local remaining=$(( (total - current) / speed ))
        eta=$(format_eta "$remaining")
    elif [[ $current -ge $total ]]; then
        eta="0s"
    else
        eta="--"
    fi

    # Bar width (matches Go: min(40, max(10, term_width - 56 - indent_len)))
    local term_width=80
    if command -v tput >/dev/null 2>&1; then
        local tw
        tw=$(tput cols 2>/dev/null || echo 80)
        [[ $tw -gt 0 ]] && term_width=$tw
    fi
    local bar_width=$((term_width - 56 - ${#indent}))
    [[ $bar_width -lt 10 ]] && bar_width=10
    [[ $bar_width -gt 40 ]] && bar_width=40

    # Filled/unfilled counts
    local filled unfilled
    filled=$(awk "BEGIN {v=int(($pct / 100) * $bar_width); if(v>$bar_width) v=$bar_width; if(v<0) v=0; print v}")
    unfilled=$((bar_width - filled))

    # Unicode detection (fallback to ASCII)
    local fill_char unfill_char
    if [[ "${LANG:-}${LC_ALL:-}" == *[Uu][Tt][Ff]* ]]; then
        fill_char=$'\xe2\x96\x88'   # █ (U+2588)
        unfill_char=$'\xe2\x96\x91' # ░ (U+2591)
    else
        fill_char="#"
        unfill_char="-"
    fi

    # Build bar string
    local bar_str=""
    local i
    for ((i = 0; i < filled; i++)); do bar_str+="$fill_char"; done
    for ((i = 0; i < unfilled; i++)); do bar_str+="$unfill_char"; done

    # Format values
    local cur_fmt tot_fmt spd_fmt
    cur_fmt=$(format_bytes "$current")
    tot_fmt=$(format_bytes "$total")
    spd_fmt=$(format_speed "$speed")

    printf "\r%s[%s] %5s%%   %s/%s   %s   ETA %s\033[K" \
        "$indent" "$bar_str" "$pct" "$cur_fmt" "$tot_fmt" "$spd_fmt" "$eta"
}

# Helper: Download a file with Go-style progress bar
# Arguments: url output_file
# Returns: 0 on success, non-zero on failure
download_with_progress() {
    local url="$1" output_file="$2"
    local total_size=0 use_curl=1

    if command -v curl >/dev/null 2>&1; then
        use_curl=1
    elif command -v wget >/dev/null 2>&1; then
        use_curl=0
    else
        return 1
    fi

    # Non-TTY fallback: simple download with percentage lines at 10% intervals
    if [[ ! -t 1 ]]; then
        if [[ $use_curl -eq 1 ]]; then
            # Get size for percentage reporting
            total_size=$(curl -sLI "$url" 2>/dev/null | grep -i '^content-length:' | tail -1 | awk '{print $2}' | tr -d '\r\n')
            total_size=${total_size:-0}
            curl -sL -o "$output_file" "$url" < /dev/null &
        else
            total_size=$(wget --spider --server-response "$url" 2>&1 | grep -i 'content-length:' | tail -1 | awk '{print $2}' | tr -d '\r\n')
            total_size=${total_size:-0}
            wget -q -O "$output_file" "$url" < /dev/null &
        fi
        local dl_pid=$!
        local last_threshold=-1
        while kill -0 "$dl_pid" 2>/dev/null; do
            if [[ -f "$output_file" && $total_size -gt 0 ]]; then
                local cur
                cur=$(get_file_size "$output_file")
                local threshold=$(( (cur * 100 / total_size) / 10 * 10 ))
                if [[ $threshold -gt $last_threshold ]]; then
                    last_threshold=$threshold
                    printf "  Downloading... %d%%\n" "$threshold"
                fi
            fi
            sleep 1
        done
        wait "$dl_pid"
        return $?
    fi

    # TTY mode: show progress bar

    # Get Content-Length via HEAD request (follows redirects)
    if [[ $use_curl -eq 1 ]]; then
        total_size=$(curl -sLI "$url" 2>/dev/null | grep -i '^content-length:' | tail -1 | awk '{print $2}' | tr -d '\r\n')
    else
        total_size=$(wget --spider --server-response "$url" 2>&1 | grep -i 'content-length:' | tail -1 | awk '{print $2}' | tr -d '\r\n')
    fi
    total_size=${total_size:-0}

    # Start download in background (silent)
    local start_epoch dl_pid
    start_epoch=$(date +%s)

    if [[ $use_curl -eq 1 ]]; then
        curl -sL -o "$output_file" "$url" < /dev/null &
        dl_pid=$!
    else
        wget -q -O "$output_file" "$url" < /dev/null &
        dl_pid=$!
    fi

    # Poll file size and render progress bar
    local current_size=0
    while kill -0 "$dl_pid" 2>/dev/null; do
        if [[ -f "$output_file" ]]; then
            current_size=$(get_file_size "$output_file")
        fi
        render_progress_bar "$current_size" "$total_size" "$start_epoch"
        sleep 0.2 2>/dev/null || sleep 1
    done

    # Capture exit code
    wait "$dl_pid"
    local dl_exit=$?

    # Final frame (100% if successful)
    if [[ $dl_exit -eq 0 && -f "$output_file" ]]; then
        current_size=$(get_file_size "$output_file")
        [[ $total_size -eq 0 ]] && total_size=$current_size
        render_progress_bar "$current_size" "$total_size" "$start_epoch"
    fi
    printf "\n"

    return $dl_exit
}

# Helper: Check if node is running
node_running() {
    local TO; TO=$(timeout_cmd)
    local status_json
    if [[ -n "$TO" ]]; then
        status_json=$($TO 2 "$MANAGER_BIN" status --output json < /dev/null 2>/dev/null || echo "{}")
    else
        status_json=$("$MANAGER_BIN" status --output json < /dev/null 2>/dev/null || echo "{}")
    fi

    if command -v jq >/dev/null 2>&1; then
        echo "$status_json" | jq -er '.node.running // .running // false' >/dev/null 2>&1 && return 0 || return 1
    else
        echo "$status_json" | grep -q '"running"[[:space:]]*:[[:space:]]*true' && return 0 || return 1
    fi
}

# Helper: Stop all running node processes with verification
stop_all_processes() {
    local timeout="${1:-15}"
    local use_sudo=""

    step "Stopping any running node processes..."

    # Determine if we need sudo (processes may be owned by root)
    if [[ $(id -u) -ne 0 ]]; then
        if pgrep -f "pchaind|cosmovisor|push-validator" >/dev/null 2>&1; then
            use_sudo="sudo"
        fi
    fi

    # 1. Try graceful stop via manager first
    if [[ -x "$MANAGER_BIN" ]]; then
        $use_sudo "$MANAGER_BIN" stop < /dev/null >/dev/null 2>&1 || true
        sleep 2
    fi

    # 2. Kill push-validator processes (logs, etc.)
    $use_sudo pkill -f "push-validator" 2>/dev/null || true

    # 3. Kill cosmovisor processes
    $use_sudo pkill -f "cosmovisor.*run" 2>/dev/null || true

    # 4. Kill pchaind processes
    $use_sudo pkill -f "pchaind.*start" 2>/dev/null || true

    # 5. Wait and verify all processes stopped
    local elapsed=0
    while [[ $elapsed -lt $timeout ]]; do
        if ! pgrep -f "cosmovisor.*run" >/dev/null 2>&1 && \
           ! pgrep -f "pchaind.*start" >/dev/null 2>&1 && \
           ! pgrep -f "push-validator" >/dev/null 2>&1; then
            ok "All processes stopped"
            return 0
        fi
        sleep 1
        ((elapsed++))
    done

    # 6. Force kill if still running
    verbose "Processes still running, force killing..."
    $use_sudo pkill -9 -f "push-validator" 2>/dev/null || true
    $use_sudo pkill -9 -f "cosmovisor.*run" 2>/dev/null || true
    $use_sudo pkill -9 -f "pchaind.*start" 2>/dev/null || true
    sleep 1

    # 7. Final check
    if pgrep -f "cosmovisor.*run" >/dev/null 2>&1 || \
       pgrep -f "pchaind.*start" >/dev/null 2>&1; then
        err "Failed to stop all processes"
        return 1
    fi

    ok "All processes stopped"
    return 0
}

# Helper: Check if any node processes are running (cosmovisor, pchaind, or push-validator)
any_node_running() {
    pgrep -f "cosmovisor.*run" >/dev/null 2>&1 && return 0
    pgrep -f "pchaind.*start" >/dev/null 2>&1 && return 0
    pgrep -f "push-validator" >/dev/null 2>&1 && return 0
    # Also check via manager status if available
    if [[ -x "$MANAGER_BIN" ]]; then
        node_running && return 0
    fi
    return 1
}

# Helper: Check if current node consensus key already exists in validator set
node_is_validator() {
    local result
    if ! result=$("$MANAGER_BIN" register-validator --check-only --output json < /dev/null 2>/dev/null); then
        return 1
    fi
    if command -v jq >/dev/null 2>&1; then
        local flag
        flag=$(echo "$result" | jq -r '.registered // false' 2>/dev/null || echo "false")
        [[ "$flag" == "true" ]] && return 0 || return 1
    else
        echo "$result" | grep -q '"registered"[[:space:]]*:[[:space:]]*true' && return 0 || return 1
    fi
}

# Helper: Print useful commands
print_useful_cmds() {
    echo
    echo "Useful commands:"
    echo "  push-validator status          # Check node status"
    echo "  push-validator logs            # View logs"
    echo "  push-validator stop            # Stop the node"
    echo "  push-validator restart         # Restart the node"
    echo "  push-validator register-validator  # Register as validator"
    echo
}

# Helper: Prompt for user confirmation
prompt_yes_no() {
    local prompt="$1"
    local default="${2:-n}"
    local response

    if [[ "$default" == "y" ]]; then
        echo -n "$prompt [Y/n]: "
    else
        echo -n "$prompt [y/N]: "
    fi

    read -r response
    response=${response:-$default}

    case "$response" in
        [yY][eE][sS]|[yY]) return 0 ;;
        *) return 1 ;;
    esac
}

# Helper: Update shell profile with Go PATH
update_shell_profile() {
    local go_install_dir="$1"
    local profile_updated=0

    # Detect shell and profile files
    local shell_name="${SHELL##*/}"
    local profile_files=()

    case "$shell_name" in
        bash)
            profile_files=("$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile")
            ;;
        zsh)
            profile_files=("$HOME/.zshrc" "$HOME/.zprofile")
            ;;
        *)
            profile_files=("$HOME/.profile" "$HOME/.bashrc")
            ;;
    esac

    local go_path_line="export PATH=\"$go_install_dir/bin:\$PATH\""

    for profile in "${profile_files[@]}"; do
        if [[ -f "$profile" ]]; then
            # Check if Go path already exists
            if ! grep -q "$go_install_dir/bin" "$profile" 2>/dev/null; then
                echo "" >> "$profile"
                echo "# Added by Push Chain installer" >> "$profile"
                echo "$go_path_line" >> "$profile"
                profile_updated=1
                verbose "Updated $profile with Go PATH"
                break
            else
                verbose "Go PATH already in $profile"
                profile_updated=1  # Mark as updated since PATH already exists
                break
            fi
        fi
    done

    # Create .profile if no profile exists
    if [[ $profile_updated -eq 0 ]] && [[ ${#profile_files[@]} -gt 0 ]]; then
        local default_profile="${profile_files[0]}"
        echo "" >> "$default_profile"
        echo "# Added by Push Chain installer" >> "$default_profile"
        echo "$go_path_line" >> "$default_profile"
        verbose "Created $default_profile with Go PATH"
    fi

    # Export for current session
    export PATH="$go_install_dir/bin:$PATH"
}

# Helper: Install Go automatically
install_go() {
    local go_version="1.23.3"
    local arch
    local os="linux"
    local download_url
    local install_dir
    local use_sudo=0
    local temp_dir

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        armv7l)
            arch="armv6l"
            ;;
        *)
            err "Unsupported architecture: $(uname -m)"
            return 1
            ;;
    esac

    # Detect OS (already set to linux, but keeping for future macOS support)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        os="darwin"
    fi

    download_url="https://go.dev/dl/go${go_version}.${os}-${arch}.tar.gz"

    # Determine installation directory and sudo requirement
    if [[ -w "/usr/local" ]]; then
        install_dir="/usr/local"
        use_sudo=0
    elif command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
        install_dir="/usr/local"
        use_sudo=1
    else
        # Install to user's home directory
        install_dir="$HOME/.local"
        use_sudo=0
        mkdir -p "$install_dir"
    fi

    phase "Installing Go ${go_version}"

    # Check if Go already exists at target location
    if [[ -d "$install_dir/go" ]]; then
        step "Backing up existing Go installation"
        if [[ $use_sudo -eq 1 ]]; then
            sudo mv "$install_dir/go" "$install_dir/go.backup.$(date +%s)" || true
        else
            mv "$install_dir/go" "$install_dir/go.backup.$(date +%s)" || true
        fi
    fi

    # Create temp directory for download
    temp_dir=$(mktemp -d)
    trap "rm -rf '$temp_dir'" EXIT

    step "Downloading Go ${go_version} for ${os}/${arch}"
    if ! download_with_progress "$download_url" "$temp_dir/go.tar.gz"; then
        err "Failed to download Go"
        return 1
    fi

    step "Extracting Go to $install_dir"
    if [[ $use_sudo -eq 1 ]]; then
        sudo tar -C "$install_dir" -xzf "$temp_dir/go.tar.gz" || {
            err "Failed to extract Go"
            return 1
        }
    else
        tar -C "$install_dir" -xzf "$temp_dir/go.tar.gz" || {
            err "Failed to extract Go"
            return 1
        }
    fi

    # Update PATH for current session and shell profile
    step "Updating PATH environment"
    update_shell_profile "$install_dir/go"

    # Verify installation
    if "$install_dir/go/bin/go" version < /dev/null >/dev/null 2>&1; then
        local installed_version
        installed_version=$("$install_dir/go/bin/go" version < /dev/null | awk '{print $3}')
        ok "Go installed successfully: $installed_version"
        echo
        echo -e "${GREEN}Go has been installed to: $install_dir/go${NC}"
        echo -e "${YELLOW}Note: You may need to restart your shell or run: source ~/.bashrc${NC}"
        echo
        trap - EXIT  # Clear the EXIT trap before returning
        return 0
    else
        err "Go installation verification failed"
        trap - EXIT  # Clear the EXIT trap before returning
        return 1
    fi
}

# Helper: Download pchaind binary from GitHub releases
download_pchaind() {
    local os arch version download_url checksum_url temp_dir filename ver_num

    # Detect OS
    case "$(uname -s)" in
        Linux) os="linux" ;;
        Darwin) os="darwin" ;;
        *)
            verbose "Unsupported OS for pre-built binary: $(uname -s)"
            return 1
            ;;
    esac

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)
            verbose "Unsupported architecture for pre-built binary: $(uname -m)"
            return 1
            ;;
    esac

    # Get version (use PCHAIN_REF if it looks like a version tag, otherwise fetch latest stable release)
    if [[ "$PCHAIN_REF" =~ ^v[0-9] ]]; then
        version="$PCHAIN_REF"
        step "Using specified version: $version"
    else
        step "Fetching latest release version"
        if command -v curl >/dev/null 2>&1; then
            # Only use /releases/latest (stable releases, not pre-releases)
            version=$(curl -sL https://api.github.com/repos/pushchain/push-chain-node/releases/latest 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
        fi
        if [[ -z "$version" ]]; then
            verbose "Could not determine latest release version (no stable release found)"
            return 1
        fi
        verbose "Latest release: $version"
    fi

    # Strip 'v' prefix for filename (v0.0.1 -> 0.0.1)
    ver_num="${version#v}"

    # Build download URL
    # Format: push-chain_<version>_<os>_<arch>.tar.gz
    filename="push-chain_${ver_num}_${os}_${arch}.tar.gz"
    download_url="https://github.com/pushchain/push-chain-node/releases/download/${version}/${filename}"
    checksum_url="${download_url}.sha256"

    step "Downloading pchaind ${version} for ${os}/${arch}"
    verbose "URL: $download_url"

    # Create temp directory
    temp_dir=$(mktemp -d)
    # Use a subshell trap to clean up temp dir
    (
        trap "rm -rf '$temp_dir'" EXIT

        # Download tarball with Go-style progress bar
        if ! download_with_progress "$download_url" "$temp_dir/$filename"; then
            verbose "Download failed"
            exit 1
        fi

        # Verify file was downloaded and has content
        if [[ ! -s "$temp_dir/$filename" ]]; then
            verbose "Downloaded file is empty or missing"
            exit 1
        fi

        # Verify checksum (optional but recommended)
        if curl -sL -o "$temp_dir/checksum.sha256" "$checksum_url" 2>/dev/null && [[ -s "$temp_dir/checksum.sha256" ]]; then
            step "Verifying checksum"
            local expected_sum actual_sum
            expected_sum=$(cat "$temp_dir/checksum.sha256" | awk '{print $1}')
            if command -v sha256sum >/dev/null 2>&1; then
                actual_sum=$(sha256sum "$temp_dir/$filename" | awk '{print $1}')
            elif command -v shasum >/dev/null 2>&1; then
                actual_sum=$(shasum -a 256 "$temp_dir/$filename" | awk '{print $1}')
            fi
            if [[ -n "$actual_sum" && -n "$expected_sum" ]]; then
                if [[ "$expected_sum" != "$actual_sum" ]]; then
                    err "Checksum mismatch! Expected: $expected_sum, Got: $actual_sum"
                    exit 1
                fi
                ok "Checksum verified"
            fi
        else
            verbose "Checksum file not available, skipping verification"
        fi

        # Extract tarball
        step "Extracting binary"
        if ! tar -xzf "$temp_dir/$filename" -C "$temp_dir" 2>/dev/null; then
            verbose "Failed to extract tarball"
            exit 1
        fi

        # Find pchaind binary in extracted files
        local binary
        binary=$(find "$temp_dir" -name "pchaind" -type f 2>/dev/null | head -1)
        if [[ -z "$binary" || ! -f "$binary" ]]; then
            verbose "pchaind binary not found in archive"
            exit 1
        fi

        # Install binary directly to cosmovisor genesis directory
        mkdir -p "$COSMOVISOR_GENESIS_BIN"
        mkdir -p "$COSMOVISOR_DIR/upgrades"
        rm -f "$COSMOVISOR_GENESIS_BIN/pchaind" 2>/dev/null || true
        cp "$binary" "$COSMOVISOR_GENESIS_BIN/pchaind"
        chmod +x "$COSMOVISOR_GENESIS_BIN/pchaind"

        # Also copy libwasmvm if present (required on macOS)
        local wasmlib
        wasmlib=$(find "$temp_dir" -name "libwasmvm.dylib" -type f 2>/dev/null | head -1)
        if [[ -n "$wasmlib" && -f "$wasmlib" ]]; then
            rm -f "$COSMOVISOR_GENESIS_BIN/libwasmvm.dylib" 2>/dev/null || true
            cp "$wasmlib" "$COSMOVISOR_GENESIS_BIN/libwasmvm.dylib"
        fi

        exit 0
    )
    local result=$?

    # Clean up temp dir (in case subshell didn't)
    rm -rf "$temp_dir" 2>/dev/null || true

    if [[ $result -eq 0 ]]; then
        # Verify installation
        if [[ -x "$COSMOVISOR_GENESIS_BIN/pchaind" ]]; then
            local installed_version
            installed_version=$("$COSMOVISOR_GENESIS_BIN/pchaind" version < /dev/null 2>&1 | head -1 || echo "")
            if [[ -n "$installed_version" ]]; then
                ok "Installed pchaind ($installed_version)"
            else
                ok "Installed pchaind ${version}"
            fi
            return 0
        fi
    fi

    return 1
}

# Helper: Download push-validator binary from GitHub releases
download_push_validator() {
    local os arch version download_url checksum_url temp_dir filename ver_num

    # Detect OS
    case "$(uname -s)" in
        Linux) os="linux" ;;
        Darwin) os="darwin" ;;
        *)
            verbose "Unsupported OS for pre-built binary: $(uname -s)"
            return 1
            ;;
    esac

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)
            verbose "Unsupported architecture for pre-built binary: $(uname -m)"
            return 1
            ;;
    esac

    # Get version (use PNM_REF if it looks like a version tag, otherwise fetch latest release)
    if [[ "$PNM_REF" =~ ^v[0-9] ]]; then
        version="$PNM_REF"
        step "Using specified version: $version"
    else
        step "Fetching latest release version"
        if command -v curl >/dev/null 2>&1; then
            version=$(curl -sL https://api.github.com/repos/pushchain/push-validator-cli/releases/latest 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
        fi
        if [[ -z "$version" ]]; then
            verbose "Could not determine latest release version (no stable release found)"
            return 1
        fi
        verbose "Latest release: $version"
    fi

    # Strip 'v' prefix for filename (v0.1.0 -> 0.1.0)
    ver_num="${version#v}"

    # Build download URL
    # Format: push-validator_<version>_<os>_<arch>.tar.gz
    filename="push-validator_${ver_num}_${os}_${arch}.tar.gz"
    download_url="https://github.com/pushchain/push-validator-cli/releases/download/${version}/${filename}"
    checksum_url="https://github.com/pushchain/push-validator-cli/releases/download/${version}/checksums.txt"

    step "Downloading push-validator ${version} for ${os}/${arch}"
    verbose "URL: $download_url"

    # Create temp directory
    temp_dir=$(mktemp -d)
    # Use a subshell trap to clean up temp dir
    (
        trap "rm -rf '$temp_dir'" EXIT

        # Download tarball with Go-style progress bar
        if ! download_with_progress "$download_url" "$temp_dir/$filename"; then
            verbose "Download failed"
            exit 1
        fi

        # Verify file was downloaded and has content
        if [[ ! -s "$temp_dir/$filename" ]]; then
            verbose "Downloaded file is empty or missing"
            exit 1
        fi

        # Verify checksum
        if curl -sL -o "$temp_dir/checksums.txt" "$checksum_url" 2>/dev/null && [[ -s "$temp_dir/checksums.txt" ]]; then
            step "Verifying checksum"
            local expected_sum actual_sum
            # Extract checksum for our specific file from checksums.txt
            expected_sum=$(grep "$filename" "$temp_dir/checksums.txt" 2>/dev/null | awk '{print $1}')
            if command -v sha256sum >/dev/null 2>&1; then
                actual_sum=$(sha256sum "$temp_dir/$filename" | awk '{print $1}')
            elif command -v shasum >/dev/null 2>&1; then
                actual_sum=$(shasum -a 256 "$temp_dir/$filename" | awk '{print $1}')
            fi
            if [[ -n "$actual_sum" && -n "$expected_sum" ]]; then
                if [[ "$expected_sum" != "$actual_sum" ]]; then
                    err "Checksum mismatch! Expected: $expected_sum, Got: $actual_sum"
                    exit 1
                fi
                ok "Checksum verified"
            fi
        else
            verbose "Checksum file not available, skipping verification"
        fi

        # Extract tarball
        step "Extracting binary"
        if ! tar -xzf "$temp_dir/$filename" -C "$temp_dir" 2>/dev/null; then
            verbose "Failed to extract tarball"
            exit 1
        fi

        # Find push-validator binary in extracted files
        local binary
        binary=$(find "$temp_dir" -name "push-validator" -type f 2>/dev/null | head -1)
        if [[ -z "$binary" || ! -f "$binary" ]]; then
            verbose "push-validator binary not found in archive"
            exit 1
        fi

        # Install binary to target location
        mkdir -p "$(dirname "$MANAGER_BIN")"
        rm -f "$MANAGER_BIN" 2>/dev/null || true
        cp "$binary" "$MANAGER_BIN"
        chmod +x "$MANAGER_BIN"

        exit 0
    )
    local result=$?

    # Clean up temp dir (in case subshell didn't)
    rm -rf "$temp_dir" 2>/dev/null || true

    if [[ $result -eq 0 ]]; then
        # Verify installation
        if [[ -x "$MANAGER_BIN" ]]; then
            local installed_version
            installed_version=$("$MANAGER_BIN" version < /dev/null 2>&1 | awk '{print $2}' || echo "")
            if [[ -n "$installed_version" ]]; then
                ok "Installed push-validator ($installed_version)"
            else
                ok "Installed push-validator ${version}"
            fi
            return 0
        fi
    fi

    return 1
}

clean_data_and_preserve_keys() {
    local mode="$1"
    local suffix="${2:-1}"

    local wallet_backup
    local validator_backup

    wallet_backup=$(mktemp -d 2>/dev/null || echo "/tmp/pchain-wallet-backup-$$-$suffix")
    validator_backup=$(mktemp -d 2>/dev/null || echo "/tmp/pchain-validator-backup-$$-$suffix")

    if [[ "$mode" == "initial" ]]; then
        if [[ -d "$HOME_DIR" ]]; then
            step "Backing up wallet keys (your account credentials)"
            mkdir -p "$wallet_backup"
            local backed_wallet=0
            for keyring_dir in "$HOME_DIR"/keyring-*; do
                if [[ -d "$keyring_dir" ]]; then
                    cp -r "$keyring_dir" "$wallet_backup/" 2>/dev/null || true
                    backed_wallet=1
                fi
            done
            if [[ $backed_wallet -eq 1 ]]; then
                ok "Wallets backed up"
            fi
        fi

        if [[ -d "$HOME_DIR/config" ]]; then
            step "Backing up validator keys"
            mkdir -p "$validator_backup"
            cp "$HOME_DIR/config/priv_validator_key.json" "$validator_backup/" 2>/dev/null || true
            cp "$HOME_DIR/config/node_key.json" "$validator_backup/" 2>/dev/null || true
            if [[ -n "$(ls -A "$validator_backup" 2>/dev/null)" ]]; then
                ok "Validator keys backed up"
            fi
        fi
    else
        step "Backing up wallet and validator keys"
        mkdir -p "$wallet_backup" "$validator_backup"
        for keyring_dir in "$HOME_DIR"/keyring-*; do
            if [[ -d "$keyring_dir" ]]; then
                cp -r "$keyring_dir" "$wallet_backup/" 2>/dev/null || true
            fi
        done
        cp "$HOME_DIR/config/priv_validator_key.json" "$validator_backup/" 2>/dev/null || true
        cp "$HOME_DIR/config/node_key.json" "$validator_backup/" 2>/dev/null || true
        ok "Keys backed up to temporary location"
    fi

    if [[ "$mode" == "initial" ]]; then
        step "Removing old installation"
        rm -rf "$ROOT_DIR" 2>/dev/null || true
        rm -rf "$HOME_DIR/data" 2>/dev/null || true
        # Note: snapshot-cache/ is intentionally preserved to avoid re-downloading
        rm -f "$HOME_DIR/pchaind.pid" 2>/dev/null || true
        rm -f "$MANAGER_BIN" 2>/dev/null || true
        rm -f "$INSTALL_BIN_DIR/pchaind" 2>/dev/null || true
        rm -f "$HOME_DIR/.initial_state_sync" 2>/dev/null || true
        rm -f "$HOME_DIR/.snapshot_downloaded" 2>/dev/null || true

        rm -f "$HOME_DIR/config/config.toml" 2>/dev/null || true
        rm -f "$HOME_DIR/config/app.toml" 2>/dev/null || true
        rm -f "$HOME_DIR/config/addrbook.json" 2>/dev/null || true
        rm -f "$HOME_DIR/config/genesis.json" 2>/dev/null || true
        rm -f "$HOME_DIR/config/config.toml."*.bak 2>/dev/null || true
    else
        step "Cleaning all chain data (fixing potential corruption)"
        rm -rf "$HOME_DIR/data" 2>/dev/null || true
        # Note: snapshot-cache/ is intentionally preserved to avoid re-downloading
        rm -f "$HOME_DIR/.initial_state_sync" 2>/dev/null || true
        rm -f "$HOME_DIR/.snapshot_downloaded" 2>/dev/null || true
        mkdir -p "$HOME_DIR/data"
        echo '{"height":"0","round":0,"step":0}' > "$HOME_DIR/data/priv_validator_state.json"
    fi

    if [[ "$mode" == "initial" ]]; then
        if [[ -d "$wallet_backup" && -n "$(ls -A "$wallet_backup" 2>/dev/null)" ]]; then
            step "Restoring wallets"
            mkdir -p "$HOME_DIR"
            cp -r "$wallet_backup"/. "$HOME_DIR/" 2>/dev/null || true
            ok "Wallets restored"
        fi
        if [[ -d "$validator_backup" && -n "$(ls -A "$validator_backup" 2>/dev/null)" ]]; then
            step "Restoring validator keys"
            mkdir -p "$HOME_DIR/config"
            cp -r "$validator_backup"/. "$HOME_DIR/config/" 2>/dev/null || true
            ok "Validator keys restored"
        fi
    else
        step "Restoring wallet and validator keys"
        mkdir -p "$HOME_DIR" "$HOME_DIR/config"
        cp -r "$wallet_backup"/. "$HOME_DIR/" 2>/dev/null || true
        cp "$validator_backup"/priv_validator_key.json "$HOME_DIR/config/" 2>/dev/null || true
        cp "$validator_backup"/node_key.json "$HOME_DIR/config/" 2>/dev/null || true
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$HOME_DIR/.initial_state_sync"
        ok "Keys restored successfully"
    fi

    rm -rf "$wallet_backup" "$validator_backup" 2>/dev/null || true

    if [[ "$mode" == "retry" ]]; then
        ok "Data cleaned, ready for fresh sync"
    fi
}

# Phase tracking with timing
INSTALL_START_TIME=$(date +%s)
PHASE_NUM=0
TOTAL_PHASES=6  # Will be adjusted based on what's needed
PHASE_START_TIME=0
next_phase() {
    ((++PHASE_NUM))  # Use pre-increment to avoid returning 0 with set -e
    PHASE_START_TIME=$(date +%s)
    phase "[$PHASE_NUM/$TOTAL_PHASES] $1"
}

# Script location (works when piped or invoked directly)
if [ -n "${BASH_SOURCE+x}" ]; then SCRIPT_SOURCE="${BASH_SOURCE[0]}"; else SCRIPT_SOURCE="$0"; fi
SELF_DIR="$(cd -- "$(dirname -- "$SCRIPT_SOURCE")" >/dev/null 2>&1 && pwd -P || pwd)"

# Defaults (overridable via env)
MONIKER="${MONIKER:-push-validator}"
GENESIS_DOMAIN="${GENESIS_DOMAIN:-rpc-testnet-donut-node1.push.org}"
KEYRING_BACKEND="${KEYRING_BACKEND:-test}"
CHAIN_ID="${CHAIN_ID:-push_42101-1}"
SNAPSHOT_URL="${SNAPSHOT_URL:-https://snapshots.donut.push.org}"
RESET_DATA="${RESET_DATA:-yes}"
AUTO_START="${AUTO_START:-yes}"
PNM_REF="${PNM_REF:-main}"
PCHAIN_REF="${PCHAIN_REF:-main}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
PREFIX="${PREFIX:-}"

# Flags
USE_LOCAL="no"
LOCAL_REPO=""
PCHAIND="${PCHAIND:-}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-start) AUTO_START="no"; shift ;;
    --start) AUTO_START="yes"; shift ;;
    --no-reset) RESET_DATA="no"; shift ;;
    --reset) RESET_DATA="yes"; shift ;;
    --verbose) VERBOSE="yes"; shift ;;
    --no-color) NO_COLOR="1"; shift ;;
    --bin-dir) BIN_DIR="$2"; shift 2 ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    --moniker) MONIKER="$2"; shift 2 ;;
    --genesis) GENESIS_DOMAIN="$2"; shift 2 ;;
    --keyring) KEYRING_BACKEND="$2"; shift 2 ;;
    --chain-id) CHAIN_ID="$2"; shift 2 ;;
    --snapshot-url) SNAPSHOT_URL="$2"; shift 2 ;;
    --pchaind-ref) PCHAIN_REF="$2"; shift 2 ;;  # deprecated, use --pchain-ref
    --pchain-ref) PCHAIN_REF="$2"; shift 2 ;;
    --dev|--use-local) USE_LOCAL="yes"; shift ;;
    --dev-dir|--local-repo) LOCAL_REPO="$2"; shift 2 ;;
    --help)
      echo "Push Validator Manager (Go) - Installer"
      echo
      echo "Usage: bash install.sh [OPTIONS]"
      echo
      echo "By default, pre-built binaries are downloaded from GitHub Releases."
      echo "Use --dev for development builds from source."
      echo
      echo "Installation Options:"
      echo "  --dev                Build from script's directory (dev mode)"
      echo "  --dev-dir DIR        Build from specific directory (dev mode)"
      echo "  --bin-dir DIR        Install binaries to DIR (default: ~/.local/bin)"
      echo "  --prefix DIR         Use DIR as installation prefix (sets data dir)"
      echo
      echo "Node Configuration:"
      echo "  --moniker NAME       Set validator moniker (default: push-validator)"
      echo "  --chain-id ID        Set chain ID (default: push_42101-1)"
      echo "  --genesis DOMAIN     Genesis domain (default: rpc-testnet-donut-node1.push.org)"
      echo "  --snapshot-url URL   Snapshot download URL (default: https://snapshots.donut.push.org)"
      echo "  --keyring BACKEND    Keyring backend (default: test)"
      echo
      echo "Version Options:"
      echo "  --pchain-ref REF     Version/ref for pchaind binary (default: latest release)"
      echo "  --pchaind-ref REF    (deprecated) Alias for --pchain-ref"
      echo
      echo "Behavior Options:"
      echo "  --reset              Reset all data (default)"
      echo "  --no-reset           Keep existing data"
      echo "  --start              Start node after installation (default)"
      echo "  --no-start           Install only, don't start"
      echo
      echo "Output Options:"
      echo "  --verbose            Show verbose output"
      echo "  --no-color           Disable colored output"
      echo
      echo "Environment Variables:"
      echo "  NO_COLOR             Set to disable colors"
      echo "  VERBOSE              Set to 'yes' for verbose output"
      echo "  PNM_REF              Version/ref for push-validator (default: latest release)"
      echo "  PCHAIN_REF           Version/ref for pchaind (default: latest release)"
      echo
      echo "Examples:"
      echo "  bash install.sh                          # Download latest binaries"
      echo "  bash install.sh --dev --verbose          # Build from local repo"
      echo "  bash install.sh --no-reset --no-start    # Install only"
      echo "  PNM_REF=v0.1.1 bash install.sh           # Download specific version"
      exit 0
      ;;
    *) err "Unknown flag: $1 (use --help for usage)"; exit 2 ;;
  esac
done

# Paths
if [[ -n "$PREFIX" ]]; then
  ROOT_DIR="$PREFIX/share/push-validator"
  INSTALL_BIN_DIR="$PREFIX/bin"  # --prefix overrides BIN_DIR for relocatable installs
  HOME_DIR="${HOME_DIR:-$PREFIX/data}"
else
  if [[ -n "${XDG_DATA_HOME:-}" ]]; then ROOT_DIR="$XDG_DATA_HOME/push-validator"; else ROOT_DIR="$HOME/.local/share/push-validator"; fi
  INSTALL_BIN_DIR="$BIN_DIR"
  HOME_DIR="${HOME_DIR:-$HOME/.pchain}"
fi

# Cosmovisor directory structure (binary goes here directly)
COSMOVISOR_DIR="$HOME_DIR/cosmovisor"
COSMOVISOR_GENESIS_BIN="$COSMOVISOR_DIR/genesis/bin"

REPO_DIR="$ROOT_DIR/repo"
MANAGER_BIN="$INSTALL_BIN_DIR/push-validator"

# Detect what phases are needed BEFORE creating directories
HAS_RUNNING_NODE="no"
HAS_EXISTING_INSTALL="no"

# Check if node is running (cosmovisor, pchaind, or via manager status)
if pgrep -f "cosmovisor.*run" >/dev/null 2>&1; then
  HAS_RUNNING_NODE="yes"
elif pgrep -f "pchaind.*start" >/dev/null 2>&1; then
  HAS_RUNNING_NODE="yes"
elif [[ -x "$MANAGER_BIN" ]] && command -v "$MANAGER_BIN" >/dev/null 2>&1; then
  # Manager exists, check if node is actually running via status
  STATUS_JSON=$("$MANAGER_BIN" status --output json < /dev/null 2>/dev/null || echo "{}")
  if echo "$STATUS_JSON" | grep -q '"running"[[:space:]]*:[[:space:]]*true'; then
    HAS_RUNNING_NODE="yes"
  fi
fi

# Check if installation exists (check for actual installation artifacts, not just config)
if [[ -d "$ROOT_DIR" ]] || [[ -x "$MANAGER_BIN" ]]; then
  HAS_EXISTING_INSTALL="yes"
elif [[ -d "$HOME_DIR/data" ]] && [[ -n "$(ls -A "$HOME_DIR/data" 2>/dev/null)" ]]; then
  # Only count as existing if data directory has content
  HAS_EXISTING_INSTALL="yes"
fi

mkdir -p "$ROOT_DIR" "$INSTALL_BIN_DIR"

verbose "Installation paths:"
verbose "  Root dir: $ROOT_DIR"
verbose "  Bin dir: $INSTALL_BIN_DIR"
verbose "  Home dir: $HOME_DIR"

# Check git (only needed for dev mode: --use-local or --local-repo)
if [[ "$USE_LOCAL" = "yes" || -n "$LOCAL_REPO" ]]; then
  if ! command -v git >/dev/null 2>&1; then
    err "Missing dependency: git"
    echo
    echo "Git is required for local development builds (--use-local)."
    echo
    if [[ "$OSTYPE" == "darwin"* ]]; then
      echo "Install with: brew install git"
      echo "Or download from: https://git-scm.com/downloads"
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
      if command -v apt-get >/dev/null 2>&1; then
        echo "Install with: sudo apt-get install git"
      elif command -v yum >/dev/null 2>&1; then
        echo "Install with: sudo yum install git"
      else
        echo "Install using your package manager or download from: https://git-scm.com/downloads"
      fi
    else
      echo "Download from: https://git-scm.com/downloads"
    fi
    exit 1
  fi
fi

# Check Go with automatic installation option
# Go is required for:
# - Dev mode: building push-validator from local repo
# - Always: installing cosmovisor (go install)
GO_NEEDS_INSTALL=0
GO_NEEDS_UPGRADE=0

if ! command -v go >/dev/null 2>&1; then
  GO_NEEDS_INSTALL=1
  warn "Go is not installed"
else
  # Validate Go version (requires 1.23+ for cosmovisor)
  GO_VERSION=$(go version < /dev/null | awk '{print $3}' | sed 's/go//')
  GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
  GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

  if [[ "$GO_MAJOR" -lt 1 ]] || [[ "$GO_MAJOR" -eq 1 && "$GO_MINOR" -lt 23 ]]; then
    GO_NEEDS_UPGRADE=1
    warn "Go version too old: $GO_VERSION (need 1.23+)"
  else
    verbose "Go version check passed: $GO_VERSION"
  fi
fi

# Handle Go installation/upgrade if needed
if [[ $GO_NEEDS_INSTALL -eq 1 ]] || [[ $GO_NEEDS_UPGRADE -eq 1 ]]; then
  echo
  if [[ $GO_NEEDS_INSTALL -eq 1 ]]; then
    echo -e "${BOLD}Go 1.23 or higher is required to install Cosmovisor (automatic upgrades).${NC}"
  else
    echo -e "${BOLD}Your Go version is too old. Go 1.23+ is required.${NC}"
  fi
  echo

  # Check if we're in non-interactive mode (CI/automation)
  if [[ ! -t 0 ]] || [[ "${CI:-false}" == "true" ]] || [[ "${NON_INTERACTIVE:-false}" == "true" ]]; then
    echo "Running in non-interactive mode. Attempting automatic Go installation..."
    if install_go; then
      # Re-check Go after installation
      if ! command -v go >/dev/null 2>&1; then
        # Try with the newly installed path
        if [[ -f "$HOME/.local/go/bin/go" ]]; then
          export PATH="$HOME/.local/go/bin:$PATH"
        elif [[ -f "/usr/local/go/bin/go" ]]; then
          export PATH="/usr/local/go/bin:$PATH"
        fi
      fi

      # Verify installation worked
      if command -v go >/dev/null 2>&1; then
        GO_VERSION=$(go version < /dev/null | awk '{print $3}' | sed 's/go//')
        verbose "Go successfully installed: $GO_VERSION"
      else
        err "Go installation completed but 'go' command not found in PATH"
        echo "Please add Go to your PATH and run the installer again."
        exit 1
      fi
    else
      err "Automatic Go installation failed"
      echo
      echo "Please install Go manually:"
      echo "  • Download from: https://go.dev/dl/"
      echo "  • Or use your package manager (ensure version 1.23+)"
      exit 1
    fi
  else
    # Interactive mode - prompt user
    if prompt_yes_no "Would you like to install Go 1.23.3 automatically?" "y"; then
      if install_go; then
        # Re-check Go after installation
        if ! command -v go >/dev/null 2>&1; then
          # Try with the newly installed path
          if [[ -f "$HOME/.local/go/bin/go" ]]; then
            export PATH="$HOME/.local/go/bin:$PATH"
          elif [[ -f "/usr/local/go/bin/go" ]]; then
            export PATH="/usr/local/go/bin:$PATH"
          fi
        fi

        # Verify installation worked
        if command -v go >/dev/null 2>&1; then
          GO_VERSION=$(go version < /dev/null | awk '{print $3}' | sed 's/go//')
          verbose "Go successfully installed: $GO_VERSION"
        else
          err "Go installation completed but 'go' command not found in PATH"
          echo "Please add Go to your PATH and run the installer again."
          exit 1
        fi
      else
        err "Go installation failed"
        echo
        echo "Please install Go manually and run this installer again."
        echo "Download from: https://go.dev/dl/"
        exit 1
      fi
    else
      echo
      echo "Manual installation required. Please install Go 1.23+ from:"
      echo "  • Download: https://go.dev/dl/"
      if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "  • Using Homebrew: brew install go"
      elif command -v apt-get >/dev/null 2>&1; then
        echo "  • Using apt: sudo apt-get install golang-go (check version)"
      elif command -v yum >/dev/null 2>&1; then
        echo "  • Using yum: sudo yum install golang (check version)"
      fi
      echo
      echo "After installing Go, run this installer again."
      exit 1
    fi
  fi
fi

# Ensure Go is accessible after installation (refresh command cache)
if ! command -v go >/dev/null 2>&1; then
  if [[ -f "/usr/local/go/bin/go" ]]; then
    export PATH="/usr/local/go/bin:$PATH"
  elif [[ -f "$HOME/.local/go/bin/go" ]]; then
    export PATH="$HOME/.local/go/bin:$PATH"
  fi
  hash -r 2>/dev/null || true  # Refresh bash command cache
fi

# Optional dependencies (warn if missing, fallbacks exist)
if ! command -v jq >/dev/null 2>&1; then
  warn "jq not found; JSON parsing will be less robust (using grep fallback)"
fi
TO_CMD=$(timeout_cmd)
if [[ -z "$TO_CMD" ]]; then
  warn "timeout/gtimeout not found; RPC checks may block longer than expected"
fi

# Store environment info (will print after manager is built)
OS_NAME=$(uname -s | tr '[:upper:]' '[:lower:]')
OS_ARCH=$(uname -m)
GO_VER=$(go version < /dev/null | awk '{print $3}' | sed 's/go//')
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

# Calculate total phases needed (detection already done above before mkdir)
TOTAL_PHASES=5  # Base: Install Manager, Build Chain, Download Snapshot, Init, Start
if [[ "$HAS_RUNNING_NODE" = "yes" ]]; then
  ((TOTAL_PHASES++))  # Add stopping phase
fi
if [[ "$RESET_DATA" = "yes" ]] && [[ "$HAS_EXISTING_INSTALL" = "yes" ]]; then
  ((TOTAL_PHASES++))  # Add cleaning phase
fi

verbose "Phases needed: $TOTAL_PHASES (running=$HAS_RUNNING_NODE, existing=$HAS_EXISTING_INSTALL)"

# Print installation banner
echo
echo "╔═══════════════════════════════════════════════════════════════╗"
echo "║          PUSH VALIDATOR MANAGER INSTALLATION                  ║"
echo "╚═══════════════════════════════════════════════════════════════╝"
echo

# Stop any running processes first (only if needed)
if [[ "$HAS_RUNNING_NODE" = "yes" ]]; then
  next_phase "Stopping Validator Processes"
  if ! stop_all_processes 15; then
    err "Could not stop running processes. Please stop them manually and retry."
    exit 1
  fi
else
  verbose "No running processes to stop (skipped)"
fi

# Clean install: remove all previous installation artifacts (preserve wallets and validator keys)
NEED_INIT="no"  # Track if we need to force init
if [[ "$RESET_DATA" = "yes" ]] && [[ "$HAS_EXISTING_INSTALL" = "yes" ]]; then
  next_phase "Cleaning Installation"
  clean_data_and_preserve_keys "initial" "init"
  NEED_INIT="yes"  # Force init after full reset
  ok "Clean installation ready"
elif [[ "$RESET_DATA" = "yes" ]]; then
  verbose "Fresh installation detected (skipped cleanup)"
  NEED_INIT="yes"  # Force init for fresh installations too
else
  verbose "Skipping data reset (--no-reset)"
fi

next_phase "Installing Validator Manager"
verbose "Target directory: $ROOT_DIR"

# Determine installation method
if [[ "$USE_LOCAL" = "yes" || -n "$LOCAL_REPO" ]]; then
  # Dev mode: build from local repository
  if [[ -n "$LOCAL_REPO" ]]; then REPO_DIR="$(cd "$LOCAL_REPO" && pwd -P)"; else REPO_DIR="$(cd "$SELF_DIR" && pwd -P)"; fi
  step "Using local repository: $REPO_DIR"
  if [[ ! -f "$REPO_DIR/go.mod" ]]; then
    err "Expected Go module not found at: $REPO_DIR/go.mod"; exit 1
  fi

  # Check if already up-to-date (idempotent install)
  SKIP_BUILD=no
  if [[ -x "$MANAGER_BIN" ]]; then
    CURRENT_COMMIT=$(cd "$REPO_DIR" && git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    # Extract commit from version output (format: "push-validator vX.Y.Z (1f599bd) built ...")
    INSTALLED_COMMIT=$("$MANAGER_BIN" version < /dev/null 2>/dev/null | sed -n 's/.*(\([0-9a-f]\{7,\}\)).*/\1/p')
    # Only skip build if both are valid hex commits and match
    if [[ "$CURRENT_COMMIT" =~ ^[0-9a-f]+$ ]] && [[ "$INSTALLED_COMMIT" =~ ^[0-9a-f]+$ ]] && [[ "$CURRENT_COMMIT" == "$INSTALLED_COMMIT" ]]; then
      step "Manager already up-to-date ($CURRENT_COMMIT) - skipped"
      SKIP_BUILD=yes
    fi
  fi

  if [[ "$SKIP_BUILD" = "no" ]]; then
    step "Building Push Validator Manager binary"
    pushd "$REPO_DIR" >/dev/null

    # Build version information
    VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")}
    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
    LDFLAGS="-X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildDate=$BUILD_DATE"

    GOFLAGS="-trimpath" CGO_ENABLED=0 go build -mod=mod -ldflags="$LDFLAGS" -o "$MANAGER_BIN" ./cmd/push-validator
    popd >/dev/null
    chmod +x "$MANAGER_BIN"

    # Compute and display SHA256
    if command -v sha256sum >/dev/null 2>&1; then
      MANAGER_SHA=$(sha256sum "$MANAGER_BIN" 2>/dev/null | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
      MANAGER_SHA=$(shasum -a 256 "$MANAGER_BIN" 2>/dev/null | awk '{print $1}')
    fi
    if [[ -n "$MANAGER_SHA" ]]; then
      SHA_SHORT="${MANAGER_SHA:0:8}...${MANAGER_SHA: -8}"
      ok "Built push-validator (SHA256: $SHA_SHORT)"
    else
      ok "Built push-validator"
    fi
  fi
else
  # Production mode: download pre-built binary from GitHub releases
  if ! download_push_validator; then
    err "Failed to download push-validator binary"
    echo
    echo "Please check your internet connection and try again."
    echo "Or download manually from:"
    echo "  https://github.com/pushchain/push-validator-cli/releases"
    echo
    echo "Alternatively, use --use-local to build from a local repository."
    exit 1
  fi
fi

ok "Manager installed: $MANAGER_BIN"

# Print environment banner now that manager is built
MANAGER_VER_BANNER="dev unknown"
if [[ -x "$MANAGER_BIN" ]]; then
  # Parse full version output: "push-validator v1.0.0 (abc1234) built 2025-01-08"
  MANAGER_FULL=$("$MANAGER_BIN" version < /dev/null 2>/dev/null || echo "unknown")
  if [[ "$MANAGER_FULL" != "unknown" ]]; then
    MANAGER_VER_BANNER=$(echo "$MANAGER_FULL" | awk '{print $2, $3}' | sed 's/[()]//g')
  fi
fi
echo

# Ensure PATH for current session
case ":$PATH:" in *":$INSTALL_BIN_DIR:"*) : ;; *) export PATH="$INSTALL_BIN_DIR:$PATH" ;; esac

# Persist PATH in common shell config files (idempotent - won't add duplicates)
SHELL_CONFIG=""
if [[ -f "$HOME/.zshrc" ]]; then SHELL_CONFIG="$HOME/.zshrc"; elif [[ -f "$HOME/.bashrc" ]]; then SHELL_CONFIG="$HOME/.bashrc"; elif [[ -f "$HOME/.bash_profile" ]]; then SHELL_CONFIG="$HOME/.bash_profile"; fi
if [[ -n "$SHELL_CONFIG" ]]; then
  # Check if PATH already contains this directory in an export statement
  if ! grep -E "^[[:space:]]*export[[:space:]]+PATH=.*$INSTALL_BIN_DIR" "$SHELL_CONFIG" >/dev/null 2>&1; then
    echo "" >> "$SHELL_CONFIG"
    echo "# Push Validator Manager (Go)" >> "$SHELL_CONFIG"
    echo "export PATH=\"$INSTALL_BIN_DIR:\$PATH\"" >> "$SHELL_CONFIG"
  fi
fi

next_phase "Installing Chain Binary"

# Download pre-built binary using push-validator CLI (with progress bar)
VERSION_FLAG=""
if [[ "$PCHAIN_REF" =~ ^v[0-9] ]]; then
  VERSION_FLAG="--version $PCHAIN_REF"
fi

if ! "$MANAGER_BIN" chain install $VERSION_FLAG --home "$HOME_DIR" < /dev/null; then
  err "Failed to download pchaind binary"
  echo
  echo "Please check your internet connection and try again."
  echo "Or download manually from:"
  echo "  https://github.com/pushchain/push-chain-node/releases"
  echo
  exit 1
fi
export PCHAIND="$COSMOVISOR_GENESIS_BIN/pchaind"

verbose "Using built-in WebSocket monitor (no external dependency)"

# Install Cosmovisor for automatic upgrades (pinned to v1.7.1)
step "Installing Cosmovisor for automatic upgrades"
if ! command -v cosmovisor >/dev/null 2>&1; then
  COSMOVISOR_LOG=$(mktemp /tmp/cosmovisor-install-XXXXXX.log)
  go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@v1.7.1 < /dev/null > "$COSMOVISOR_LOG" 2>&1 &
  GO_PID=$!
  # Show spinner while go install runs
  if [[ -t 1 ]]; then
    spin_chars='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    i=0
    while kill -0 "$GO_PID" 2>/dev/null; do
      printf "\r  %s Building cosmovisor from source..." "${spin_chars:i++%${#spin_chars}:1}"
      sleep 0.1 2>/dev/null || sleep 1
    done
    printf "\r\033[K"
  else
    # Non-TTY: just wait silently
    wait "$GO_PID"
  fi
  wait "$GO_PID" 2>/dev/null
  GO_EXIT=$?
  if [[ $GO_EXIT -eq 0 ]]; then
    rm -f "$COSMOVISOR_LOG"
    # Ensure go bin directory is in PATH for current session and future shells
    GO_BIN_DIR="${GOBIN:-${GOPATH:-$HOME/go}/bin}"
    export PATH="$GO_BIN_DIR:$PATH"
    # Persist to shell profile if not already there
    SHELL_CONFIG="${HOME}/.bashrc"
    [[ "${SHELL##*/}" = "zsh" ]] && SHELL_CONFIG="${HOME}/.zshrc"
    if ! grep -q "$GO_BIN_DIR" "$SHELL_CONFIG" 2>/dev/null; then
      echo "" >> "$SHELL_CONFIG"
      echo "# Go bin directory (cosmovisor)" >> "$SHELL_CONFIG"
      echo "export PATH=\"$GO_BIN_DIR:\$PATH\"" >> "$SHELL_CONFIG"
    fi
    ok "Cosmovisor v1.7.1 installed"
  else
    err "Cosmovisor installation failed (see $COSMOVISOR_LOG)"
    exit 1
  fi
else
  ok "Cosmovisor already installed"
fi

if [[ "$AUTO_START" = "yes" ]]; then
  # Download snapshot to cache (separate phase for better progress tracking)
  next_phase "Downloading Snapshot"
  SNAPSHOT_MARKER="$HOME_DIR/.snapshot_downloaded"
  DATA_DIR="$HOME_DIR/data"

  step "Downloading blockchain snapshot (~6-7GB)"
  "$MANAGER_BIN" snapshot download \
    --home "$HOME_DIR" \
    --snapshot-url "$SNAPSHOT_URL" < /dev/null || { err "snapshot download failed"; exit 1; }
  ok "Snapshot cached"

  next_phase "Initializing Node"
  # Initialize if: forced by reset, or config/genesis missing
  if [[ "${NEED_INIT:-no}" = "yes" ]] || [[ ! -f "$HOME_DIR/config/config.toml" ]] || [[ ! -f "$HOME_DIR/config/genesis.json" ]]; then
    step "Configuring node"
    "$MANAGER_BIN" init \
      --moniker "$MONIKER" \
      --home "$HOME_DIR" \
      --chain-id "$CHAIN_ID" \
      --genesis-domain "$GENESIS_DOMAIN" \
      --snapshot-url "$SNAPSHOT_URL" \
      --skip-snapshot \
      --bin "${PCHAIND:-pchaind}" < /dev/null 2>&1 | indent_output || { err "init failed"; exit 1; }
    ok "Node initialized"

    # Optimize config for faster sync
    step "Optimizing node configuration"
    APP_TOML="$HOME_DIR/config/app.toml"
    CONFIG_TOML="$HOME_DIR/config/config.toml"

    if [[ -f "$APP_TOML" ]]; then
      # Increase IAVL cache size for faster sync (default: 781250 -> 2000000)
      sed -i.bak 's/^iavl-cache-size = .*/iavl-cache-size = 2000000/' "$APP_TOML" 2>/dev/null || \
        sed -i '' 's/^iavl-cache-size = .*/iavl-cache-size = 2000000/' "$APP_TOML" 2>/dev/null || true
      rm -f "$APP_TOML.bak" 2>/dev/null || true
    fi

    if [[ -f "$CONFIG_TOML" ]]; then
      # Increase peer limits for more chunk sources
      sed -i.bak 's/^max_num_inbound_peers = .*/max_num_inbound_peers = 100/' "$CONFIG_TOML" 2>/dev/null || \
        sed -i '' 's/^max_num_inbound_peers = .*/max_num_inbound_peers = 100/' "$CONFIG_TOML" 2>/dev/null || true
      sed -i.bak 's/^max_num_outbound_peers = .*/max_num_outbound_peers = 50/' "$CONFIG_TOML" 2>/dev/null || \
        sed -i '' 's/^max_num_outbound_peers = .*/max_num_outbound_peers = 50/' "$CONFIG_TOML" 2>/dev/null || true
      # Increase transfer rates for faster data transfer
      sed -i.bak 's/^send_rate = .*/send_rate = 10240000/' "$CONFIG_TOML" 2>/dev/null || \
        sed -i '' 's/^send_rate = .*/send_rate = 10240000/' "$CONFIG_TOML" 2>/dev/null || true
      sed -i.bak 's/^recv_rate = .*/recv_rate = 10240000/' "$CONFIG_TOML" 2>/dev/null || \
        sed -i '' 's/^recv_rate = .*/recv_rate = 10240000/' "$CONFIG_TOML" 2>/dev/null || true
      # Disable tx indexer during sync for less disk I/O
      sed -i.bak 's/^indexer = .*/indexer = "null"/' "$CONFIG_TOML" 2>/dev/null || \
        sed -i '' 's/^indexer = .*/indexer = "null"/' "$CONFIG_TOML" 2>/dev/null || true
      rm -f "$CONFIG_TOML.bak" 2>/dev/null || true
    fi
    ok "Configuration optimized"
  else
    step "Configuration exists, skipping init"
  fi

  # Extract snapshot from cache to data directory (after init, to avoid being wiped by --overwrite)
  step "Extracting snapshot to data directory"
  "$MANAGER_BIN" snapshot extract \
    --home "$HOME_DIR" \
    --target "$DATA_DIR" \
    --force < /dev/null || { err "snapshot extract failed"; exit 1; }

  # Mark snapshot as downloaded
  echo "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$SNAPSHOT_MARKER"
  ok "Snapshot extracted to data directory"

  next_phase "Starting and Syncing Node"
  MAX_RETRIES=5
  RETRY_COUNT=0
  SYNC_RC=0

  while true; do
    if [[ $RETRY_COUNT -eq 0 ]]; then
      step "Starting Push Chain validator node"
    else
      step "Restarting node (attempt $((RETRY_COUNT + 1))/$((MAX_RETRIES + 1)))"
    fi

    "$MANAGER_BIN" start --no-prompt --home "$HOME_DIR" --bin "${PCHAIND:-pchaind}" < /dev/null 2>&1 | indent_output || { err "start failed"; exit 1; }

    step "Waiting for sync"
    # Stream compact sync until fully synced (monitor prints snapshot/block progress)
    set +e
    "$MANAGER_BIN" sync --compact --window 30 --rpc "http://127.0.0.1:26657" --remote "https://$GENESIS_DOMAIN:443" --skip-final-message < /dev/null
    SYNC_RC=$?
    set -e

    if [[ $SYNC_RC -eq 0 ]]; then
      echo
      sleep 5
      echo -e "  ${GREEN}✓ Sync complete! Node is fully synced.${NC}"
      break
    fi

    if [[ $SYNC_RC -eq 42 ]]; then
      ((RETRY_COUNT++))

      if [[ $RETRY_COUNT -gt $MAX_RETRIES ]]; then
        echo
        err "Sync failed after $MAX_RETRIES retry attempts"
        echo
        echo "The sync process repeatedly got stuck or encountered errors."
        echo
        echo "Common causes:"
        echo "  • Network connectivity issues"
        echo "  • Snapshot corruption (app hash mismatch)"
        echo "  • RPC server temporarily unavailable"
        echo "  • Insufficient peers for sync"
        echo
        echo "Troubleshooting steps:"
        echo "  1. Check network: curl https://$GENESIS_DOMAIN/status"
        echo "  2. Verify peers: push-validator status"
        echo "  3. Check logs: tail -100 $HOME_DIR/logs/pchaind.log"
        echo
        echo "If issues persist:"
        echo "  • Discord: https://discord.com/invite/pushchain"
        echo "  • Support: https://push.org/support/"
        echo
        "$MANAGER_BIN" stop < /dev/null >/dev/null 2>&1 || true
        exit 1
      fi

      warn "Sync stuck or failed. Performing full data reset (attempt $RETRY_COUNT/$MAX_RETRIES)..."
      echo

      step "Stopping node"
      "$MANAGER_BIN" stop < /dev/null >/dev/null 2>&1 || true
      sleep 2
      pkill -x pchaind 2>/dev/null || true
      pkill -x push-validator 2>/dev/null || true
      clean_data_and_preserve_keys "retry" "$RETRY_COUNT"
      echo
      continue
    fi

    warn "Sync monitoring ended with code $SYNC_RC (not a stuck condition, skipping retry)"
    "$MANAGER_BIN" stop < /dev/null >/dev/null 2>&1 || true
    break
  done

  if [[ $SYNC_RC -ne 0 ]]; then
    err "Sync failed with exit code $SYNC_RC"
    exit $SYNC_RC
  fi

  # Detect whether a controlling TTY is available for prompts/log view
  INTERACTIVE="no"
  if [[ -t 0 ]] && [[ -t 1 ]]; then
    INTERACTIVE="yes"
  elif [[ -e /dev/tty ]]; then
    INTERACTIVE="yes"
  fi

  REGISTRATION_STATUS="skipped"

  ALREADY_VALIDATOR="no"
  if node_is_validator; then
    ALREADY_VALIDATOR="yes"
    REGISTRATION_STATUS="already"
  fi

  # Prompt for validator registration if not already registered
  if [[ "$INTERACTIVE" == "yes" ]] && [[ "$ALREADY_VALIDATOR" == "no" ]]; then
    echo
    echo "Next steps to become a validator:"
    echo "1. Get test tokens from: https://faucet.push.org"
    echo "2. Register as validator with the command below"
    echo
  fi

  # Guard registration prompt in non-interactive mode
  if [[ "$ALREADY_VALIDATOR" == "yes" ]]; then
    RESP="N"
  else
    if [[ "$INTERACTIVE" == "yes" ]]; then
      if [[ -e /dev/tty ]]; then
        read -r -p "Register as a validator now? (y/N) " RESP < /dev/tty 2> /dev/tty || true
      else
        read -r -p "Register as a validator now? (y/N) " RESP || true
      fi
    else
      RESP="N"
    fi
  fi
  case "${RESP:-}" in
    [Yy])
      echo
      echo "Push Validator Manager - Registration"
      echo "══════════════════════════════════════"
      # Run registration flow directly (CLI handles prompts and status checks)
      if "$MANAGER_BIN" register-validator; then
        REGISTRATION_STATUS="success"
      else
        REGISTRATION_STATUS="failed"
      fi
      ;;
    *)
      # Ensure clean separation before summary
      echo
      # Only mark as skipped if user declined and was not already a validator
      if [[ "$ALREADY_VALIDATOR" != "yes" ]]; then
        REGISTRATION_STATUS="skipped"
      fi
      ;;
  esac
fi

# Calculate total time for summary
INSTALL_END_TIME=$(date +%s)
TOTAL_TIME=$((INSTALL_END_TIME - ${INSTALL_START_TIME:-$INSTALL_END_TIME}))

# Get node information for unified summary
MANAGER_VER=$("$MANAGER_BIN" version < /dev/null 2>/dev/null | awk '{print $2}' || echo "unknown")
PCHAIND_PATH="${PCHAIND:-pchaind}"
# Extract pchaind version if binary exists
if command -v "$PCHAIND_PATH" >/dev/null 2>&1; then
  CHAIN_VER=$("$PCHAIND_PATH" version < /dev/null 2>/dev/null | head -1 || echo "")
  if [[ -n "$CHAIN_VER" ]]; then
    PCHAIND_VER="$PCHAIND_PATH ($CHAIN_VER)"
  else
    PCHAIND_VER="$PCHAIND_PATH"
  fi
else
  PCHAIND_VER="$PCHAIND_PATH"
fi
RPC_URL="http://127.0.0.1:26657"

# Try to get Node status info
TO_CMD=$(timeout_cmd)
if [[ -n "$TO_CMD" ]]; then
  STATUS_JSON=$($TO_CMD 5 "$MANAGER_BIN" status --output json < /dev/null 2>/dev/null || echo "{}")
else
  STATUS_JSON=$("$MANAGER_BIN" status --output json < /dev/null 2>/dev/null || echo "{}")
fi
if command -v jq >/dev/null 2>&1; then
  NETWORK=$(echo "$STATUS_JSON" | jq -r '.network // .node.network // empty' 2>/dev/null)
  MONIKER=$(echo "$STATUS_JSON" | jq -r '.moniker // .node.moniker // empty' 2>/dev/null)
  SYNCED=$(echo "$STATUS_JSON" | jq -r '.synced // .node.synced // empty' 2>/dev/null)
  PEERS=$(echo "$STATUS_JSON" | jq -r '.peers // .node.peers // empty' 2>/dev/null)
else
  NETWORK=$(echo "$STATUS_JSON" | grep -o '"network"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"network"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
  MONIKER=$(echo "$STATUS_JSON" | grep -o '"moniker"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"moniker"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
  SYNCED=$(echo "$STATUS_JSON" | grep -o '"synced"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*"synced"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
  PEERS=$(echo "$STATUS_JSON" | grep -o '"peers"[[:space:]]*:[[:space:]]*[0-9]*' | sed 's/.*"peers"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/')
fi

# Determine node status indicator
NODE_STATUS_ICON="⚙️ "
if [[ "$SYNCED" == "true" ]]; then
  NODE_STATUS_ICON="✅"
fi

VALIDATOR_STATUS_ICON="❌"
if [[ "$ALREADY_VALIDATOR" == "yes" ]]; then
  VALIDATOR_STATUS_ICON="✅"
fi

echo
echo "╔═══════════════════════════════════════════════════════════════╗"
# Dynamic padding for Installation Complete header
TEXT="INSTALLATION COMPLETE (${TOTAL_TIME}s)"
TEXT_LEN=${#TEXT}
BOX_WIDTH=63
PADDING_LEFT=$(( (BOX_WIDTH - TEXT_LEN) / 2 ))
PADDING_RIGHT=$(( BOX_WIDTH - TEXT_LEN - PADDING_LEFT ))
printf "║%*s%s%*s║\n" $PADDING_LEFT "" "$TEXT" $PADDING_RIGHT ""
echo "╚═══════════════════════════════════════════════════════════════╝"
echo
echo "  📊 Node Status"
echo "     $NODE_STATUS_ICON Synced"
echo "     🌐 Peers:   $PEERS"
if [[ "$ALREADY_VALIDATOR" == "yes" ]]; then
  echo "     $VALIDATOR_STATUS_ICON Validator Registered"
fi
echo
echo "  ⚙️  Configuration"
if [[ -n "$NETWORK" ]]; then
  echo "     Network:  $NETWORK"
fi
if [[ -n "$MONIKER" ]]; then
  echo "     Moniker:  $MONIKER"
fi
echo "     RPC:      $RPC_URL"
echo "     Home:     $HOME_DIR"
echo
echo "  📦 Binaries"
echo "     Manager:  $MANAGER_BIN ($MANAGER_VER)"
echo "     Chain:    $PCHAIND_VER"
echo
echo "  💡 Quick Commands"
echo "     • Check status:    push-validator status"
echo "     • View dashboard:  push-validator dashboard"
echo "     • View logs:       push-validator logs"
echo "     • Stop node:       push-validator stop"
echo "     • Restart node:    push-validator restart"
if [[ "$ALREADY_VALIDATOR" == "no" ]]; then
  echo "     • Register:        push-validator register-validator"
fi
echo "     • All commands:    push-validator help"
echo

if [[ "$INTERACTIVE" == "yes" ]]; then
  # Interactive mode: show registration action status and pause before dashboard
  echo
  case "$REGISTRATION_STATUS" in
    success)
      ok "Validator registration completed"
      ;;
    failed)
      warn "Validator registration encountered issues; check logs with: push-validator logs"
      ;;
    skipped)
      warn "Validator registration was skipped"
      echo "Run 'push-validator register-validator' when ready"
      ;;
  esac

  if [[ "$REGISTRATION_STATUS" != "already" ]]; then
    echo
  fi

  # Dashboard prompt with clear instructions and options
  echo
  echo "╔═══════════════════════════════════════════════════════════════╗"
  echo "║                   DASHBOARD AVAILABLE                         ║"
  echo "╚═══════════════════════════════════════════════════════════════╝"
  echo
  echo "  The node is running in the background."
  echo "  Press ENTER to open the interactive dashboard (or Ctrl+C to skip)"
  echo "  Note: The node will continue running in the background."
  echo
  echo "─────────────────────────────────────────────────────────────"
  # Read from /dev/tty to work correctly when script is piped (e.g., curl | bash)
  if [[ -e /dev/tty ]]; then
    read -r -p "Press ENTER to continue to the dashboard... " < /dev/tty 2> /dev/tty || {
      echo
      echo "  Dashboard skipped. Node is running in background."
      echo
      exit 0
    }
  else
    # Fallback if /dev/tty is not available (shouldn't happen on most systems)
    read -r -p "Press ENTER to continue to the dashboard... " || {
      echo
      echo "  Dashboard skipped. Node is running in background."
      echo
      exit 0
    }
  fi

  echo
  "$MANAGER_BIN" dashboard < /dev/tty > /dev/tty 2>&1 || true

  # After dashboard exit, show clear status and next steps
  echo
  echo "═══════════════════════════════════════════════════════════════"
  if node_running; then
    ok "Dashboard closed. Node is still running in background."
  else
    warn "Node is not running"
    echo "  Start it with: push-validator start"
  fi
  echo "═══════════════════════════════════════════════════════════════"
else
  # Non-interactive mode: show status
  echo
  if node_running; then
    ok "Node is running in background"
  else
    warn "Node is not running"
    echo "Start it with: push-validator start"
  fi
fi
