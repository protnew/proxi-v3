package nostr

// P6 (2026-09-20): official NIP-44 v2 test vectors
// Source: https://github.com/paulmillr/nip44/blob/main/nip44.vectors.json
// sha256(file) = 269ed0f69e4c192512cc779e78c555090cebc7c785b609e338a62afc3ce25040

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
)

func mustPriv(t *testing.T, hexKey string) *btcec.PrivateKey {
	t.Helper()
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		t.Fatalf("bad seckey hex: %v", err)
	}
	priv, _ := btcec.PrivKeyFromBytes(b)
	return priv
}

func mustPub(t *testing.T, hexKey string) *btcec.PublicKey {
	t.Helper()
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		t.Fatalf("bad pubkey hex: %v", err)
	}
	pub, err := btcec.ParsePubKey(append([]byte{0x02}, b...))
	if err != nil {
		// try odd parity
		pub, err = btcec.ParsePubKey(append([]byte{0x03}, b...))
		if err != nil {
			t.Fatalf("bad pubkey: %v", err)
		}
	}
	return pub
}

// valid.get_conversation_key — sample of 5 official vectors
func TestNip44VectorConversationKey(t *testing.T) {
	vectors := []struct{ sec1, pub2, want string }{
		{"315e59ff51cb9209768cf7da80791ddcaae56ac9775eb25b6dee1234bc5d2268",
			"c2f9d9948dc8c7c38321e4b85c8558872eafa0641cd269db76848a6073e69133",
			"3dfef0ce2a4d80a25e7a328accf73448ef67096f65f79588e358d9a0eb9013f1"},
		{"a1e37752c9fdc1273be53f68c5f74be7c8905728e8de75800b94262f9497c86e",
			"03bb7947065dde12ba991ea045132581d0954f042c84e06d8c00066e23c1a800",
			"4d14f36e81b8452128da64fe6f1eae873baae2f444b02c950b90e43553f2178b"},
		{"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364139",
			"0000000000000000000000000000000000000000000000000000000000000002",
			"8b6392dbf2ec6a2b2d5b1477fc2be84d63ef254b667cadd31bd3f444c44ae6ba"},
		{"5e914bdac54f3f8e2cba94ee898b33240019297b69e96e70c8a495943a72fc98",
			"5bd097924f606695c59f18ff8fd53c174adbafaaa71b3c0b4144a3e0a474b198",
			"f5a0aecf2984bf923c8cd5e7bb8be262d1a8353cb93959434b943a07cf5644bc"},
		{"e4a8bcacbf445fd3721792b939ff58e691cdcba6a8ba67ac3467b45567a03e5c",
			"b54053189e8c9252c6950059c783edb10675d06d20c7b342f73ec9fa6ed39c9d",
			"7b3933b4ef8189d347169c7955589fc1cfc01da5239591a08a183ff6694c44ad"},
	}
	for i, v := range vectors {
		priv := mustPriv(t, v.sec1)
		pub := mustPub(t, v.pub2)
		got, err := computeConversationKey(priv, pub)
		if err != nil {
			t.Fatalf("vec %d: %v", i, err)
		}
		if hex.EncodeToString(got[:]) != v.want {
			t.Errorf("vec %d: conversation_key mismatch\n got: %x\nwant: %s", i, got, v.want)
		}
	}
}

// valid.calc_padded_len — full official set
func TestNip44VectorPaddedLen(t *testing.T) {
	vectors := [][2]int{
		{16, 32}, {32, 32}, {33, 64}, {37, 64}, {45, 64}, {49, 64},
		{64, 64}, {65, 96}, {100, 128}, {111, 128}, {200, 224},
		{250, 256}, {320, 320}, {383, 384}, {384, 384}, {400, 448},
		{500, 512}, {512, 512}, {515, 640}, {700, 768}, {800, 896},
		{900, 1024}, {1020, 1024}, {65536, 65536},
	}
	for _, v := range vectors {
		if got := calcPaddedLen(v[0]); got != v[1] {
			t.Errorf("calcPaddedLen(%d): got %d, want %d", v[0], got, v[1])
		}
	}
}

// valid.encrypt_decrypt — official vectors (deterministic nonce)
func TestNip44VectorEncryptDecrypt(t *testing.T) {
	vectors := []struct{ sec1, sec2, nonce, plaintext, payload string }{
		{"0000000000000000000000000000000000000000000000000000000000000001",
			"0000000000000000000000000000000000000000000000000000000000000002",
			"0000000000000000000000000000000000000000000000000000000000000001",
			"a",
			"AgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABee0G5VSK0/9YypIObAtDKfYEAjD35uVkHyB0F4DwrcNaCXlCWZKaArsGrY6M9wnuTMxWfp1RTN9Xga8no+kF5Vsb"},
		{"0000000000000000000000000000000000000000000000000000000000000002",
			"0000000000000000000000000000000000000000000000000000000000000001",
			"f00000000000000000000000000000f00000000000000000000000000000000f",
			"🍕🫃",
			"AvAAAAAAAAAAAAAAAAAAAPAAAAAAAAAAAAAAAAAAAAAPSKSK6is9ngkX2+cSq85Th16oRTISAOfhStnixqZziKMDvB0QQzgFZdjLTPicCJaV8nDITO+QfaQ61+KbWQIOO2Yj"},
		{"5c0c523f52a5b6fad39ed2403092df8cebc36318b39383bca6c00808626fab3a",
			"4b22aa260e4acb7021e32f38a6cdf4b673c6a277755bfce287e370c924dc936d",
			"b635236c42db20f021bb8d1cdff5ca75dd1a0cc72ea742ad750f33010b24f73b",
			"表ポあA鷗ŒéＢ逍Üßªąñ丂㐀𠀀",
			"ArY1I2xC2yDwIbuNHN/1ynXdGgzHLqdCrXUPMwELJPc7s7JqlCMJBAIIjfkpHReBPXeoMCyuClwgbT419jUWU1PwaNl4FEQYKCDKVJz+97Mp3K+Q2YGa77B6gpxB/lr1QgoqpDf7wDVrDmOqGoiPjWDqy8KzLueKDcm9BVP8xeTJIxs="},
		{"8f40e50a84a7462e2b8d24c28898ef1f23359fff50d8c509e6fb7ce06e142f9c",
			"b9b0a1e9cc20100c5faa3bbe2777303d25950616c4c6a3fa2e3e046f936ec2ba",
			"b20989adc3ddc41cd2c435952c0d59a91315d8c5218d5040573fc3749543acaf",
			"ability🤝的 ȺȾ",
			"ArIJia3D3cQc0sQ1lSwNWakTFdjFIY1QQFc/w3SVQ6yvbG2S0x4Yu86QGwPTy7mP3961I1XqB6SFFTzqDZZavhxoWMj7mEVGMQIsh2RLWI5EYQaQDIePSnXPlzf7CIt+voTD"},
		{"d5633530f5bcfebceb5584cfbbf718a30df0751b729dd9a789b9f30c0587d74e",
			"b74e6a341fb134127272b795a08b59250e5fa45a82a2eb4095e4ce9ed5f5e214",
			"38d1ca0abef9e5f564e89761a86cee04574b6825d3ef2063b10ad75899e4b023",
			"الكل في المجمو عة (5)",
			"AjjRygq++eX1ZOiXYahs7gRXS2gl0+8gY7EK11iZ5LAjbOTrlfrxak5Lki42v2jMPpLSicy8eHjsWkkMtF0i925vOaKG/ZkMHh9ccQBdfTvgEGKzztedqDCAWb5TP1YwU1PsWaiiqG3+WgVvJiO4lUdMHXL7+zKKx8bgDtowzz4QAwI="},
	}
	for i, v := range vectors {
		sec1 := mustPriv(t, v.sec1)
		sec2 := mustPriv(t, v.sec2)
		pub2 := sec2.PubKey()
		pub1 := sec1.PubKey()

		nonce, err := hex.DecodeString(v.nonce)
		if err != nil {
			t.Fatalf("vec %d: bad nonce: %v", i, err)
		}

		// Encrypt with fixed nonce must reproduce the official payload byte-for-byte.
		got, err := encrypt44WithNonce(sec1, pub2, v.plaintext, nonce)
		if err != nil {
			t.Fatalf("vec %d: encrypt: %v", i, err)
		}
		if got != v.payload {
			t.Errorf("vec %d: payload mismatch\n got: %s\nwant: %s", i, got, v.payload)
		}

		// Decrypt the official payload.
		pt, err := Decrypt44(sec2, pub1, v.payload)
		if err != nil {
			t.Fatalf("vec %d: decrypt: %v", i, err)
		}
		if pt != v.plaintext {
			t.Errorf("vec %d: plaintext mismatch: got %q, want %q", i, pt, v.plaintext)
		}
	}
}
