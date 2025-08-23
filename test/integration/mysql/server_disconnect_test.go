//go:build integration
// +build integration

package mysql_test


import (
	"database/sql"
	"os/exec"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Integration test: Start the server, connect via MySQL client, and test disconnect handling.
func TestMySQLServer_ClientDisconnect_NoPanic(t *testing.T) {
	// Start the server as a subprocess
	cmd := exec.Command("../../bin/multitenant-db", "--auth-username=root", "--auth-password=")
	// If running from the project root, use the correct path
	if _, err := exec.LookPath("../../bin/multitenant-db"); err != nil {
		cmd = exec.Command("../../../bin/multitenant-db", "--auth-username=root", "--auth-password=")
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Connect to the server using the MySQL driver
	dsn := "root:@tcp(127.0.0.1:3306)/"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL server: %v", err)
	}
	// Immediately close the connection to simulate abrupt disconnect
	db.Close()

	// Wait a moment to allow server to process disconnect
	time.Sleep(200 * time.Millisecond)

	// If the server panicked, the test runner will detect it via exit code
	// Optionally, try a second connection to verify server is still alive
	db2, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("Failed to reconnect to MySQL server: %v", err)
	}
	defer db2.Close()

	if err := db2.Ping(); err != nil {
		t.Errorf("Ping after disconnect failed: %v", err)
	}
}
