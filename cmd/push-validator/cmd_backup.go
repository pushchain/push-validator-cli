package main

import (
    "fmt"

    "github.com/pushchain/push-validator-cli/internal/admin"
)

// handleBackup creates a backup archive of the node configuration and
// prints the resulting path, or a JSON object when --output=json.
func handleBackup(d *Deps) error {
    path, err := admin.Backup(admin.BackupOptions{HomeDir: d.Cfg.HomeDir})
    if err != nil {
        if flagOutput == "json" { d.Printer.JSON(map[string]any{"ok": false, "error": err.Error()}) } else { d.Printer.Error(fmt.Sprintf("backup error: %v", err)) }
        return err
    }
    if flagOutput == "json" { d.Printer.JSON(map[string]any{"ok": true, "backup_path": path}) } else { d.Printer.Success(fmt.Sprintf("backup created: %s", path)) }
    return nil
}
