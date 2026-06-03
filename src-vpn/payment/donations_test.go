package payment

import (
	"testing"
)

func TestCreateDonation_Success(t *testing.T) {
	db := newTestDB(t)
	err := CreateDonation(db, "donor1", "channel1", 5000)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateDonation_Multiple(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 5; i++ {
		err := CreateDonation(db, "donor1", "channel1", 100)
		if err != nil {
			t.Fatal(err)
		}
	}
	total, err := GetTotalDonations(db, "channel1")
	if err != nil {
		t.Fatal(err)
	}
	if total != 500 {
		t.Errorf("expected 500 total, got %d", total)
	}
}

func TestGetDonations_Empty(t *testing.T) {
	db := newTestDB(t)
	donations, err := GetDonations(db, "nonexistent_channel", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(donations) != 0 {
		t.Errorf("expected 0 donations, got %d", len(donations))
	}
}

func TestGetDonations_Limit(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 10; i++ {
		CreateDonation(db, "donor", "ch", 50)
	}
	donations, err := GetDonations(db, "ch", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(donations) != 3 {
		t.Errorf("expected 3 donations with limit, got %d", len(donations))
	}
}

func TestGetDonations_DifferentChannels(t *testing.T) {
	db := newTestDB(t)
	CreateDonation(db, "a", "ch1", 100)
	CreateDonation(db, "b", "ch2", 200)
	CreateDonation(db, "c", "ch1", 300)

	donations1, _ := GetDonations(db, "ch1", 10)
	donations2, _ := GetDonations(db, "ch2", 10)

	if len(donations1) != 2 {
		t.Errorf("ch1 should have 2 donations, got %d", len(donations1))
	}
	if len(donations2) != 1 {
		t.Errorf("ch2 should have 1 donation, got %d", len(donations2))
	}
}

func TestGetDonations_Fields(t *testing.T) {
	db := newTestDB(t)
	CreateDonation(db, "alice", "chan1", 777)

	donations, err := GetDonations(db, "chan1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(donations) != 1 {
		t.Fatalf("expected 1 donation, got %d", len(donations))
	}
	d := donations[0]
	if d["from_user"] != "alice" {
		t.Errorf("from_user mismatch: %v", d["from_user"])
	}
	if d["to_channel"] != "chan1" {
		t.Errorf("to_channel mismatch: %v", d["to_channel"])
	}
	if d["amount_sats"].(int64) != 777 {
		t.Errorf("amount_sats mismatch: %v", d["amount_sats"])
	}
}

func TestGetTotalDonations_NoDonations(t *testing.T) {
	db := newTestDB(t)
	total, err := GetTotalDonations(db, "nochannel")
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("expected 0 total for no donations, got %d", total)
	}
}

func TestGetTotalDonations_MultipleDonors(t *testing.T) {
	db := newTestDB(t)
	CreateDonation(db, "user1", "ch", 1000)
	CreateDonation(db, "user2", "ch", 2000)
	CreateDonation(db, "user3", "other_ch", 500)

	total, err := GetTotalDonations(db, "ch")
	if err != nil {
		t.Fatal(err)
	}
	if total != 3000 {
		t.Errorf("expected 3000 total, got %d", total)
	}
}

func TestCreateDonation_DifferentAmounts(t *testing.T) {
	db := newTestDB(t)
	amounts := []int64{1, 10, 100, 1000, 10000}
	for _, amt := range amounts {
		err := CreateDonation(db, "donor", "ch", amt)
		if err != nil {
			t.Fatalf("failed for amount %d: %v", amt, err)
		}
	}
	total, _ := GetTotalDonations(db, "ch")
	if total != 11111 {
		t.Errorf("expected 11111 total, got %d", total)
	}
}
