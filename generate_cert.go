package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"
)

func main() {
	// WebTransport требует короткоживущих сертификатов (макс 14 дней)
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(10 * 24 * time.Hour) 

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Indestructible VPN Local Dev"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Fatal(err)
	}

	// Вычисляем SHA-256 хэш сертификата
	// Клиентский браузер (Frontend) должен передать этот хэш в конструктор WebTransport(url, {serverCertificateHashes: [...]})
	// Иначе браузер отклонит соединение к самоподписанному localhost
	hash := sha256.Sum256(derBytes)
	hashHex := hex.EncodeToString(hash[:])
	fmt.Printf("\n=======================================================\n")
	fmt.Printf("ВНИМАНИЕ! Скопируйте этот ХЭШ во Frontend приложение:\n")
	fmt.Printf("WebTransport Certificate Hash: %s\n", hashHex)
	fmt.Printf("=======================================================\n\n")

	certOut, err := os.Create("cert.pem")
	if err != nil {
		log.Fatal(err)
	}
	defer certOut.Close()
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	keyOut, err := os.Create("cert.key")
	if err != nil {
		log.Fatal(err)
	}
	defer keyOut.Close()
	x509Bytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		log.Fatal(err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: x509Bytes})
	
	fmt.Println("Сгенерированы ключи: cert.pem и cert.key")
}
