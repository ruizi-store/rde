// User Store - Svelte 5 Runes
// 用户状态管理

export interface User {
  id: string;
  username: string;
  email?: string;
  nickname?: string;
  avatar?: string;
  role: "admin" | "user";
}

class UserStore {
  // 当前用户
  user = $state<User | null>(null);

  // Token
  token = $state<string | null>(null);

  // 是否已登录
  get isLoggedIn(): boolean {
    return this.user !== null && this.token !== null;
  }

  // 是否是管理员
  get isAdmin(): boolean {
    return this.user?.role === "admin";
  }

  // 登录
  login(user: User, token: string): void {
    this.user = user;
    this.token = token;
    localStorage.setItem("auth_token", token);
  }

  // 登出
  logout(): void {
    this.user = null;
    this.token = null;
    localStorage.removeItem("auth_token");
  }

  // 从 localStorage 恢复 Token
  restoreToken(): string | null {
    if (typeof window === "undefined") return null;
    const token = localStorage.getItem("auth_token");
    if (token) {
      this.token = token;
    }
    return token;
  }

  // 更新用户信息
  updateUser(updates: Partial<User>): void {
    if (this.user) {
      this.user = { ...this.user, ...updates };
    }
  }

  // 设置用户（不带 token）
  setUser(userData: User): void {
    this.user = userData;
  }
}

export const userStore = new UserStore();
export const user = userStore; // 别名
