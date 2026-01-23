package main

import (
    "context"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/pushchain/push-validator-cli/internal/dashboard"
)

// handleBalance prints an account balance. It resolves the address from
// either a positional argument or KEY_NAME when --address/arg is omitted.
// When --output=json is set, it emits a structured object.
func handleBalance(d *Deps, args []string) error {
    var addr string
    if len(args) > 0 { addr = args[0] }
    if addr == "" {
        key := os.Getenv("KEY_NAME")
        if key == "" {
            if flagOutput == "json" { d.Printer.JSON(map[string]any{"ok": false, "error": "address not provided; set KEY_NAME or pass --address"}) } else { fmt.Println("usage: push-validator balance <address> (or set KEY_NAME)") }
            return fmt.Errorf("address not provided")
        }
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        out, err := d.Runner.Run(ctx, findPchaind(), "keys", "show", key, "-a", "--keyring-backend", d.Cfg.KeyringBackend, "--home", d.Cfg.HomeDir)
        cancel()
        if err != nil {
            if flagOutput == "json" { d.Printer.JSON(map[string]any{"ok": false, "error": err.Error()}) } else { fmt.Printf("resolve address error: %v\n", err) }
            return fmt.Errorf("resolve address: %w", err)
        }
        addr = strings.TrimSpace(string(out))
    }

    // Convert hex address (0x...) to bech32 if needed
    if strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X") {
        convCtx, convCancel := context.WithTimeout(context.Background(), 10*time.Second)
        bech32Addr, convErr := hexToBech32Address(convCtx, addr, d.Runner)
        convCancel()
        if convErr != nil {
            if flagOutput == "json" { d.Printer.JSON(map[string]any{"ok": false, "error": convErr.Error(), "address": addr}) } else { d.Printer.Error(fmt.Sprintf("address conversion error: %v", convErr)) }
            return convErr
        }
        addr = bech32Addr
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    bal, err := d.Validator.Balance(ctx, addr)
    if err != nil {
        if flagOutput == "json" { d.Printer.JSON(map[string]any{"ok": false, "error": err.Error(), "address": addr}) } else { d.Printer.Error(fmt.Sprintf("balance error: %v", err)) }
        return err
    }
    if flagOutput == "json" { d.Printer.JSON(map[string]any{"ok": true, "address": addr, "balance": bal, "denom": d.Cfg.Denom}) } else { d.Printer.Info(fmt.Sprintf("%s %s", dashboard.FormatSmartNumber(bal), d.Cfg.Denom)) }
    return nil
}
