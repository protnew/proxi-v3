package economy

import (
	"fmt"
	"sync"
	"testing"
)

func TestTokenMint(t *testing.T) {
	ledger := NewTokenLedger()

	err := ledger.Mint("alice", 1000, TokenReward)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	bal := ledger.Balance("alice")
	if bal != 1000 {
		t.Errorf("alice balance = %d, want 1000", bal)
	}
	if ledger.TotalSupply() != 1000 {
		t.Errorf("total supply = %d, want 1000", ledger.TotalSupply())
	}
}

func TestTokenTransfer(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.Mint("alice", 1000, TokenReward)

	err := ledger.Transfer("alice", "bob", 500)
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}

	if ledger.Balance("alice") != 500 {
		t.Errorf("alice balance = %d, want 500", ledger.Balance("alice"))
	}
	if ledger.Balance("bob") != 500 {
		t.Errorf("bob balance = %d, want 500", ledger.Balance("bob"))
	}
	// Total supply unchanged after transfer
	if ledger.TotalSupply() != 1000 {
		t.Errorf("total supply = %d, want 1000", ledger.TotalSupply())
	}
}

func TestTokenBurn(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.Mint("alice", 1000, TokenReward)

	err := ledger.Burn("alice", 300)
	if err != nil {
		t.Fatalf("Burn: %v", err)
	}

	if ledger.Balance("alice") != 700 {
		t.Errorf("alice balance = %d, want 700", ledger.Balance("alice"))
	}
	if ledger.TotalSupply() != 700 {
		t.Errorf("total supply = %d, want 700", ledger.TotalSupply())
	}
}

func TestTokenInsufficientBalance(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.Mint("alice", 100, TokenReward)

	err := ledger.Transfer("alice", "bob", 500)
	if err == nil {
		t.Fatal("expected error for insufficient balance")
	}
}

func TestTokenStake(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.Mint("node-1", 1000, TokenReward)

	err := ledger.Stake("node-1", 500)
	if err != nil {
		t.Fatalf("Stake: %v", err)
	}

	// Available balance reduced
	if ledger.Balance("node-1") != 500 {
		t.Errorf("available balance = %d, want 500", ledger.Balance("node-1"))
	}
	// Staked amount
	if ledger.StakedBalance("node-1") != 500 {
		t.Errorf("staked balance = %d, want 500", ledger.StakedBalance("node-1"))
	}
	// Total supply unchanged (stake is just a move within the ledger)
	if ledger.TotalSupply() != 1000 {
		t.Errorf("total supply = %d, want 1000", ledger.TotalSupply())
	}
}

func TestTokenSlash(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.Mint("node-1", 1000, TokenReward)
	ledger.Stake("node-1", 500)

	err := ledger.Slash("node-1", 200)
	if err != nil {
		t.Fatalf("Slash: %v", err)
	}

	// Stake reduced
	if ledger.StakedBalance("node-1") != 300 {
		t.Errorf("staked balance = %d, want 300", ledger.StakedBalance("node-1"))
	}
	// Total supply reduced (slashed tokens are destroyed)
	if ledger.TotalSupply() != 800 {
		t.Errorf("total supply = %d, want 800", ledger.TotalSupply())
	}
	// Available balance unchanged
	if ledger.Balance("node-1") != 500 {
		t.Errorf("available balance = %d, want 500", ledger.Balance("node-1"))
	}
}

func TestTokenHistory(t *testing.T) {
	ledger := NewTokenLedger()

	ledger.Mint("alice", 1000, TokenReward)
	ledger.Transfer("alice", "bob", 400)
	ledger.Burn("alice", 100)

	history := ledger.History("alice")
	// alice has: mint(1000) + transfer(-400) + burn(-100) = 3 entries
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries for alice, got %d", len(history))
	}

	// Verify amounts
	var totalAmount int64
	for _, tok := range history {
		totalAmount += tok.Amount
	}
	// +1000 + (-400) + (-100) = 500
	if totalAmount != 500 {
		t.Errorf("history amounts sum = %d, want 500", totalAmount)
	}

	// bob's history: transfer received (+400)
	bobHist := ledger.History("bob")
	if len(bobHist) != 1 {
		t.Fatalf("expected 1 history entry for bob, got %d", len(bobHist))
	}
	if bobHist[0].Amount != 400 {
		t.Errorf("bob history amount = %d, want 400", bobHist[0].Amount)
	}

	// Non-existent account
	emptyHist := ledger.History("nobody")
	if len(emptyHist) != 0 {
		t.Errorf("non-existent account history should be empty, got %d entries", len(emptyHist))
	}
}

func TestTokenConcurrent(t *testing.T) {
	ledger := NewTokenLedger()

	// Pre-fund accounts
	ledger.Mint("pool", 100000, TokenReward)

	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	// 10 goroutines doing mint+transfer
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			target := fmt.Sprintf("goroutine-%d", idx)
			if err := ledger.Mint(target, 1000, TokenReward); err != nil {
				errCh <- err
				return
			}
			if err := ledger.Transfer(target, "pool", 500); err != nil {
				errCh <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation: %v", err)
	}

	// Each goroutine minted 1000 and transferred 500 to pool
	// So each goroutine-X has balance 500, pool has 100000 + 10*500
	for i := 0; i < 10; i++ {
		target := fmt.Sprintf("goroutine-%d", i)
		bal := ledger.Balance(target)
		if bal != 500 {
			t.Errorf("goroutine-%d balance = %d, want 500", i, bal)
		}
	}

	poolBal := ledger.Balance("pool")
	if poolBal != 105000 {
		t.Errorf("pool balance = %d, want 105000", poolBal)
	}

	// Total supply: 100000 (initial) + 10*1000 (mints) = 110000
	if ledger.TotalSupply() != 110000 {
		t.Errorf("total supply = %d, want 110000", ledger.TotalSupply())
	}
}

func TestTokenLifecycle(t *testing.T) {
	ledger := NewTokenLedger()

	// Step 1: Mint
	if err := ledger.Mint("node-1", 2000, TokenReward); err != nil {
		t.Fatalf("mint: %v", err)
	}
	if ledger.Balance("node-1") != 2000 {
		t.Errorf("after mint: balance = %d, want 2000", ledger.Balance("node-1"))
	}

	// Step 2: Transfer
	if err := ledger.Transfer("node-1", "node-2", 500); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if ledger.Balance("node-1") != 1500 {
		t.Errorf("after transfer: node-1 balance = %d, want 1500", ledger.Balance("node-1"))
	}
	if ledger.Balance("node-2") != 500 {
		t.Errorf("after transfer: node-2 balance = %d, want 500", ledger.Balance("node-2"))
	}

	// Step 3: Stake
	if err := ledger.Stake("node-1", 800); err != nil {
		t.Fatalf("stake: %v", err)
	}
	if ledger.Balance("node-1") != 700 {
		t.Errorf("after stake: available balance = %d, want 700", ledger.Balance("node-1"))
	}
	if ledger.StakedBalance("node-1") != 800 {
		t.Errorf("after stake: staked = %d, want 800", ledger.StakedBalance("node-1"))
	}

	// Step 4: Slash
	if err := ledger.Slash("node-1", 300); err != nil {
		t.Fatalf("slash: %v", err)
	}
	if ledger.StakedBalance("node-1") != 500 {
		t.Errorf("after slash: staked = %d, want 500", ledger.StakedBalance("node-1"))
	}

	// Step 5: Burn
	if err := ledger.Burn("node-1", 200); err != nil {
		t.Fatalf("burn: %v", err)
	}
	if ledger.Balance("node-1") != 500 {
		t.Errorf("after burn: available balance = %d, want 500", ledger.Balance("node-1"))
	}

	// Total supply: 2000 (minted) - 300 (slashed) - 200 (burned) = 1500
	if ledger.TotalSupply() != 1500 {
		t.Errorf("total supply = %d, want 1500", ledger.TotalSupply())
	}
}

func TestTotalSupply(t *testing.T) {
	ledger := NewTokenLedger()

	if ledger.TotalSupply() != 0 {
		t.Errorf("initial total supply = %d, want 0", ledger.TotalSupply())
	}

	ledger.Mint("alice", 5000, TokenReward)
	if ledger.TotalSupply() != 5000 {
		t.Errorf("after mint: total = %d, want 5000", ledger.TotalSupply())
	}

	ledger.Mint("bob", 3000, TokenReward)
	if ledger.TotalSupply() != 8000 {
		t.Errorf("after second mint: total = %d, want 8000", ledger.TotalSupply())
	}

	ledger.Transfer("alice", "bob", 1000)
	if ledger.TotalSupply() != 8000 {
		t.Errorf("after transfer: total = %d, want 8000 (unchanged)", ledger.TotalSupply())
	}

	ledger.Burn("alice", 500)
	if ledger.TotalSupply() != 7500 {
		t.Errorf("after burn: total = %d, want 7500", ledger.TotalSupply())
	}

	ledger.Stake("bob", 2000)
	if ledger.TotalSupply() != 7500 {
		t.Errorf("after stake: total = %d, want 7500 (unchanged)", ledger.TotalSupply())
	}

	ledger.Slash("bob", 1000)
	if ledger.TotalSupply() != 6500 {
		t.Errorf("after slash: total = %d, want 6500", ledger.TotalSupply())
	}
}
