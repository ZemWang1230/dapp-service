package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"timelocker-backend/internal/types"
)

func main() {
	// 模拟前端钱包认证流程
	fmt.Println("=== TimeLocker 钱包认证演示 ===")
	fmt.Println()

	// 1. 生成测试用的私钥和地址
	privateKey, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		panic(fmt.Sprintf("Failed to create private key: %v", err))
	}

	walletAddress := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
	fmt.Printf("钱包地址: %s\n", walletAddress)

	// 2. 生成签名消息
	message := fmt.Sprintf("TimeLocker Login Nonce: %d", time.Now().Unix())
	fmt.Printf("签名消息: %s\n", message)

	// 3. 对消息进行签名
	messageHash := accounts.TextHash([]byte(message))
	signature, err := crypto.Sign(messageHash, privateKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to sign message: %v", err))
	}

	// 调整v值以符合以太坊标准
	if signature[64] < 27 {
		signature[64] += 27
	}

	signatureHex := hexutil.Encode(signature)
	fmt.Printf("签名结果: %s\n\n", signatureHex)

	// 4. 构造认证请求
	authRequest := types.WalletConnectRequest{
		WalletAddress: walletAddress,
		Signature:     signatureHex,
		Message:       message,
		ChainId:       1, // 以太坊主网
	}

	// 5. 发送认证请求（这里只是演示，实际需要启动服务器）
	fmt.Println("=== 认证请求数据 ===")
	requestJSON, _ := json.MarshalIndent(authRequest, "", "  ")
	fmt.Printf("POST /api/v1/auth/wallet-connect\n%s\n\n", string(requestJSON))

	// 6. 模拟成功响应
	fmt.Println("=== 预期响应 ===")
	mockResponse := types.APIResponse{
		Success: true,
		Data: types.WalletConnectResponse{
			AccessToken:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			RefreshToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			User: types.User{
				ID:            1,
				WalletAddress: walletAddress,
				CreatedAt:     time.Now(),
				LastLogin:     &[]time.Time{time.Now()}[0],
				Preferences:   make(map[string]interface{}),
				Status:        1,
			},
		},
	}

	responseJSON, _ := json.MarshalIndent(mockResponse, "", "  ")
	fmt.Printf("%s\n", string(responseJSON))
	fmt.Println()

	// 7. 演示如何在前端使用
	fmt.Println("=== 前端集成示例 ===")
	fmt.Println(`
// JavaScript/TypeScript 示例
async function connectWallet() {
  // 1. 连接钱包
  const accounts = await window.ethereum.request({ 
    method: 'eth_requestAccounts' 
  });
  const walletAddress = accounts[0];
  
  // 2. 生成签名消息
  const message = ` + "`TimeLocker Login Nonce: ${Date.now()}`" + `;
  
  // 3. 请求用户签名
  const signature = await window.ethereum.request({
    method: 'personal_sign',
    params: [message, walletAddress]
  });
  
  // 4. 发送认证请求
  const response = await fetch('/api/v1/auth/wallet-connect', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      wallet_address: walletAddress,
      signature: signature,
      message: message,
      chain_id: 1
    })
  });
  
  const result = await response.json();
  
  if (result.success) {
    // 保存令牌
    localStorage.setItem('access_token', result.data.access_token);
    localStorage.setItem('refresh_token', result.data.refresh_token);
    
    console.log('认证成功:', result.data.user);
  } else {
    console.error('认证失败:', result.error);
  }
}
`)

	fmt.Println("\n=== 演示完成 ===")
	fmt.Println("要测试完整功能，请：")
	fmt.Println("1. 启动PostgreSQL数据库")
	fmt.Println("2. 复制 config.yaml.example 到 config.yaml 并配置数据库连接")
	fmt.Println("3. 运行: go run cmd/server/main.go")
	fmt.Println("4. 使用上述请求数据测试API端点")
}

// 演示如何发送HTTP请求（需要服务器运行）
func sendAuthRequest(request types.WalletConnectRequest) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		return
	}

	resp, err := http.Post("http://localhost:8080/api/v1/auth/wallet-connect", 
		"application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	fmt.Printf("Response: %s\n", string(body))
} 