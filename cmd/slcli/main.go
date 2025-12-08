// Package main provides the Service Layer CLI for balance and payment management.
//
// Usage:
//
//	slcli balance <user_id>                          - Check user balance
//	slcli deposit <user_id> <tx_hash> <amount>       - Credit deposit (admin only)
//	slcli pay-contract <user_id> <contract> <amount> - Pay for a contract
//	slcli pay-user <user_id> <recipient> <amount>    - Pay for another user
//	slcli transactions <user_id> [limit]             - View transaction history
//	slcli fees                                       - List service fees
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/gasbank"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx := context.Background()

	// Initialize database client
	dbClient, err := database.NewClient(database.Config{
		URL:        os.Getenv("SUPABASE_URL"),
		ServiceKey: os.Getenv("SUPABASE_SERVICE_KEY"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	repo := database.NewRepository(dbClient)
	manager := gasbank.NewManager(repo)

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "balance":
		cmdBalance(ctx, manager, args)
	case "deposit":
		cmdDeposit(ctx, manager, args)
	case "pay-contract":
		cmdPayContract(ctx, manager, args)
	case "pay-user":
		cmdPayUser(ctx, manager, args)
	case "transactions", "txs":
		cmdTransactions(ctx, manager, args)
	case "fees":
		cmdFees()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Service Layer CLI - Balance and Payment Management

Usage:
  slcli <command> [arguments]

Commands:
  balance <user_id>                            Check user balance
  deposit <user_id> <tx_hash> <amount>         Credit deposit (admin only)
  pay-contract <user_id> <contract> <amount>   Pay for a contract's service fees
  pay-user <user_id> <recipient> <amount>      Pay for another user's service fees
  transactions <user_id> [limit]               View transaction history
  fees                                         List service fees

Environment Variables:
  SUPABASE_URL         Supabase project URL
  SUPABASE_SERVICE_KEY Supabase service key

Examples:
  slcli balance user-123
  slcli deposit user-123 0xabc123... 1000000
  slcli pay-contract user-123 NXa1... 500000
  slcli pay-user user-123 user-456 100000
  slcli transactions user-123 20
  slcli fees`)
}

func cmdBalance(ctx context.Context, m *gasbank.Manager, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: slcli balance <user_id>")
		os.Exit(1)
	}

	userID := args[0]
	balance, reserved, available, err := m.GetBalance(ctx, userID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Account: %s\n", userID)
	fmt.Printf("Balance:   %d (%.8f GAS)\n", balance, float64(balance)/1e8)
	fmt.Printf("Reserved:  %d (%.8f GAS)\n", reserved, float64(reserved)/1e8)
	fmt.Printf("Available: %d (%.8f GAS)\n", available, float64(available)/1e8)
}

func cmdDeposit(ctx context.Context, m *gasbank.Manager, args []string) {
	fs := flag.NewFlagSet("deposit", flag.ExitOnError)
	note := fs.String("note", "", "Optional note for the deposit")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: slcli deposit <user_id> <tx_hash> <amount> [-note <note>]")
		os.Exit(1)
	}

	userID := remaining[0]
	txHash := remaining[1]
	amount, err := strconv.ParseInt(remaining[2], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid amount: %v\n", err)
		os.Exit(1)
	}

	ref := txHash
	if *note != "" {
		ref = fmt.Sprintf("%s:%s", txHash, *note)
	}

	if err := m.Deposit(ctx, userID, amount, ref); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deposited %d (%.8f GAS) to %s\n", amount, float64(amount)/1e8, userID)
	fmt.Printf("Reference: %s\n", ref)
}

func cmdPayContract(ctx context.Context, m *gasbank.Manager, args []string) {
	fs := flag.NewFlagSet("pay-contract", flag.ExitOnError)
	note := fs.String("note", "", "Optional note for the payment")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: slcli pay-contract <user_id> <contract_address> <amount> [-note <note>]")
		os.Exit(1)
	}

	userID := remaining[0]
	contractAddr := remaining[1]
	amount, err := strconv.ParseInt(remaining[2], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid amount: %v\n", err)
		os.Exit(1)
	}

	noteStr := *note
	if noteStr == "" {
		noteStr = time.Now().Format(time.RFC3339)
	}

	if err := m.PayForContract(ctx, userID, contractAddr, amount, noteStr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Paid %d (%.8f GAS) for contract %s\n", amount, float64(amount)/1e8, contractAddr)
	fmt.Printf("Sponsor: %s\n", userID)
}

func cmdPayUser(ctx context.Context, m *gasbank.Manager, args []string) {
	fs := flag.NewFlagSet("pay-user", flag.ExitOnError)
	note := fs.String("note", "", "Optional note for the payment")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: slcli pay-user <user_id> <recipient_user_id> <amount> [-note <note>]")
		os.Exit(1)
	}

	userID := remaining[0]
	recipientID := remaining[1]
	amount, err := strconv.ParseInt(remaining[2], 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid amount: %v\n", err)
		os.Exit(1)
	}

	noteStr := *note
	if noteStr == "" {
		noteStr = time.Now().Format(time.RFC3339)
	}

	if err := m.PayForUser(ctx, userID, recipientID, amount, noteStr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Paid %d (%.8f GAS) for user %s\n", amount, float64(amount)/1e8, recipientID)
	fmt.Printf("Sponsor: %s\n", userID)
}

func cmdTransactions(ctx context.Context, m *gasbank.Manager, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: slcli transactions <user_id> [limit]")
		os.Exit(1)
	}

	userID := args[0]
	limit := 20
	if len(args) > 1 {
		if l, err := strconv.Atoi(args[1]); err == nil && l > 0 {
			limit = l
		}
	}

	txs, err := m.GetTransactions(ctx, userID, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(txs) == 0 {
		fmt.Println("No transactions found")
		return
	}

	fmt.Printf("%-36s %-15s %15s %15s %-20s\n", "ID", "Type", "Amount", "Balance", "Time")
	fmt.Println(string(make([]byte, 105)))
	for _, tx := range txs {
		amtStr := fmt.Sprintf("%+d", tx.Amount)
		balStr := fmt.Sprintf("%d", tx.BalanceAfter)
		timeStr := tx.CreatedAt.Format("2006-01-02 15:04:05")
		fmt.Printf("%-36s %-15s %15s %15s %-20s\n", tx.ID[:8]+"...", tx.TxType, amtStr, balStr, timeStr)
	}
}

func cmdFees() {
	fmt.Println("Service Fees (in GAS smallest unit, 1e-8 GAS):")
	fmt.Printf("%-15s %15s %15s\n", "Service", "Fee (units)", "Fee (GAS)")
	fmt.Println(string(make([]byte, 47)))
	for service, fee := range gasbank.ServiceFees {
		fmt.Printf("%-15s %15d %15.8f\n", service, fee, float64(fee)/1e8)
	}
}
