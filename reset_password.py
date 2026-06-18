#!/usr/bin/env python3
"""
重置管理员密码脚本
使用方法: python reset_password.py [用户名] [新密码]
默认: python reset_password.py admin Admin123!
"""

import sys
import os

# 添加项目路径
sys.path.insert(0, os.path.dirname(__file__))

from app.database import get_db
from app.auth import hash_password


def get_user_sync(username: str) -> dict | None:
    """同步获取用户（仅用于脚本）"""
    with get_db() as conn:
        row = conn.execute("SELECT * FROM users WHERE username = ?", (username,)).fetchone()
        return dict(row) if row else None


def reset_password(username: str = "admin", new_password: str = "Admin123!"):
    """重置指定用户的密码"""
    print(f"Resetting password for user '{username}'...")

    user = get_user_sync(username)
    if not user:
        print(f"[ERROR] User '{username}' not found")
        return False

    new_hash = hash_password(new_password)

    with get_db() as conn:
        conn.execute(
            "UPDATE users SET password_hash = ?, password_version = COALESCE(password_version, 1) + 1 WHERE username = ?",
            (new_hash, username),
        )
        conn.commit()

    user = get_user_sync(username)
    if user:
        print(f"[OK] Password reset successfully!")
        print(f"  Username: {username}")
        print(f"  New password: {new_password}")
        print(f"  Password version: {user.get('password_version', 1)}")
        return True
    else:
        print("[ERROR] Password reset failed")
        return False


def reset_first_login(username: str = "admin"):
    """重置首次登录状态"""
    print(f"Resetting first login status for user '{username}'...")

    with get_db() as conn:
        conn.execute(
            "UPDATE users SET first_login = 0 WHERE username = ?",
            (username,),
        )
        conn.commit()

    print(f"[OK] First login status reset to 0")


def list_users():
    """列出所有用户"""
    print("Existing users:")
    with get_db() as conn:
        rows = conn.execute("SELECT id, username, first_login, password_version FROM users").fetchall()
        for row in rows:
            print(f"  ID: {row['id']}, Username: {row['username']}, First login: {row['first_login']}, Password version: {row['password_version']}")


if __name__ == "__main__":
    if len(sys.argv) == 1:
        reset_password()
        reset_first_login()
    elif len(sys.argv) == 2:
        if sys.argv[1] == "--list":
            list_users()
        else:
            print("Usage: python reset_password.py [username] [new_password]")
            print("       python reset_password.py --list")
    elif len(sys.argv) == 3:
        username = sys.argv[1]
        new_password = sys.argv[2]
        reset_password(username, new_password)
    else:
        print("Usage: python reset_password.py [username] [new_password]")
        print("       python reset_password.py --list")
