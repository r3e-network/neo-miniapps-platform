// Package contract provides test utilities for Neo Express contract deployment and testing.
package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	neoxpPath      = "neoxp"
	defaultTimeout = 30 * time.Second
)

type NeoExpress struct {
	mu      sync.Mutex
	t       *testing.T
	running bool
	cmd     *exec.Cmd
	rpcURL  string
}

type DeployedContract struct {
	Name    string
	Hash    string
	Address string
}

func NewNeoExpress(t *testing.T) *NeoExpress {
	t.Helper()
	return &NeoExpress{
		t:      t,
		rpcURL: "http://127.0.0.1:50012",
	}
}

func (n *NeoExpress) neoxpCmd() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dotnet", "tools", neoxpPath)
}

func (n *NeoExpress) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.running {
		return nil
	}

	n.cmd = exec.CommandContext(ctx, n.neoxpCmd(), "run", "-s", "0")
	n.cmd.Stdout = os.Stdout
	n.cmd.Stderr = os.Stderr

	if err := n.cmd.Start(); err != nil {
		return fmt.Errorf("start neo-express: %w", err)
	}

	n.running = true
	time.Sleep(2 * time.Second)

	return nil
}

func (n *NeoExpress) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running || n.cmd == nil || n.cmd.Process == nil {
		return nil
	}

	if err := n.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill neo-express: %w", err)
	}

	n.running = false
	return nil
}

func (n *NeoExpress) RPCURL() string {
	return n.rpcURL
}

func (n *NeoExpress) CreateWallet(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, n.neoxpCmd(), "wallet", "create", name).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "already exists") {
			return n.GetWalletAddress(name)
		}
		return "", fmt.Errorf("create wallet: %s: %w", string(out), err)
	}

	return n.GetWalletAddress(name)
}

func (n *NeoExpress) GetWalletAddress(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, n.neoxpCmd(), "wallet", "list").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("list wallets: %s: %w", string(out), err)
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, name) {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "N") && len(part) == 34 {
					return part, nil
				}
			}
		}
	}

	return "", fmt.Errorf("wallet %s not found", name)
}

func (n *NeoExpress) TransferGAS(from, to string, amount float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	amountStr := fmt.Sprintf("%.8f", amount)
	out, err := exec.CommandContext(ctx, n.neoxpCmd(), "transfer", amountStr, "GAS", from, to).CombinedOutput()
	if err != nil {
		return fmt.Errorf("transfer GAS: %s: %w", string(out), err)
	}

	return nil
}

func (n *NeoExpress) Deploy(nefPath, manifestPath, account string) (*DeployedContract, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, n.neoxpCmd(), "contract", "deploy", nefPath, account).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("deploy contract: %s: %w", string(out), err)
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Contract") && strings.Contains(line, "deployed") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "0x") && len(part) == 42 {
					return &DeployedContract{
						Hash: part,
					}, nil
				}
			}
		}
	}

	return &DeployedContract{}, nil
}

func (n *NeoExpress) Invoke(contract, method string, args ...interface{}) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmdArgs := []string{"contract", "invoke", contract, method}
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			cmdArgs = append(cmdArgs, v)
		default:
			jsonArg, _ := json.Marshal(v)
			cmdArgs = append(cmdArgs, string(jsonArg))
		}
	}

	out, err := exec.CommandContext(ctx, n.neoxpCmd(), cmdArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("invoke contract: %s: %w", string(out), err)
	}

	return string(out), nil
}

func (n *NeoExpress) RunCheckpoint(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, n.neoxpCmd(), "checkpoint", "create", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create checkpoint: %s: %w", string(out), err)
	}

	return nil
}

func (n *NeoExpress) RestoreCheckpoint(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, n.neoxpCmd(), "checkpoint", "restore", name, "-f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("restore checkpoint: %s: %w", string(out), err)
	}

	return nil
}

func (n *NeoExpress) Reset() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, n.neoxpCmd(), "reset", "-f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("reset: %s: %w", string(out), err)
	}

	return nil
}

func (n *NeoExpress) FastForward(blocks int) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, n.neoxpCmd(), "fastfwd", fmt.Sprintf("%d", blocks)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("fastfwd: %s: %w", string(out), err)
	}

	return nil
}

func FindContractArtifacts(contractName string) (nefPath, manifestPath string, err error) {
	basePath := filepath.Join("..", "..", "contracts", strings.ToLower(contractName))

	nefPath = filepath.Join(basePath, contractName+".nef")
	manifestPath = filepath.Join(basePath, contractName+".manifest.json")

	if _, err := os.Stat(nefPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("NEF file not found: %s", nefPath)
	}
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("manifest not found: %s", manifestPath)
	}

	return nefPath, manifestPath, nil
}

func SkipIfNoNeoExpress(t *testing.T) {
	t.Helper()
	home, _ := os.UserHomeDir()
	neoxp := filepath.Join(home, ".dotnet", "tools", neoxpPath)
	if _, err := os.Stat(neoxp); os.IsNotExist(err) {
		t.Skip("neo-express not installed, skipping contract tests")
	}
}

func SkipIfNoCompiledContracts(t *testing.T) {
	t.Helper()
	contractDir := filepath.Join("..", "..", "contracts", "gateway")
	nefPath := filepath.Join(contractDir, "ServiceLayerGateway.nef")
	if _, err := os.Stat(nefPath); os.IsNotExist(err) {
		t.Skip("contracts not compiled, run 'make build-contracts' first")
	}
}
