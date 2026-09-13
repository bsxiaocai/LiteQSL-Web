package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/bsxiaocai/LiteQSL-Web/internal/auth"
	"github.com/bsxiaocai/LiteQSL-Web/internal/config"
	"github.com/bsxiaocai/LiteQSL-Web/internal/database"
)

// runResetPassword 实现 `liteqsl reset-password` 子命令，替代 v1.x reset_password.py。
//
// 用法：
//
//	liteqsl reset-password                       # 将 admin 重置为 Admin123! 并重置首次登录状态
//	liteqsl reset-password --list                # 列出所有用户
//	liteqsl reset-password <用户名> <新密码>      # 重置指定用户密码
//	liteqsl reset-password -config <路径> ...    # 指定配置文件
func runResetPassword(args []string) int {
	fs := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法: liteqsl reset-password [-config 路径] [--list] [用户名] [新密码]")
		fs.PrintDefaults()
	}
	configPath := fs.String("config", "config.yaml", "配置文件路径")
	list := fs.Bool("list", false, "列出所有用户")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		return 1
	}

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败 (%s): %v\n", cfg.DBPath, err)
		return 1
	}
	defer db.Close()

	if err := database.Init(db); err != nil {
		fmt.Fprintf(os.Stderr, "初始化数据库失败: %v\n", err)
		return 1
	}

	if *list {
		return listUsers(db)
	}

	rest := fs.Args()
	username, password := "admin", "Admin123!"
	defaultInvocation := len(rest) == 0
	if len(rest) >= 1 {
		username = rest[0]
	}
	if len(rest) >= 2 {
		password = rest[1]
	}

	user, err := database.GetUser(db, username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询用户失败: %v\n", err)
		return 1
	}
	if user == nil {
		fmt.Fprintf(os.Stderr, "[ERROR] 用户不存在: %s\n", username)
		return 1
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成密码哈希失败: %v\n", err)
		return 1
	}
	if _, err := database.UpdatePassword(db, username, hash); err != nil {
		fmt.Fprintf(os.Stderr, "更新密码失败: %v\n", err)
		return 1
	}
	fmt.Printf("[OK] 密码重置成功\n  用户名: %s\n  新密码: %s\n", username, password)

	// 与 v1.x一致：不带参数调用时同时重置首次登录状态。
	if defaultInvocation {
		if _, err := db.Exec("UPDATE users SET first_login = 0 WHERE username = ?", username); err != nil {
			fmt.Fprintf(os.Stderr, "重置首次登录状态失败: %v\n", err)
			return 1
		}
		fmt.Println("[OK] 首次登录状态已重置为 0")
	}

	// 提示旧会话失效
	if after, err := database.GetUser(db, username); err == nil && after != nil {
		fmt.Printf("  密码版本: %d（旧会话已失效，请重新登录）\n", after.PasswordVersion)
	}
	return 0
}

// listUsers 打印所有用户。
func listUsers(db *sql.DB) int {
	rows, err := db.Query("SELECT id, username, COALESCE(first_login,0), COALESCE(password_version,1) FROM users ORDER BY id")
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询用户失败: %v\n", err)
		return 1
	}
	defer rows.Close()

	fmt.Println("现有用户:")
	count := 0
	for rows.Next() {
		var id int64
		var username string
		var firstLogin, passwordVersion int
		if err := rows.Scan(&id, &username, &firstLogin, &passwordVersion); err != nil {
			fmt.Fprintf(os.Stderr, "读取用户失败: %v\n", err)
			return 1
		}
		fmt.Printf("  ID: %d, 用户名: %s, 首次登录: %d, 密码版本: %d\n", id, username, firstLogin, passwordVersion)
		count++
	}
	if count == 0 {
		fmt.Println("  （无用户）")
	}
	return 0
}
