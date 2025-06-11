class TimeLockWalletTest {
    constructor() {
        this.provider = null;
        this.signer = null;
        this.walletAddress = null;
        this.accessToken = null;
        this.refreshToken = null;
        this.apiBaseUrl = 'http://localhost:8080';
        
        this.init();
    }

    init() {
        // 绑定事件监听器
        document.getElementById('connectWalletBtn').addEventListener('click', () => this.connectWallet());
        document.getElementById('refreshTokenBtn').addEventListener('click', () => this.refreshTokenAction());
        document.getElementById('getProfileBtn').addEventListener('click', () => this.getProfile());
        
        // 监听配置变化
        document.getElementById('apiBaseUrl').addEventListener('change', (e) => {
            this.apiBaseUrl = e.target.value;
        });
        
        this.updateUI();
        this.checkWalletConnection();
    }

    async checkWalletConnection() {
        if (typeof window.ethereum !== 'undefined') {
            try {
                const accounts = await window.ethereum.request({ method: 'eth_accounts' });
                if (accounts.length > 0) {
                    this.updateConnectionStatus('检测到已连接的钱包', 'warning');
                }
            } catch (error) {
                console.log('检查钱包连接失败:', error);
            }
        } else {
            this.updateConnectionStatus('未检测到MetaMask或其他钱包', 'danger');
        }
    }

    async connectWallet() {
        try {
            if (typeof window.ethereum === 'undefined') {
                alert('请安装MetaMask或其他以太坊钱包!');
                return;
            }

            // 显示连接模态框
            const walletModal = new bootstrap.Modal(document.getElementById('walletModal'));
            walletModal.show();

            // 连接钱包
            this.provider = new ethers.BrowserProvider(window.ethereum);
            await this.provider.send("eth_requestAccounts", []);
            this.signer = await this.provider.getSigner();
            this.walletAddress = await this.signer.getAddress();

            // 更新UI
            document.getElementById('walletAddress').textContent = this.walletAddress;
            document.getElementById('walletInfo').style.display = 'block';
            
            walletModal.hide();

            this.updateConnectionStatus(`钱包已连接: ${this.walletAddress}`, 'success');

            // 开始认证流程
            await this.authenticateWallet();

        } catch (error) {
            console.error('连接钱包失败:', error);
            this.updateConnectionStatus(`连接失败: ${error.message}`, 'danger');
            
            // 隐藏模态框
            const walletModal = bootstrap.Modal.getInstance(document.getElementById('walletModal'));
            if (walletModal) {
                walletModal.hide();
            }
        }
    }

    async authenticateWallet() {
        try {
            // 显示签名模态框
            const signModal = new bootstrap.Modal(document.getElementById('signModal'));
            signModal.show();

            // 生成签名消息
            const timestamp = Date.now();
            const message = `TimeLocker Login Nonce: ${timestamp}`;
            document.getElementById('signMessage').textContent = message;

            // 请求用户签名
            const signature = await this.signer.signMessage(message);
            
            // 获取链ID
            const network = await this.provider.getNetwork();
            const chainId = Number(network.chainId);

            // 构造认证请求
            const authRequest = {
                wallet_address: this.walletAddress,
                signature: signature,
                message: message,
                chain_id: chainId
            };

            // 发送认证请求
            const response = await this.callAPI('/api/v1/auth/wallet-connect', 'POST', authRequest);
            
            signModal.hide();

            if (response.success) {
                // 保存令牌
                this.accessToken = response.data.access_token;
                this.refreshToken = response.data.refresh_token;
                
                // 更新UI
                this.updateTokenDisplay(response.data);
                this.updateConnectionStatus('认证成功!', 'success');
                this.updateUI();
                
                // 显示认证响应
                document.getElementById('authResponse').textContent = JSON.stringify(response, null, 2);
                
            } else {
                throw new Error(response.error?.message || '认证失败');
            }

        } catch (error) {
            console.error('认证失败:', error);
            this.updateConnectionStatus(`认证失败: ${error.message}`, 'danger');
            
            // 隐藏签名模态框
            const signModal = bootstrap.Modal.getInstance(document.getElementById('signModal'));
            if (signModal) {
                signModal.hide();
            }
            
            // 显示错误响应
            document.getElementById('authResponse').textContent = JSON.stringify({
                success: false,
                error: error.message
            }, null, 2);
        }
    }

    async refreshTokenAction() {
        try {
            if (!this.refreshToken) {
                throw new Error('没有可用的刷新令牌');
            }

            const refreshRequest = {
                refresh_token: this.refreshToken
            };

            const response = await this.callAPI('/api/v1/auth/refresh', 'POST', refreshRequest);

            if (response.success) {
                // 更新令牌
                this.accessToken = response.data.access_token;
                this.refreshToken = response.data.refresh_token;
                
                // 更新UI
                this.updateTokenDisplay(response.data);
                this.updateConnectionStatus('令牌刷新成功!', 'success');
                
                // 显示刷新响应
                document.getElementById('refreshResponse').textContent = JSON.stringify(response, null, 2);
                
            } else {
                throw new Error(response.error?.message || '刷新令牌失败');
            }

        } catch (error) {
            console.error('刷新令牌失败:', error);
            this.updateConnectionStatus(`刷新失败: ${error.message}`, 'danger');
            
            // 显示错误响应
            document.getElementById('refreshResponse').textContent = JSON.stringify({
                success: false,
                error: error.message
            }, null, 2);
        }
    }

    async getProfile() {
        try {
            if (!this.accessToken) {
                throw new Error('没有可用的访问令牌');
            }

            const response = await this.callAPI('/api/v1/auth/profile', 'GET', null, {
                'Authorization': `Bearer ${this.accessToken}`
            });

            if (response.success) {
                this.updateConnectionStatus('获取用户资料成功!', 'success');
                
                // 显示用户资料响应
                document.getElementById('profileResponse').textContent = JSON.stringify(response, null, 2);
                
            } else {
                throw new Error(response.error?.message || '获取用户资料失败');
            }

        } catch (error) {
            console.error('获取用户资料失败:', error);
            this.updateConnectionStatus(`获取资料失败: ${error.message}`, 'danger');
            
            // 显示错误响应
            document.getElementById('profileResponse').textContent = JSON.stringify({
                success: false,
                error: error.message
            }, null, 2);
        }
    }

    async callAPI(endpoint, method = 'GET', data = null, headers = {}) {
        try {
            const url = `${this.apiBaseUrl}${endpoint}`;
            const config = {
                method: method,
                headers: {
                    'Content-Type': 'application/json',
                    ...headers
                }
            };

            if (data) {
                config.body = JSON.stringify(data);
            }

            const response = await fetch(url, config);
            const result = await response.json();
            
            return result;
        } catch (error) {
            console.error('API调用失败:', error);
            throw error;
        }
    }

    updateTokenDisplay(authData) {
        document.getElementById('accessToken').value = authData.access_token || '';
        document.getElementById('refreshToken').value = authData.refresh_token || '';
        document.getElementById('expiresAt').value = authData.expires_at ? 
            new Date(authData.expires_at).toLocaleString() : '';
        document.getElementById('userId').value = authData.user?.id || '';
    }

    updateConnectionStatus(message, type) {
        const statusElement = document.getElementById('connectionStatus');
        statusElement.className = `alert alert-${type}`;
        statusElement.innerHTML = `<i class="fas fa-info-circle me-2"></i>${message}`;
    }

    updateUI() {
        const isConnected = this.walletAddress && this.accessToken;
        
        // 更新按钮状态
        document.getElementById('connectWalletBtn').textContent = 
            this.walletAddress ? '重新连接钱包' : '连接钱包';
        document.getElementById('refreshTokenBtn').disabled = !this.refreshToken;
        document.getElementById('getProfileBtn').disabled = !this.accessToken;
    }

    // 工具方法：格式化钱包地址
    formatAddress(address) {
        if (!address) return '';
        return `${address.slice(0, 6)}...${address.slice(-4)}`;
    }

    // 工具方法：复制到剪贴板
    async copyToClipboard(text) {
        try {
            await navigator.clipboard.writeText(text);
            alert('已复制到剪贴板!');
        } catch (error) {
            console.error('复制失败:', error);
            // 降级方案
            const textarea = document.createElement('textarea');
            textarea.value = text;
            document.body.appendChild(textarea);
            textarea.select();
            document.execCommand('copy');
            document.body.removeChild(textarea);
            alert('已复制到剪贴板!');
        }
    }
}

// 初始化应用
document.addEventListener('DOMContentLoaded', () => {
    window.walletTest = new TimeLockWalletTest();
});

// 添加全局错误处理
window.addEventListener('error', (event) => {
    console.error('全局错误:', event.error);
});

// 添加未处理Promise拒绝的处理
window.addEventListener('unhandledrejection', (event) => {
    console.error('未处理的Promise拒绝:', event.reason);
});

// 监听钱包账户变化
if (typeof window.ethereum !== 'undefined') {
    window.ethereum.on('accountsChanged', (accounts) => {
        console.log('钱包账户变化:', accounts);
        if (accounts.length === 0) {
            // 用户断开了钱包连接
            if (window.walletTest) {
                window.walletTest.walletAddress = null;
                window.walletTest.accessToken = null;
                window.walletTest.refreshToken = null;
                window.walletTest.updateConnectionStatus('钱包已断开连接', 'warning');
                window.walletTest.updateUI();
            }
        } else if (window.walletTest && accounts[0] !== window.walletTest.walletAddress) {
            // 用户切换了账户
            window.location.reload(); // 简单粗暴的方法：重新加载页面
        }
    });

    window.ethereum.on('chainChanged', (chainId) => {
        console.log('链ID变化:', chainId);
        // 链变化时重新加载页面
        window.location.reload();
    });
} 