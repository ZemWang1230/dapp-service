# TimeLocker 前端测试项目

这是一个用于测试 TimeLocker 后端认证 API 的前端项目，支持钱包连接、用户认证、令牌刷新和用户资料获取功能。

## 功能特点

- 🔗 **钱包连接**: 支持 MetaMask 等以太坊钱包连接
- 🔐 **数字签名认证**: 通过数字签名验证钱包所有权
- 🔄 **令牌刷新**: 自动刷新访问令牌
- 👤 **用户资料**: 获取并显示用户资料信息
- 📱 **响应式设计**: 支持移动设备和桌面设备
- 🎨 **现代化UI**: 美观的渐变设计和动画效果

## 技术栈

- **前端框架**: 纯 JavaScript (ES6+)
- **UI框架**: Bootstrap 5
- **Web3库**: Ethers.js v6
- **HTTP客户端**: 原生 Fetch API
- **钱包支持**: MetaMask, WalletConnect 等

## 项目结构

```
front-test/
├── index.html          # 主页面
├── app.js             # 主要逻辑代码
├── style.css          # 自定义样式
├── package.json       # 项目配置
└── README.md          # 项目说明
```

## 快速开始

### 1. 环境准备

确保你的系统已安装：
- Node.js (推荐 v16+)
- 现代浏览器 (Chrome, Firefox, Safari, Edge)
- MetaMask 或其他以太坊钱包扩展

### 2. 安装依赖

```bash
cd front-test
npm install
```

### 3. 启动后端服务

确保 TimeLocker 后端服务正在运行：

```bash
# 在 timelocker-backend 目录下
go run cmd/server/main.go
```

后端服务应该在 `http://localhost:8080` 上运行。

### 4. 启动前端测试项目

```bash
npm start
```

这将启动一个本地开发服务器，通常在 `http://localhost:3000` 上。

### 5. 或者直接打开 HTML 文件

你也可以直接在浏览器中打开 `index.html` 文件：

```bash
open index.html
# 或者双击文件
```

## 使用说明

### 1. 连接钱包

1. 点击 "连接钱包" 按钮
2. 在弹出的钱包界面中确认连接
3. 钱包连接后会自动开始认证流程

### 2. 数字签名认证

1. 钱包连接后，会弹出签名请求
2. 在钱包中确认签名
3. 签名成功后获得访问令牌和刷新令牌

### 3. 刷新令牌

1. 点击 "刷新Token" 按钮
2. 系统会使用刷新令牌获取新的访问令牌

### 4. 获取用户资料

1. 点击 "获取用户资料" 按钮
2. 系统会显示当前用户的基本信息

## API 接口说明

### 1. 钱包连接认证

- **接口**: `POST /api/v1/auth/wallet-connect`
- **功能**: 通过钱包签名进行用户认证
- **请求参数**:
  ```json
  {
    "wallet_address": "0x...",
    "signature": "0x...",
    "message": "TimeLocker Login Nonce: 1234567890",
    "chain_id": 1
  }
  ```

### 2. 刷新访问令牌

- **接口**: `POST /api/v1/auth/refresh`
- **功能**: 使用刷新令牌获取新的访问令牌
- **请求参数**:
  ```json
  {
    "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
  }
  ```

### 3. 获取用户资料

- **接口**: `GET /api/v1/auth/profile`
- **功能**: 获取当前认证用户的资料信息
- **请求头**: `Authorization: Bearer <access_token>`

## 配置说明

### 后端服务地址

在页面的"配置设置"区域，你可以修改后端服务地址：
- 默认: `http://localhost:8080`
- 如果后端运行在其他地址，请相应修改

### 链ID设置

支持不同的以太坊网络：
- `1`: 以太坊主网
- `5`: Goerli 测试网
- `11155111`: Sepolia 测试网
- 其他自定义网络

## 故障排除

### 1. 钱包连接失败

- 确保已安装 MetaMask 或其他支持的钱包
- 检查钱包是否已解锁
- 尝试刷新页面重新连接

### 2. 签名失败

- 确保在钱包中点击了"签名"而不是"取消"
- 检查钱包网络是否正确
- 尝试切换到支持的网络

### 3. API 调用失败

- 确保后端服务正在运行
- 检查后端服务地址配置是否正确
- 查看浏览器控制台的错误信息
- 检查CORS设置（如果跨域访问）

### 4. 常见错误信息

- `INVALID_WALLET_ADDRESS`: 钱包地址格式无效
- `INVALID_SIGNATURE`: 签名验证失败
- `INVALID_REFRESH_TOKEN`: 刷新令牌无效或过期
- `USER_NOT_FOUND`: 用户不存在
- `UNAUTHORIZED`: 访问令牌无效或过期

## 开发说明

### 代码结构

- `TimeLockWalletTest` 类是主要的应用类
- 使用事件驱动的架构
- 支持钱包状态变化监听
- 包含完整的错误处理机制

### 扩展功能

如需添加新功能，可以：

1. 在 `TimeLockWalletTest` 类中添加新方法
2. 在 HTML 中添加对应的UI元素
3. 绑定事件监听器
4. 更新UI状态管理

### 样式定制

- 修改 `style.css` 中的 CSS 变量
- 支持响应式设计
- 使用 Bootstrap 5 组件系统

## 安全注意事项

1. **私钥安全**: 本项目不会接触或存储私钥
2. **令牌存储**: 令牌仅存储在内存中，页面刷新后需重新认证
3. **HTTPS**: 生产环境建议使用 HTTPS
4. **跨域**: 注意 CORS 安全配置

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

如有问题，请联系开发团队。 