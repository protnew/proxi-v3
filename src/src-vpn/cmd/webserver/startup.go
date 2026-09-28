package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"github.com/unkillable-messenger/vpn"
	"github.com/unkillable-messenger/vpn/auth"
	"github.com/unkillable-messenger/vpn/federation"
	"github.com/unkillable-messenger/vpn/identity"
	"github.com/unkillable-messenger/vpn/ipfs"
	"github.com/unkillable-messenger/vpn/mesh"
	"github.com/unkillable-messenger/vpn/nostr"
	"github.com/unkillable-messenger/vpn/storage"
	"github.com/unkillable-messenger/vpn/store"
	"github.com/unkillable-messenger/vpn/tor"
	"go.uber.org/zap"
)

func main() {
	// Data-safety CLI (P-C): --backup / --wipe-data run before .env load.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--backup":
			dst, err := cliBackup()
			if err != nil {
				log.Fatalf("backup: %v", err)
			}
			log.Printf("backup written: %s", dst)
			return
		case "--wipe-data":
			if err := cliWipeData(); err != nil {
				log.Fatalf("wipe: %v", err)
			}
			log.Println("data wiped")
			return
		}
	}

	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it, relying on system env vars")
	}

	// Initialize Sentry (no-op if SENTRY_DSN is empty)
	sentryDSN := os.Getenv("SENTRY_DSN")
	if sentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:           sentryDSN,
			EnableTracing: false,
			Release:       "proxi@0.1.0",
			Environment:   os.Getenv("SENTRY_ENVIRONMENT"),
		})
		if err != nil {
			log.Printf("⚠️  Sentry init failed: %v", err)
		} else {
			log.Println("🔍 Sentry error tracking enabled")
		}
		// Ensure Sentry flushes events on exit
		defer sentry.Flush(2 * time.Second)
	} else {
		log.Println("🔍 Sentry disabled (SENTRY_DSN not set)")
	}

	// Initialize Zap structured logging
	logger, _ := zap.NewProduction()
	defer logger.Sync() // flushes buffer, if any
	zap.ReplaceGlobals(logger)
	zap.RedirectStdLog(logger)
	log.Println("🚀 Proxi initializing...")

	// Wrap main logic in a deferred recover so panics are reported to Sentry
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.Flush(2 * time.Second)
			log.Fatalf("💥 Panic recovered: %v", r)
		}
	}()

	if err := run(); err != nil {
		sentry.CaptureException(err)
		sentry.Flush(2 * time.Second)
		log.Fatalf("💥 Fatal: %v", err)
	}
}

func run() error {
	startTime = time.Now()

	distDir = os.Getenv("DIST_DIR")
	if distDir == "" {
		distDir = "/app/dist"
	}

	port := resolvePort()

	// Check dist dir exists
	if info, err := os.Stat(distDir); err != nil || !info.IsDir() {
		log.Printf("⚠️  WARNING: dist dir %s does not exist, static files will not be served", distDir)
	} else {
		log.Printf("📁 Serving static files from %s", distDir)
	}

	// Start embedded IPFS node manager
	ipfsNode, err := ipfs.StartNode(context.Background())
	if err != nil {
		log.Printf("⚠️  IPFS node start warning: %v", err)
	} else {
		defer ipfsNode.Stop()
	}

	// Initialize SQLite store — one path, not process cwd
	dbPath, dataDir := resolveDBPath()
	if dataDir == "" {
		return fmt.Errorf("DATA_DIR unset and no project marker (go.mod) found — refusing silent data dir")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("data dir %s: %w", dataDir, err)
	}
	db, err := store.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	log.Println("✅ Database initialized")

	srv := &Server{
		db: db,
	}

	// P8 (2026-09-20): DMS worker один — deadMansSwitchLoop ниже (после initHub).

	var spErr error
	storageProvider, spErr = storage.NewLocalStore(filepath.Join(dataDir, "uploads"))
	if spErr != nil {
		return fmt.Errorf("failed to init storage: %w", spErr)
	}

	// Initialize Chat Hub
	srv.initHub()

	// Initialize P2P Mesh Network
	maxHops := 6
	if envHops := os.Getenv("MESH_MAX_HOPS"); envHops != "" {
		fmt.Sscanf(envHops, "%d", &maxHops)
	}
	meshNet = mesh.NewMeshNet(maxHops)
	meshNet.StartPruner(context.Background(), 5*time.Minute, 30*time.Minute)
	log.Printf("🕸️  P2P Mesh Network initialized (maxHops=%d, prune every 5m, stale after 30m)", maxHops)

	// Initialize Nostr Relay
	nostrRelay = nostr.NewRelay(50000, db)
	log.Println("📡 Nostr NIP-01 relay initialized")

	// INF-010: VPN signaling hook kind:30090 → global VPNSignaling (was local dead var)
	vpnSignaling = vpn.NewVPNSignaling()
	nostrRelay.OnEvent = func(ev nostr.Event) {
		if ev.Kind != vpn.VPNEventKind {
			return
		}
		ve, err := vpn.DeserializeVPNEvent([]byte(ev.Content))
		if err != nil {
			id := ev.ID
			if len(id) > 12 {
				id = id[:12]
			}
			log.Printf("[VPN-SIG] bad content on %s: %v", id, err)
			return
		}
		// F2: never trust content.from — publisher is ev.PubKey only.
		ve.From = ev.PubKey
		vpnSignaling.HandleIncomingEvent(ev.ID, ve)
		log.Printf("[VPN-SIG] kind:30090 %s from=%s to=%s", ve.Type, ve.From, ve.To)
	}

	// Initialize Federation Relay
	fedRelay = federation.NewFederatedRelay(nostrRelay)
	// Load persisted federation peers from DB
	fedPeers, fedPeerErr := db.GetFederationPeers()
	if fedPeerErr != nil {
		log.Printf("⚠️  Failed to load federation peers: %v", fedPeerErr)
	} else {
		for _, fp := range fedPeers {
			if err := fedRelay.AddPeer(fp.URL); err == nil {
				log.Printf("🌐 Federation peer restored: %s (%s)", fp.URL, fp.Status)
			}
		}
		if len(fedPeers) > 0 {
			log.Printf("🌐 Federation relay initialized with %d peer(s)", len(fedPeers))
		} else {
			log.Println("🌐 Federation relay initialized (no peers)")
		}
	}

	// Initialize IPFS client
	ipfsClient = ipfs.NewClient("", "")
	if ipfsClient.IsAvailable() {
		log.Println("📦 IPFS daemon connected")
	} else {
		log.Println("📦 IPFS daemon not found (file upload will return error)")
	}

	// Initialize Tor dialer
	torDialer = tor.NewTorDialer("")
	if torDialer.IsTorRunning() {
		log.Println("🧅 Tor SOCKS5 proxy connected")
	} else {
		log.Println("🧅 Tor not found (.onion connections disabled)")
	}

	// Initialize VPN Manager
	var vpnErr error
	vpnMgr, vpnErr = vpn.NewManager(getDataDir() + "/vpn")
	if vpnErr != nil {
		return fmt.Errorf("failed to initialize VPN Manager: %w", vpnErr)
	}
	log.Println("🔒 VPN Manager initialized")

	// Initialize identity — try DB first, fall back to file
	npub, _, _, idErr := db.LoadIdentity()
	if idErr != nil {
		// No identity in DB yet; try loading from legacy file
		idPath := getDataDir() + "/identity.json"
		privKey, fileErr := identity.LoadIdentity(idPath)
		if fileErr == nil {
			// Migrate file-based identity to DB
			npub = identity.PubKeyToNpub(privKey.PubKey())
			nsec := identity.PrivKeyToNsec(privKey)
			if err := db.SaveIdentity(npub, nsec, ""); err != nil {
				log.Printf("Error saving identity: %v", err)
			}
			log.Printf("🔑 Identity migrated from file to DB: %s", npub)
		}
	} else {
		log.Printf("🔑 Identity loaded from DB: %s", npub)
	}

	// P7 (2026-09-20): второй initHub удалён — пересоздавал hub и затирал
	// hardened OnMessage (P5 fail-closed + reply-enrich) наивным persist'ом.
	// Hub инициализируется один раз выше (initHub содержит persist-логику).

	// Auto-connect VPN peers on startup
	go srv.autoConnectPeers()

	// Start scheduled messages sender (every 60 seconds)
	go srv.scheduledMessagesLoop()

	// Start dead man's switch checker (every hour)
	go srv.deadMansSwitchLoop()

	// P30 (2026-09-20): WAL checkpoint + AutoBackup были определены, но нигде
	// не запускались — wal рос бесконечно, бэкапов не было.
	go srv.startWALCheckpoint(context.Background())
	srv.db.AutoBackup(context.Background(), filepath.Join(dataDir, "backups"), 24*time.Hour)

	jwtSecret, err := perBootJWTSecret()
	if err != nil {
		return err
	}
	authSvc := auth.NewAuthService(jwtSecret)
	srv.authService = authSvc

	// F11: bind loopback; if 8090 is taken, recover on :0 before routes advertise the port.
	ln, err := listenLoopback(port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	port = portFromListener(ln)
	if err := writeCoreEndpoint(dataDir, port); err != nil {
		log.Printf("⚠️  core-endpoint.json: %v", err)
	}
	defer removeCoreEndpoint(dataDir)

	// Register all HTTP routes (extracted to startup_routes.go)
	srv.registerRoutes(authSvc, distDir, port)

	// Create HTTP server
	httpSrv := &http.Server{
		Handler:           hostGate(nil),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM
	idleConnsClosed := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("📴 Received %s, shutting down gracefully...", sig)
		if srv != nil && srv.hub != nil {
			srv.hub.RawBroadcastJSON(map[string]string{"type": "system", "text": "server shutting down"}, "")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
		close(idleConnsClosed)
	}()

	// HTTPS if TLS certs configured
	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")
	if tlsCert != "" && tlsKey != "" {
		log.Printf("   TLS:  https (certs: %s)", tlsCert)
		if err := httpSrv.ServeTLS(ln, tlsCert, tlsKey); err != http.ErrServerClosed {
			return fmt.Errorf("HTTPS server error: %w", err)
		}
	} else {
		if err := httpSrv.Serve(ln); err != http.ErrServerClosed {
			return fmt.Errorf("HTTP server error: %w", err)
		}
	}

	<-idleConnsClosed
	log.Println("✅ Server stopped")
	return nil
}

// autoConnectPeers loads saved VPN peers from DB and attempts to reconnect.
// Runs in background goroutine on startup.

// INF-001: SQLite WAL checkpoint — runs every 24h
func (srv *Server) startWALCheckpoint(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if srv.db != nil {
				srv.db.DB().Exec("PRAGMA wal_checkpoint(TRUNCATE)")
				log.Println("[infra] WAL checkpoint completed")
			}
		case <-ctx.Done():
			return
		}
	}
}
