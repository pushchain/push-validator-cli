package main

import (
    "context"
    "fmt"
    "sort"
    "strconv"
    "strings"
    "time"

    "github.com/pushchain/push-validator-cli/internal/dashboard"
    ui "github.com/pushchain/push-validator-cli/internal/ui"
    "github.com/pushchain/push-validator-cli/internal/validator"
)

// truncateAddress truncates long addresses while keeping prefix and suffix visible
func truncateAddress(addr string, maxWidth int) string {
    if len(addr) <= maxWidth {
        return addr
    }
    if strings.HasPrefix(addr, "pushvaloper") {
        prefix := addr[:14]
        suffix := addr[len(addr)-8:]
        return prefix + "..." + suffix
    }
    if strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X") {
        prefix := addr[:6]
        suffix := addr[len(addr)-6:]
        return prefix + "..." + suffix
    }
    return addr
}

// handleValidatorsWithFormat prints either a pretty table (default)
// or raw JSON (--output=json at root) of the current validator set.
func handleValidatorsWithFormat(d *Deps, jsonOut bool) error {
    cfg := d.Cfg
    // For JSON output, query raw data directly (matches chain's native format)
    if jsonOut {
        remote := fmt.Sprintf("https://%s", cfg.GenesisDomain)
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        output, err := d.Runner.Run(ctx, findPchaind(), "query", "staking", "validators", "--node", remote, "-o", "json")
        if err != nil {
            if ctx.Err() == context.DeadlineExceeded {
                return fmt.Errorf("validators: timeout connecting to %s", cfg.GenesisDomain)
            }
            return fmt.Errorf("validators: %w", err)
        }
        // passthrough raw JSON
        fmt.Println(string(output))
        return nil
    }

    // For table output, use cached fetcher (same approach as dashboard)
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    valList, err := d.Fetcher.GetAllValidators(ctx, cfg)
    if err != nil {
        return fmt.Errorf("validators: %w", err)
    }

    if valList.Total == 0 {
        fmt.Println("No validators found or node not synced")
        return nil
    }

    // Fetch my validator info to highlight in table
    myValidatorAddr := ""
    myValCtx, myValCancel := context.WithTimeout(context.Background(), 10*time.Second)
    if myVal, err := d.Fetcher.GetMyValidator(myValCtx, cfg); err == nil {
        myValidatorAddr = myVal.Address
    }
    myValCancel()

    type validatorDisplay struct {
        moniker       string
        status        string
        statusOrder   int
        jailed        bool
        tokensPC      float64
        commissionPct float64
        operatorAddr  string
        cosmosAddr    string
        evmAddress    string
        isMyValidator bool
    }
    vals := make([]validatorDisplay, len(valList.Validators))

    for i, v := range valList.Validators {
        vals[i] = validatorDisplay{
            moniker:       v.Moniker,
            operatorAddr:  v.OperatorAddress,
            cosmosAddr:    v.OperatorAddress,
            jailed:        v.Jailed,
            isMyValidator: myValidatorAddr != "" && v.OperatorAddress == myValidatorAddr,
        }
        if vals[i].moniker == "" {
            vals[i].moniker = "unknown"
        }

        // Status is already converted (BONDED, UNBONDING, UNBONDED)
        switch v.Status {
        case "BONDED":
            vals[i].status, vals[i].statusOrder = "BONDED", 1
        case "UNBONDING":
            vals[i].status, vals[i].statusOrder = "UNBONDING", 2
        case "UNBONDED":
            vals[i].status, vals[i].statusOrder = "UNBONDED", 3
        default:
            vals[i].status, vals[i].statusOrder = v.Status, 4
        }

        // Parse tokens to PC
        if v.Tokens != "" && v.Tokens != "0" {
            if t, err := strconv.ParseFloat(v.Tokens, 64); err == nil {
                vals[i].tokensPC = t / 1e18
            }
        }

        // Parse commission percentage (v.Commission is already "XX%" format, extract the number)
        if v.Commission != "" && v.Commission != "0%" {
            commStr := strings.TrimSuffix(v.Commission, "%")
            if c, err := strconv.ParseFloat(commStr, 64); err == nil {
                vals[i].commissionPct = c
            }
        }

        // Convert address to EVM format synchronously (pure Go, no subprocess)
        vals[i].evmAddress = validator.Bech32ToHex(v.OperatorAddress)
    }
    sort.Slice(vals, func(i, j int) bool {
        // My validator always comes first
        if vals[i].isMyValidator != vals[j].isMyValidator {
            return vals[i].isMyValidator
        }
        if vals[i].statusOrder != vals[j].statusOrder { return vals[i].statusOrder < vals[j].statusOrder }
        return vals[i].tokensPC > vals[j].tokensPC
    })
    c := ui.NewColorConfig()
    fmt.Println()
    fmt.Println(c.Header(" 👥 Active Push Chain Validators "))
    headers := []string{"VALIDATOR", "STATUS", "STAKE(PC)", "COMM%", "EVM_ADDR"}
    rows := make([][]string, 0, len(vals))
    for _, v := range vals {
        // Check if this is my validator
        moniker := v.moniker
        if v.isMyValidator {
            moniker = moniker + " [My Validator]"
        }

        // Build status string with optional (JAILED) suffix
        statusStr := v.status
        if v.jailed {
            statusStr = statusStr + " (JAILED)"
        }

        row := []string{
            moniker,
            statusStr,
            dashboard.FormatLargeNumber(int64(v.tokensPC)),
            fmt.Sprintf("%.0f%%", v.commissionPct),
            v.evmAddress,
        }

        // Apply green highlighting to the entire row if it's my validator
        if v.isMyValidator {
            for i := range row {
                row[i] = c.Success(row[i])
            }
        }

        rows = append(rows, row)
    }
    fmt.Print(ui.Table(c, headers, rows, nil))
    fmt.Printf("Total Validators: %d\n", len(vals))
    fmt.Println(c.Info("💡 Tip: Use --output=json for full addresses and raw data"))
    return nil
}
