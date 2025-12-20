// Package neoaccounts provides transaction signing for the neoaccounts service.
package neoaccounts

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"

	neoaccountssupabase "github.com/R3E-Network/service_layer/infrastructure/accountpool/supabase"
	"github.com/R3E-Network/service_layer/infrastructure/chain"
	intcrypto "github.com/R3E-Network/service_layer/infrastructure/crypto"
)

// SignTransaction signs a transaction hash with an account's private key.
// The account must be locked by the requesting service.
func (s *Service) SignTransaction(ctx context.Context, serviceID, accountID string, txHash []byte) (*SignTransactionResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}

	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	signature, err := signHash(priv, txHash)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	pubBytes := intcrypto.PublicKeyToBytes(&priv.PublicKey)

	return &SignTransactionResponse{
		AccountID: accountID,
		Signature: signature,
		PublicKey: pubBytes,
	}, nil
}

// BatchSign signs multiple transaction hashes.
func (s *Service) BatchSign(ctx context.Context, serviceID string, requests []SignRequest) *BatchSignResponse {
	resp := &BatchSignResponse{
		Signatures: make([]SignTransactionResponse, 0, len(requests)),
		Errors:     make([]string, 0),
	}

	for _, req := range requests {
		sig, err := s.SignTransaction(ctx, serviceID, req.AccountID, req.TxHash)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", req.AccountID, err))
			continue
		}
		resp.Signatures = append(resp.Signatures, *sig)
	}

	return resp
}

// signHash signs a hash using ECDSA.
func signHash(priv *ecdsa.PrivateKey, hash []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash)
	if err != nil {
		return nil, err
	}

	rBytes := r.Bytes()
	sBytes := s.Bytes()

	signature := make([]byte, 64)
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// verifySignature verifies an ECDSA signature.
func verifySignature(pub *ecdsa.PublicKey, hash, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}

	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	return ecdsa.Verify(pub, hash, r, s)
}

// Transfer transfers tokens from a pool account to a target address.
// The account must be locked by the requesting service.
//
// The transfer is executed as an on-chain NEP-17 `transfer(from,to,amount,data)` invocation
// signed by the pool account's derived private key.
func (s *Service) Transfer(ctx context.Context, serviceID, accountID, toAddress string, amount int64, tokenHash string) (string, error) {
	if s.repo == nil {
		return "", fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return "", fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return "", fmt.Errorf("account_id required")
	}
	if toAddress == "" {
		return "", fmt.Errorf("to_address required")
	}
	if amount <= 0 {
		return "", fmt.Errorf("amount must be positive")
	}

	tokenHash = strings.TrimSpace(tokenHash)
	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return "", fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return "", fmt.Errorf("account not locked by service %s", serviceID)
	}

	// Default to GAS if no token hash specified
	if tokenHash == "" {
		tokenHash = neoaccountssupabase.GASScriptHash
	}
	tokenHash = strings.TrimSpace(tokenHash)
	tokenHash = strings.TrimPrefix(strings.TrimPrefix(tokenHash, "0x"), "0X")
	if tokenHash == "" {
		s.mu.RUnlock()
		return "", fmt.Errorf("token_hash required")
	}
	tokenHash = "0x" + tokenHash

	// Copy required account fields while holding the lock; do not hold the lock across RPC calls.
	fromAddress := strings.TrimSpace(acc.Address)
	s.mu.RUnlock()

	// Derive pool account private key and build a neo-go signer account.
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return "", fmt.Errorf("create signer account: %w", err)
	}

	// Convert addresses to script-hash strings for RPC params.
	fromU160, err := address.StringToUint160(fromAddress)
	if err != nil {
		return "", fmt.Errorf("invalid from address %q: %w", fromAddress, err)
	}
	toU160, err := address.StringToUint160(strings.TrimSpace(toAddress))
	if err != nil {
		return "", fmt.Errorf("invalid to address %q: %w", toAddress, err)
	}

	params := []chain.ContractParam{
		chain.NewHash160Param("0x" + fromU160.StringLE()),
		chain.NewHash160Param("0x" + toU160.StringLE()),
		chain.NewIntegerParam(big.NewInt(amount)),
		chain.NewAnyParam(),
	}

	// Build and sign the transaction locally.
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, tokenHash, "transfer", params, signer.ScriptHash())
	if err != nil {
		return "", fmt.Errorf("transfer simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return "", fmt.Errorf("transfer simulation faulted: %s", invokeResult.Exception)
	}

	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, transaction.CalledByEntry)
	if err != nil {
		return "", fmt.Errorf("build transfer transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return "", err
	}

	txHashString := "0x" + txHash.StringLE()

	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		return txHashString, fmt.Errorf("wait for transfer execution: %w", err)
	}
	if appLog != nil && len(appLog.Executions) > 0 && appLog.Executions[0].VMState != "HALT" {
		return txHashString, fmt.Errorf("transfer failed with state: %s", appLog.Executions[0].VMState)
	}

	// Best-effort account metadata update; the chain tx succeeded regardless.
	s.mu.Lock()
	acc.LastUsedAt = time.Now()
	acc.TxCount++
	if updateErr := s.repo.Update(ctx, acc); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).WithFields(map[string]interface{}{
			"account_id": accountID,
			"tx_hash":    txHashString,
		}).Warn("failed to update account metadata after transfer")
	}
	s.mu.Unlock()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account_id": accountID,
		"to_address": toAddress,
		"amount":     amount,
		"token_hash": tokenHash,
		"tx_hash":    txHashString,
	}).Info("transfer completed")

	return txHashString, nil
}

// DeployContract deploys a new smart contract using a pool account.
// All signing happens inside TEE - private keys never leave the enclave.
func (s *Service) DeployContract(ctx context.Context, serviceID, accountID, nefBase64, manifestJSON string, data any) (*DeployContractResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if nefBase64 == "" {
		return nil, fmt.Errorf("nef_base64 required")
	}
	if manifestJSON == "" {
		return nil, fmt.Errorf("manifest_json required")
	}

	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key inside TEE
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("create signer account: %w", err)
	}

	// Decode NEF from base64
	nefBytes, err := base64.StdEncoding.DecodeString(nefBase64)
	if err != nil {
		return nil, fmt.Errorf("decode nef base64: %w", err)
	}

	// Build deployment parameters
	params := []chain.ContractParam{
		chain.NewByteArrayParam(nefBytes),
		chain.NewStringParam(manifestJSON),
	}
	if data != nil {
		params = append(params, chain.NewAnyParam())
	}

	// ContractManagement native contract hash
	contractMgmtHash := "0xfffdc93764dbaddd97c48f252a53ea4643faa3fd"

	// Simulate deployment first
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractMgmtHash, "deploy", params, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("deployment simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("deployment simulation faulted: %s", invokeResult.Exception)
	}

	// Build and sign the transaction inside TEE
	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, transaction.CalledByEntry)
	if err != nil {
		return nil, fmt.Errorf("build deployment transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast deployment: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Wait for confirmation
	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		return nil, fmt.Errorf("wait for deployment execution: %w", err)
	}

	// Extract contract hash from deployment result
	contractHash := ""
	if appLog != nil && len(appLog.Executions) > 0 {
		exec := appLog.Executions[0]
		if exec.VMState != "HALT" {
			return nil, fmt.Errorf("deployment failed with state: %s", exec.VMState)
		}
		// Contract hash is typically in the first notification or stack result
		// The stack item contains the deployed contract state as a struct
		if len(exec.Stack) > 0 {
			// Try to extract hash from the stack item's Value field
			var valueMap map[string]any
			if err := json.Unmarshal(exec.Stack[0].Value, &valueMap); err == nil {
				if h, ok := valueMap["hash"].(string); ok {
					contractHash = h
				}
			}
		}
	}

	// Update account metadata
	s.mu.Lock()
	acc.LastUsedAt = time.Now()
	acc.TxCount++
	if updateErr := s.repo.Update(ctx, acc); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).Warn("failed to update account metadata after deploy")
	}
	s.mu.Unlock()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account_id":    accountID,
		"tx_hash":       txHashString,
		"contract_hash": contractHash,
		"gas_consumed":  invokeResult.GasConsumed,
	}).Info("contract deployed")

	return &DeployContractResponse{
		TxHash:       txHashString,
		ContractHash: contractHash,
		GasConsumed:  invokeResult.GasConsumed,
		AccountID:    accountID,
	}, nil
}

// UpdateContract updates an existing smart contract using a pool account.
// All signing happens inside TEE - private keys never leave the enclave.
func (s *Service) UpdateContract(ctx context.Context, serviceID, accountID, contractHash, nefBase64, manifestJSON string, data any) (*UpdateContractResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if contractHash == "" {
		return nil, fmt.Errorf("contract_hash required")
	}
	if nefBase64 == "" {
		return nil, fmt.Errorf("nef_base64 required")
	}
	if manifestJSON == "" {
		return nil, fmt.Errorf("manifest_json required")
	}

	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key inside TEE
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("create signer account: %w", err)
	}

	// Decode NEF from base64
	nefBytes, err := base64.StdEncoding.DecodeString(nefBase64)
	if err != nil {
		return nil, fmt.Errorf("decode nef base64: %w", err)
	}

	// Build update parameters - call update on the contract itself
	params := []chain.ContractParam{
		chain.NewByteArrayParam(nefBytes),
		chain.NewStringParam(manifestJSON),
	}
	if data != nil {
		params = append(params, chain.NewAnyParam())
	}

	// Simulate update
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractHash, "update", params, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("update simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return nil, fmt.Errorf("update simulation faulted: %s", invokeResult.Exception)
	}

	// Build and sign the transaction inside TEE
	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, transaction.CalledByEntry)
	if err != nil {
		return nil, fmt.Errorf("build update transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast update: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Wait for confirmation
	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		return nil, fmt.Errorf("wait for update execution: %w", err)
	}
	if appLog != nil && len(appLog.Executions) > 0 && appLog.Executions[0].VMState != "HALT" {
		return nil, fmt.Errorf("update failed with state: %s", appLog.Executions[0].VMState)
	}

	// Update account metadata
	s.mu.Lock()
	acc.LastUsedAt = time.Now()
	acc.TxCount++
	if updateErr := s.repo.Update(ctx, acc); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).Warn("failed to update account metadata after update")
	}
	s.mu.Unlock()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account_id":    accountID,
		"tx_hash":       txHashString,
		"contract_hash": contractHash,
		"gas_consumed":  invokeResult.GasConsumed,
	}).Info("contract updated")

	return &UpdateContractResponse{
		TxHash:       txHashString,
		ContractHash: contractHash,
		GasConsumed:  invokeResult.GasConsumed,
		AccountID:    accountID,
	}, nil
}

// InvokeContract invokes a contract method using a pool account.
// All signing happens inside TEE - private keys never leave the enclave.
func (s *Service) InvokeContract(ctx context.Context, serviceID, accountID, contractHash, method string, params []ContractParam) (*InvokeContractResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if contractHash == "" {
		return nil, fmt.Errorf("contract_hash required")
	}
	if method == "" {
		return nil, fmt.Errorf("method required")
	}

	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key inside TEE
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("create signer account: %w", err)
	}

	// Convert params to chain.ContractParam
	chainParams := make([]chain.ContractParam, len(params))
	for i, p := range params {
		chainParams[i] = convertToChainParam(p)
	}

	// Simulate invocation
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractHash, method, chainParams, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("invocation simulation failed: %w", err)
	}
	if invokeResult.State != "HALT" {
		return &InvokeContractResponse{
			State:       invokeResult.State,
			GasConsumed: invokeResult.GasConsumed,
			Exception:   invokeResult.Exception,
			AccountID:   accountID,
		}, fmt.Errorf("invocation simulation faulted: %s", invokeResult.Exception)
	}

	// Build and sign the transaction inside TEE
	txBuilder := chain.NewTxBuilder(s.chainClient, s.chainClient.NetworkID())
	tx, err := txBuilder.BuildAndSignTx(ctx, invokeResult, signer, transaction.CalledByEntry)
	if err != nil {
		return nil, fmt.Errorf("build invocation transaction: %w", err)
	}

	txHash, err := txBuilder.BroadcastTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("broadcast invocation: %w", err)
	}

	txHashString := "0x" + txHash.StringLE()

	// Wait for confirmation
	waitCtx, cancel := context.WithTimeout(ctx, chain.DefaultTxWaitTimeout)
	defer cancel()

	appLog, err := s.chainClient.WaitForApplicationLog(waitCtx, txHashString, chain.DefaultPollInterval)
	if err != nil {
		return nil, fmt.Errorf("wait for invocation execution: %w", err)
	}

	state := "HALT"
	exception := ""
	if appLog != nil && len(appLog.Executions) > 0 {
		state = appLog.Executions[0].VMState
		exception = appLog.Executions[0].Exception
	}

	// Update account metadata
	s.mu.Lock()
	acc.LastUsedAt = time.Now()
	acc.TxCount++
	if updateErr := s.repo.Update(ctx, acc); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).Warn("failed to update account metadata after invoke")
	}
	s.mu.Unlock()

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"account_id":    accountID,
		"tx_hash":       txHashString,
		"contract_hash": contractHash,
		"method":        method,
		"gas_consumed":  invokeResult.GasConsumed,
	}).Info("contract invoked")

	return &InvokeContractResponse{
		TxHash:      txHashString,
		State:       state,
		GasConsumed: invokeResult.GasConsumed,
		Exception:   exception,
		AccountID:   accountID,
	}, nil
}

// SimulateContract simulates a contract invocation without signing or broadcasting.
func (s *Service) SimulateContract(ctx context.Context, serviceID, accountID, contractHash, method string, params []ContractParam) (*SimulateContractResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	if s.chainClient == nil {
		return nil, fmt.Errorf("chain client not configured")
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if contractHash == "" {
		return nil, fmt.Errorf("contract_hash required")
	}
	if method == "" {
		return nil, fmt.Errorf("method required")
	}

	s.mu.RLock()
	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		s.mu.RUnlock()
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}
	s.mu.RUnlock()

	// Derive pool account private key inside TEE (only for getting script hash)
	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	dBytes := priv.D.Bytes()
	keyBytes := make([]byte, 32)
	copy(keyBytes[32-len(dBytes):], dBytes)
	signer, err := chain.AccountFromPrivateKey(hex.EncodeToString(keyBytes))
	if err != nil {
		return nil, fmt.Errorf("create signer account: %w", err)
	}

	// Convert params to chain.ContractParam
	chainParams := make([]chain.ContractParam, len(params))
	for i, p := range params {
		chainParams[i] = convertToChainParam(p)
	}

	// Simulate invocation
	invokeResult, err := s.chainClient.InvokeFunctionWithSigners(ctx, contractHash, method, chainParams, signer.ScriptHash())
	if err != nil {
		return nil, fmt.Errorf("simulation failed: %w", err)
	}

	return &SimulateContractResponse{
		State:       invokeResult.State,
		GasConsumed: invokeResult.GasConsumed,
		Exception:   invokeResult.Exception,
	}, nil
}

// convertToChainParam converts a ContractParam to chain.ContractParam.
func convertToChainParam(p ContractParam) chain.ContractParam {
	switch strings.ToLower(p.Type) {
	case "hash160":
		if s, ok := p.Value.(string); ok {
			return chain.NewHash160Param(s)
		}
	case "integer":
		switch v := p.Value.(type) {
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return chain.NewIntegerParam(big.NewInt(i))
			}
		case float64:
			return chain.NewIntegerParam(big.NewInt(int64(v)))
		case int64:
			return chain.NewIntegerParam(big.NewInt(v))
		case int:
			return chain.NewIntegerParam(big.NewInt(int64(v)))
		}
	case "string":
		if s, ok := p.Value.(string); ok {
			return chain.NewStringParam(s)
		}
	case "bytearray":
		if s, ok := p.Value.(string); ok {
			// Try base64 decode first, fall back to hex
			if bytes, err := base64.StdEncoding.DecodeString(s); err == nil {
				return chain.NewByteArrayParam(bytes)
			}
			// Try hex decode
			if bytes, err := hex.DecodeString(s); err == nil {
				return chain.NewByteArrayParam(bytes)
			}
			// Use as raw bytes
			return chain.NewByteArrayParam([]byte(s))
		}
	case "bool", "boolean":
		switch v := p.Value.(type) {
		case bool:
			return chain.NewBoolParam(v)
		case string:
			return chain.NewBoolParam(v == "true" || v == "1")
		}
	case "any":
		return chain.NewAnyParam()
	}
	// Default to any
	return chain.NewAnyParam()
}
