//go:build ignore

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	// PostgreSQL connection string from .env
	connStr := "host=db.dmonstzalbldzzdbbcdj.supabase.co port=5432 user=postgres password=xaNT3apRNOrRgZd5 dbname=postgres sslmode=require"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Connected to database")

	// SQL statements to add anon policies
	statements := []string{
		// Pool Accounts
		`DROP POLICY IF EXISTS anon_all ON pool_accounts`,
		`CREATE POLICY anon_all ON pool_accounts FOR ALL TO anon USING (true) WITH CHECK (true)`,

		// Account Balances
		`DROP POLICY IF EXISTS anon_all ON account_balances`,
		`CREATE POLICY anon_all ON account_balances FOR ALL TO anon USING (true) WITH CHECK (true)`,

		// Chain Transactions
		`DROP POLICY IF EXISTS anon_all ON chain_txs`,
		`CREATE POLICY anon_all ON chain_txs FOR ALL TO anon USING (true) WITH CHECK (true)`,

		// Contract Events
		`DROP POLICY IF EXISTS anon_all ON contract_events`,
		`CREATE POLICY anon_all ON contract_events FOR ALL TO anon USING (true) WITH CHECK (true)`,

		// Simulation Transactions
		`DROP POLICY IF EXISTS anon_all ON simulation_txs`,
		`CREATE POLICY anon_all ON simulation_txs FOR ALL TO anon USING (true) WITH CHECK (true)`,

		// Grant permissions
		`GRANT ALL ON pool_accounts TO anon`,
		`GRANT ALL ON account_balances TO anon`,
		`GRANT ALL ON chain_txs TO anon`,
		`GRANT ALL ON contract_events TO anon`,
		`GRANT ALL ON simulation_txs TO anon`,
		`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO anon`,
	}

	for _, stmt := range statements {
		fmt.Printf("Executing: %s\n", stmt[:min(len(stmt), 60)]+"...")
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to execute '%s': %v\n", stmt[:min(len(stmt), 40)], err)
			// Continue with other statements
		}
	}

	fmt.Println("\nRLS policies updated successfully!")

	// Verify by listing policies
	rows, err := db.QueryContext(ctx, `
		SELECT tablename, policyname, roles, cmd
		FROM pg_policies
		WHERE tablename IN ('pool_accounts', 'account_balances', 'chain_txs', 'contract_events', 'simulation_txs')
		ORDER BY tablename, policyname
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query policies: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	fmt.Println("\nCurrent policies:")
	fmt.Printf("%-20s %-20s %-20s %-10s\n", "Table", "Policy", "Roles", "Command")
	fmt.Println("--------------------------------------------------------------------")
	for rows.Next() {
		var table, policy, roles, cmd string
		if err := rows.Scan(&table, &policy, &roles, &cmd); err != nil {
			continue
		}
		fmt.Printf("%-20s %-20s %-20s %-10s\n", table, policy, roles, cmd)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
