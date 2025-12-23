#!/usr/bin/env node
/**
 * Run minimal migration on remote Supabase database
 */
const dns = require("dns");
// Force IPv4 to avoid ENETUNREACH on IPv6-only hosts
dns.setDefaultResultOrder("ipv4first");

const { Client } = require("pg");
const fs = require("fs");
const path = require("path");

async function runMigration() {
  // Load environment variables
  require("dotenv").config();

  // Try pooler endpoint first (has IPv4), fallback to direct connection
  const projectRef = "dmonstzalbldzzdbbcdj";
  const usePooler = process.env.USE_POOLER === "true";

  const client = new Client({
    host: usePooler ? "aws-0-ap-southeast-1.pooler.supabase.com" : process.env.POSTGRES_HOST,
    port: usePooler ? 6543 : parseInt(process.env.POSTGRES_PORT || "5432"),
    database: process.env.POSTGRES_DB || "postgres",
    user: usePooler ? `postgres.${projectRef}` : process.env.POSTGRES_USER || "postgres",
    password: process.env.POSTGRES_PASSWORD,
    ssl: {
      rejectUnauthorized: false, // For Supabase connection
    },
    connectionTimeoutMillis: 10000,
  });

  try {
    console.log("Connecting to database...");
    console.log(`Host: ${process.env.POSTGRES_HOST}`);
    await client.connect();
    console.log("Connected successfully!");

    // Read migration SQL
    const migrationPath = path.join(__dirname, "minimal_migration.sql");
    const sql = fs.readFileSync(migrationPath, "utf8");

    console.log("Executing migration...");
    await client.query(sql);
    console.log("Migration completed successfully!");

    // Verify tables were created
    const result = await client.query(`
            SELECT table_name
            FROM information_schema.tables
            WHERE table_schema = 'public'
            AND table_name IN ('pool_accounts', 'account_balances', 'chain_txs', 'contract_events', 'simulation_txs')
            ORDER BY table_name
        `);

    console.log("\nCreated tables:");
    result.rows.forEach((row) => console.log(`  - ${row.table_name}`));
  } catch (error) {
    console.error("Migration failed:", error.message);
    process.exit(1);
  } finally {
    await client.end();
  }
}

runMigration();
