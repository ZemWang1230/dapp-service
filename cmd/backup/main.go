package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"timelocker-backend/internal/config"
	"timelocker-backend/pkg/database"
	"timelocker-backend/pkg/database/migrations"
	"timelocker-backend/pkg/logger"

	"gorm.io/gorm"
)

func main() {
	// 解析命令行参数
	var (
		action     = flag.String("action", "", "Action Type: backup, restore, validate, info, reset")
		backupPath = flag.String("file", "", "Backup File Path")
		clearData  = flag.Bool("clear", false, "Clear Existing Data When Restore")
		conflict   = flag.String("conflict", "skip", "Conflict Resolution Strategy: skip, replace, error")
		help       = flag.Bool("help", false, "Show Help")
	)
	flag.Parse()

	if *help || *action == "" {
		showHelp()
		return
	}

	// 初始化日志
	logger.Init(logger.DefaultConfig())

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config", err)
		os.Exit(1)
	}

	// 连接数据库
	db, err := database.NewPostgresConnection(&cfg.Database)
	if err != nil {
		logger.Error("Failed to connect to database", err)
		os.Exit(1)
	}

	// Create backup manager
	backupManager := database.NewBackupManager(db)

	switch *action {
	case "backup":
		handleBackup(backupManager, *backupPath)
	case "restore":
		handleRestore(backupManager, *backupPath, *clearData, *conflict)
	case "validate":
		handleValidate(backupManager, *backupPath)
	case "info":
		handleInfo(backupManager, *backupPath)
	case "reset":
		handleReset(db)
	default:
		fmt.Printf("Error: Unsupported action '%s'\n", *action)
		showHelp()
		os.Exit(1)
	}
}

func handleBackup(bm *database.BackupManager, backupPath string) {
	if backupPath == "" {
		// 生成默认备份文件名
		backupPath = fmt.Sprintf("./backups/timelocker_backup_%s.json",
			time.Now().Format("20060102_150405"))
	}

	fmt.Printf("Creating backup to: %s\n", backupPath)

	if err := bm.CreateBackup(backupPath); err != nil {
		logger.Error("Backup failed", err)
		fmt.Printf("Backup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Backup created successfully: %s\n", backupPath)
}

func handleRestore(bm *database.BackupManager, backupPath string, clearData bool, conflictStr string) {
	if backupPath == "" {
		fmt.Println("Error: Backup file path is required")
		os.Exit(1)
	}

	// 解析冲突策略
	var conflictAction database.ConflictAction
	switch conflictStr {
	case "skip":
		conflictAction = database.ConflictSkip
	case "replace":
		conflictAction = database.ConflictReplace
	case "error":
		conflictAction = database.ConflictError
	default:
		fmt.Printf("Error: Unsupported conflict strategy '%s'\n", conflictStr)
		os.Exit(1)
	}

	options := database.RestoreOptions{
		ClearExisting: clearData,
		OnConflict:    conflictAction,
	}

	fmt.Printf("Restoring data from backup: %s\n", backupPath)
	if clearData {
		fmt.Println("Warning: Existing user data will be cleared")
	}
	fmt.Printf("Conflict strategy: %s\n", conflictStr)

	// 询问用户确认
	fmt.Print("Continue? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Operation cancelled")
		return
	}

	if err := bm.RestoreBackup(backupPath, options); err != nil {
		logger.Error("Restore failed", err)
		fmt.Printf("Restore failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Data restored successfully")
}

func handleValidate(bm *database.BackupManager, backupPath string) {
	if backupPath == "" {
		fmt.Println("Error: Backup file path is required")
		os.Exit(1)
	}

	fmt.Printf("Validating backup file: %s\n", backupPath)

	if err := bm.ValidateBackup(backupPath); err != nil {
		fmt.Printf("Backup file validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Backup file validated successfully")
}

func handleInfo(bm *database.BackupManager, backupPath string) {
	if backupPath == "" {
		fmt.Println("Error: Backup file path is required")
		os.Exit(1)
	}

	fmt.Printf("Reading backup file info: %s\n", backupPath)

	info, err := bm.GetBackupInfo(backupPath)
	if err != nil {
		fmt.Printf("Failed to read backup file info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nBackup file info:\n")
	fmt.Printf("Version: %s\n", info.Version)
	fmt.Printf("Created at: %s\n", info.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("User count: %d\n", len(info.Users))
	fmt.Printf("User assets count: %d\n", len(info.UserAssets))
	fmt.Printf("User ABI count: %d\n", len(info.ABIs))
	fmt.Printf("Compound Timelock count: %d\n", len(info.CompoundTimelocks))
	fmt.Printf("OpenZeppelin Timelock count: %d\n", len(info.OpenzeppelinTimelocks))
	fmt.Printf("Transaction count: %d\n", len(info.Transactions))
	fmt.Printf("Email notification count: %d\n", len(info.EmailNotifications))
}

func handleReset(db *gorm.DB) {
	fmt.Println("Warning: This operation will delete all database tables and data!")
	fmt.Print("Continue? Please enter 'RESET' to confirm: ")

	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "RESET" {
		fmt.Println("Operation cancelled")
		return
	}

	if err := migrations.ResetDangerous(db); err != nil {
		logger.Error("Reset failed", err)
		fmt.Printf("Reset failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Database reset successfully")
}

func showHelp() {
	fmt.Printf(`TimeLocker Database Backup and Restore Tool

Usage:
  %s -action=<action> [options]

Actions:
  backup    Create data backup
  restore   Restore from backup
  validate  Validate backup file
  info      Display backup file info
  reset     Reset database (dangerous operation)

Options:
  -file=<path>        Backup file path
  -clear             Clear existing data when restore (only for restore)
  -conflict=<strategy>    Conflict resolution strategy: skip|replace|error (only for restore)
  -help              Display this help message

Examples:
  # Create backup
  %s -action=backup
  %s -action=backup -file=./my_backup.json

  # Restore from backup (skip conflicts)
  %s -action=restore -file=./my_backup.json -conflict=skip

  # Restore from backup (clear existing data)
  %s -action=restore -file=./my_backup.json -clear -conflict=replace

  # Validate backup file
  %s -action=validate -file=./my_backup.json

  # Display backup file info
  %s -action=info -file=./my_backup.json

  # Reset database (dangerous)
  %s -action=reset

`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}
